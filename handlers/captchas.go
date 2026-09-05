package handlers

import (
	"context"
	"fmt"
	"html"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"minimate-bot/database"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type CaptchaSettings struct {
	Enabled        bool
	TimeoutSeconds int
	Mode           string // "button" or "math"
}

// activeVerifications tracks pending verifications: "chatID:userID" -> expected answer or "pass"
var (
	activeVerifications = make(map[string]string)
	verificationMutex   sync.RWMutex

	captchaCache = make(map[int64]CaptchaSettings)
	captchaMutex sync.RWMutex
)

func loadCaptchaSettings(chatID int64) CaptchaSettings {
	captchaMutex.Lock()
	defer captchaMutex.Unlock()

	if s, exists := captchaCache[chatID]; exists {
		return s
	}

	var s CaptchaSettings
	query := "SELECT enabled, timeout_seconds, mode FROM chat_captcha WHERE chat_id = $1"
	err := database.Pool.QueryRow(context.Background(), query, chatID).
		Scan(&s.Enabled, &s.TimeoutSeconds, &s.Mode)

	if err != nil {
		s = CaptchaSettings{Enabled: false, TimeoutSeconds: 120, Mode: "button"}
	}
	captchaCache[chatID] = s
	return s
}

func getCaptchaSettings(chatID int64) CaptchaSettings {
	captchaMutex.RLock()
	s, exists := captchaCache[chatID]
	captchaMutex.RUnlock()

	if !exists {
		return loadCaptchaSettings(chatID)
	}
	return s
}

// HandleCaptchaOnJoin restricts a newly joined user and sends an interactive challenge
func HandleCaptchaOnJoin(bot *tgbotapi.BotAPI, chatID int64, user *tgbotapi.User) {
	settings := getCaptchaSettings(chatID)
	if !settings.Enabled || user.IsBot {
		return
	}

	// 1. Mute the user immediately
	perms := tgbotapi.ChatPermissions{CanSendMessages: false}
	restrict := tgbotapi.RestrictChatMemberConfig{
		ChatMemberConfig: tgbotapi.ChatMemberConfig{
			ChatID: chatID,
			UserID: user.ID,
		},
		Permissions: &perms,
	}
	bot.Request(restrict)

	key := fmt.Sprintf("%d:%d", chatID, user.ID)

	var challengeText string
	var keyboard tgbotapi.InlineKeyboardMarkup

	if settings.Mode == "math" {
		a := rand.Intn(10) + 1
		b := rand.Intn(10) + 1
		ans := a + b

		verificationMutex.Lock()
		activeVerifications[key] = strconv.Itoa(ans)
		verificationMutex.Unlock()

		challengeText = fmt.Sprintf(`🤖 <b>Security Challenge for %s</b>

👋 Welcome! To chat in this group, please solve the challenge below within <b>%d seconds</b>:

<blockquote>🧮 <b>What is %d + %d ?</b></blockquote>`,
			html.EscapeString(user.FirstName), settings.TimeoutSeconds, a, b)

		wrong1 := ans + 2
		wrong2 := ans - 1
		if wrong2 <= 0 {
			wrong2 = ans + 3
		}

		keyboard = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("%d", ans), fmt.Sprintf("captcha_verify:%d:%d", user.ID, ans)),
				tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("%d", wrong1), fmt.Sprintf("captcha_verify:%d:%d", user.ID, wrong1)),
				tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("%d", wrong2), fmt.Sprintf("captcha_verify:%d:%d", user.ID, wrong2)),
			),
		)
	} else {
		verificationMutex.Lock()
		activeVerifications[key] = "pass"
		verificationMutex.Unlock()

		challengeText = fmt.Sprintf(`🤖 <b>Human Verification Required</b>

👋 Welcome, <b>%s</b>!
Please verify that you are human by clicking the button below within <b>%d seconds</b>.`,
			html.EscapeString(user.FirstName), settings.TimeoutSeconds)

		keyboard = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("✅ I am Human", fmt.Sprintf("captcha_verify:%d:pass", user.ID)),
			),
		)
	}

	msg := tgbotapi.NewMessage(chatID, challengeText)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = keyboard

	sentMsg, err := bot.Send(msg)
	if err != nil {
		log.Printf("Failed to send captcha message: %v", err)
		return
	}

	// 2. Start timeout background worker
	go func(targetUserID int64, challengeMsgID int) {
		time.Sleep(time.Duration(settings.TimeoutSeconds) * time.Second)

		verificationMutex.Lock()
		_, pending := activeVerifications[key]
		if pending {
			delete(activeVerifications, key)
		}
		verificationMutex.Unlock()

		// If user did not verify in time, kick them
		if pending {
			kickConfig := tgbotapi.BanChatMemberConfig{
				ChatMemberConfig: tgbotapi.ChatMemberConfig{ChatID: chatID, UserID: targetUserID},
			}
			bot.Request(kickConfig)

			unbanConfig := tgbotapi.UnbanChatMemberConfig{
				ChatMemberConfig: tgbotapi.ChatMemberConfig{ChatID: chatID, UserID: targetUserID},
				OnlyIfBanned:     true,
			}
			bot.Request(unbanConfig)

			bot.Request(tgbotapi.NewDeleteMessage(chatID, challengeMsgID))
		}
	}(user.ID, sentMsg.MessageID)
}

// HandleCaptchaCallback processes captcha button taps
func HandleCaptchaCallback(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) bool {
	if !strings.HasPrefix(query.Data, "captcha_verify:") {
		return false
	}

	parts := strings.Split(query.Data, ":")
	if len(parts) < 3 {
		return false
	}

	targetUserID, _ := strconv.ParseInt(parts[1], 10, 64)
	submittedAns := parts[2]
	clickerID := query.From.ID
	chatID := query.Message.Chat.ID

	if clickerID != targetUserID {
		alert := tgbotapi.NewCallbackWithAlert(query.ID, "❌ This verification challenge is not for you!")
		bot.Request(alert)
		return true
	}

	key := fmt.Sprintf("%d:%d", chatID, targetUserID)

	verificationMutex.RLock()
	expectedAns, exists := activeVerifications[key]
	verificationMutex.RUnlock()

	if !exists {
		alert := tgbotapi.NewCallbackWithAlert(query.ID, "⚠️ Verification expired or already completed.")
		bot.Request(alert)
		return true
	}

	if submittedAns != expectedAns {
		alert := tgbotapi.NewCallbackWithAlert(query.ID, "❌ Incorrect answer. Try again!")
		bot.Request(alert)
		return true
	}

	// Verification Success
	verificationMutex.Lock()
	delete(activeVerifications, key)
	verificationMutex.Unlock()

	perms := tgbotapi.ChatPermissions{
		CanSendMessages:       true,
		CanSendMediaMessages:  true,
		CanSendPolls:          true,
		CanSendOtherMessages:  true,
		CanAddWebPagePreviews: true,
	}
	restrict := tgbotapi.RestrictChatMemberConfig{
		ChatMemberConfig: tgbotapi.ChatMemberConfig{ChatID: chatID, UserID: targetUserID},
		Permissions:      &perms,
	}
	bot.Request(restrict)

	bot.Request(tgbotapi.NewCallback(query.ID, "✅ Verified successfully! Welcome to the group!"))

	editText := tgbotapi.NewEditMessageText(chatID, query.Message.MessageID,
		fmt.Sprintf("✅ <b>%s</b> passed verification and is now allowed to chat!", html.EscapeString(query.From.FirstName)))
	editText.ParseMode = "HTML"
	bot.Send(editText)

	go func() {
		time.Sleep(4 * time.Second)
		bot.Request(tgbotapi.NewDeleteMessage(chatID, query.Message.MessageID))
	}()

	return true
}

// HandleCaptchaCommand processes /captcha, /captchatime, /captchamode
func HandleCaptchaCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message, cmd string, args string) {
	chatID := message.Chat.ID
	fromID := int64(0)
	if message.From != nil {
		fromID = message.From.ID
	} else if message.SenderChat != nil {
		fromID = message.SenderChat.ID
	}

	if !isAdmin(bot, chatID, fromID) {
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Only group administrators can configure Captcha."))
		return
	}

	s := getCaptchaSettings(chatID)

	switch cmd {
	case "captcha":
		if strings.TrimSpace(args) == "" {
			statusStr := "OFF"
			if s.Enabled {
				statusStr = "ON"
			}
			msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("🤖 <b>Captcha Verification:</b> <code>%s</code>\nMode: <code>%s</code> | Timeout: <code>%ds</code>\n\nUsage: <code>/captcha &lt;on/off&gt;</code>", statusStr, s.Mode, s.TimeoutSeconds))
			msg.ParseMode = "HTML"
			bot.Send(msg)
			return
		}

		argLower := strings.ToLower(strings.TrimSpace(args))
		s.Enabled = (argLower == "on" || argLower == "yes" || argLower == "true" || argLower == "1")

	case "captchatime":
		if strings.TrimSpace(args) == "" {
			msg := tgbotapi.NewMessage(chatID, "❌ Usage: <code>/captchatime &lt;seconds&gt;</code> (e.g. <code>/captchatime 120</code>)")
			msg.ParseMode = "HTML"
			bot.Send(msg)
			return
		}
		val, err := strconv.Atoi(strings.TrimSpace(args))
		if err != nil || val < 30 || val > 600 {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Timeout must be between 30 and 600 seconds."))
			return
		}
		s.TimeoutSeconds = val

	case "captchamode":
		mode := strings.ToLower(strings.TrimSpace(args))
		if mode != "button" && mode != "math" {
			msg := tgbotapi.NewMessage(chatID, "❌ Usage: <code>/captchamode &lt;button|math&gt;</code>")
			msg.ParseMode = "HTML"
			bot.Send(msg)
			return
		}
		s.Mode = mode

	case "captchakick":
		bot.Send(tgbotapi.NewMessage(chatID, "⚙️ Auto-kick on captcha failure is enabled by default."))
		return
	}

	query := `
		INSERT INTO chat_captcha (chat_id, enabled, timeout_seconds, mode)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (chat_id) DO UPDATE SET
			enabled = EXCLUDED.enabled,
			timeout_seconds = EXCLUDED.timeout_seconds,
			mode = EXCLUDED.mode;
	`
	_, err := database.Pool.Exec(context.Background(), query, chatID, s.Enabled, s.TimeoutSeconds, s.Mode)
	if err != nil {
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Database error updating captcha configuration."))
		return
	}

	captchaMutex.Lock()
	captchaCache[chatID] = s
	captchaMutex.Unlock()

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ <b>Captcha Configuration Updated:</b>\n• Enabled: <code>%t</code>\n• Mode: <code>%s</code>\n• Timeout: <code>%ds</code>", s.Enabled, s.Mode, s.TimeoutSeconds))
	msg.ParseMode = "HTML"
	bot.Send(msg)
}