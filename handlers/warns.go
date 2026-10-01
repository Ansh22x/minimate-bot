package handlers

import (
	"fmt"
	"html"
	"strings"

	"minimate-bot/services"
	"minimate-bot/workers"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// HandleWarnCommand manages user warnings and threshold punishments
func HandleWarnCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message, cmd string, args string) {
	chatID := message.Chat.ID
	fromID := int64(0)
	if message.From != nil {
		fromID = message.From.ID
	} else if message.SenderChat != nil {
		fromID = message.SenderChat.ID
	}
	isUserAdmin := services.IsAdminOrRole(bot, chatID, fromID)

	switch cmd {
	case "warn", "dwarn":
		if !isUserAdmin {
			sendHTMLMessage(bot, chatID, "❌ Only admins can warn users.")
			return
		}

		target, reason := ExtractTargetUser(bot, message, args)
		if target == nil {
			sendHTMLMessage(bot, chatID, "❌ Reply to a user's message or specify their <code>@username</code> / User ID to warn them.\n\n<i>Example:</i> <code>/warn @username [reason]</code> or <code>/warn [reason]</code> (as reply)")
			return
		}

		if target.ID == bot.Self.ID {
			sendHTMLMessage(bot, chatID, "❌ I cannot warn myself.")
			return
		}
		if services.IsAdminOrRole(bot, chatID, target.ID) {
			sendHTMLMessage(bot, chatID, "❌ You cannot issue warnings to an administrator.")
			return
		}

		reasonText := "No reason provided."
		if strings.TrimSpace(reason) != "" {
			reasonText = strings.TrimSpace(reason)
		}

		// Issue warning via central Moderation Service
		warnCount, autoAction, res := services.WarnUser(bot, chatID, target.ID, fromID, reasonText)
		if !res.Success {
			sendHTMLMessage(bot, chatID, res.Message)
			return
		}

		// Track metrics
		workers.IncrMetric(chatID, workers.MetricWarn)

		settings := services.GetGroupSettings(chatID)
		warnText := fmt.Sprintf("⚠️ <b>%s</b> has been warned.\n<b>Reason:</b> %s\n<b>Warnings:</b> %d/%d",
			html.EscapeString(target.FirstName), html.EscapeString(reasonText), warnCount, settings.WarnLimit)

		if autoAction != "" {
			warnText += fmt.Sprintf("\n\n🚨 <b>Automatic Threshold Triggered:</b> <code>%s</code>", html.EscapeString(autoAction))
		}

		msg := tgbotapi.NewMessage(chatID, warnText)
		msg.ParseMode = "HTML"
		if res.UndoToken != "" {
			msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("↩️ Undo Warn", res.UndoToken),
				),
			)
		}
		SafeSend(bot, msg)

		// Delete the offending message if the command is /dwarn and it was a reply
		if cmd == "dwarn" && message.ReplyToMessage != nil {
			bot.Request(tgbotapi.NewDeleteMessage(chatID, message.ReplyToMessage.MessageID))
		}

	case "unwarn", "rmwarn", "delwarn", "removewarn", "remwarn", "unwarns":
		if !isUserAdmin {
			sendHTMLMessage(bot, chatID, "❌ Only admins can remove warnings.")
			return
		}

		target, _ := ExtractTargetUser(bot, message, args)
		if target == nil {
			sendHTMLMessage(bot, chatID, "❌ Reply to a user's message or specify their <code>@username</code> / User ID to remove a warning.\n\n<i>Example:</i> <code>/unwarn @username</code> or <code>/rmwarn</code> (as reply)")
			return
		}

		remaining, err := services.RemoveWarn(chatID, target.ID, fromID)
		if err != nil {
			sendHTMLMessage(bot, chatID, "❌ No active warnings found for this user.")
			return
		}

		settings := services.GetGroupSettings(chatID)
		sendHTMLMessage(bot, chatID, fmt.Sprintf("✅ Removed a warning for <b>%s</b>.\n<b>Warnings:</b> %d/%d",
			html.EscapeString(target.FirstName), remaining, settings.WarnLimit))

	case "rmwarns", "resetwarns", "delwarns", "clearwarns", "resetwarn", "clearwarn", "removewarns", "removeallwarns":
		if !isUserAdmin {
			sendHTMLMessage(bot, chatID, "❌ Only admins can reset warnings.")
			return
		}

		target, _ := ExtractTargetUser(bot, message, args)
		if target == nil {
			sendHTMLMessage(bot, chatID, "❌ Reply to a user's message or specify their <code>@username</code> / User ID to reset all warnings.\n\n<i>Example:</i> <code>/resetwarns @username</code> or <code>/rmwarns</code> (as reply)")
			return
		}

		err := services.ResetWarns(chatID, target.ID, fromID)
		if err != nil {
			sendHTMLMessage(bot, chatID, "❌ Failed to reset warnings.")
			return
		}

		settings := services.GetGroupSettings(chatID)
		sendHTMLMessage(bot, chatID, fmt.Sprintf("✅ All warnings for <b>%s</b> have been reset to <code>0/%d</code>.",
			html.EscapeString(target.FirstName), settings.WarnLimit))

	case "warns":
		target, _ := ExtractTargetUser(bot, message, args)
		if target == nil {
			if message.From != nil {
				target = message.From
			} else {
				sendHTMLMessage(bot, chatID, "❌ Unable to determine user identity.")
				return
			}
		}

		count := services.GetWarnCount(chatID, target.ID)
		settings := services.GetGroupSettings(chatID)

		sendHTMLMessage(bot, chatID, fmt.Sprintf("⚠️ <b>%s</b> has <code>%d/%d</code> warnings.",
			html.EscapeString(target.FirstName), count, settings.WarnLimit))

	case "warnlimit", "warnmode":
		sendHTMLMessage(bot, chatID, "⚙️ Configure custom warning limits and action modes in the <code>/panel</code> dashboard under <b>⚙️ Settings</b>.")
	}
}