// Package services provides the central permission and settings service.
package services

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"minimate-bot/database"
	"minimate-bot/models"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// ==================== SETTINGS CACHE ====================

var (
	settingsCache   = make(map[int64]*models.GroupSettings)
	settingsMu      sync.RWMutex
	settingsTTL     = make(map[int64]time.Time)
	settingsCacheTTL = 5 * time.Minute
)

// GetGroupSettings returns settings for a group, with in-memory caching.
func GetGroupSettings(chatID int64) *models.GroupSettings {
	settingsMu.RLock()
	if s, ok := settingsCache[chatID]; ok {
		if time.Now().Before(settingsTTL[chatID]) {
			settingsMu.RUnlock()
			return s
		}
	}
	settingsMu.RUnlock()

	// Load from DB
	var raw []byte
	err := database.Pool.QueryRow(context.Background(),
		"SELECT settings FROM group_settings WHERE chat_id = $1", chatID,
	).Scan(&raw)

	settings := models.DefaultGroupSettings()
	if err == nil && len(raw) > 0 {
		if err2 := json.Unmarshal(raw, &settings); err2 != nil {
			log.Printf("⚠️ Settings decode error for chat %d: %v", chatID, err2)
		}
	}

	settingsMu.Lock()
	settingsCache[chatID] = &settings
	settingsTTL[chatID] = time.Now().Add(settingsCacheTTL)
	settingsMu.Unlock()

	return &settings
}

// SaveGroupSettings persists settings to the database and invalidates cache.
func SaveGroupSettings(chatID int64, settings *models.GroupSettings) error {
	raw, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	_, err = database.Pool.Exec(context.Background(), `
		INSERT INTO group_settings (chat_id, settings, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (chat_id) DO UPDATE
		SET settings = $2, updated_at = NOW()
	`, chatID, raw)
	if err != nil {
		return err
	}

	// Invalidate cache
	settingsMu.Lock()
	delete(settingsCache, chatID)
	delete(settingsTTL, chatID)
	settingsMu.Unlock()

	return nil
}

// InvalidateSettingsCache clears cached settings for a group.
func InvalidateSettingsCache(chatID int64) {
	settingsMu.Lock()
	delete(settingsCache, chatID)
	delete(settingsTTL, chatID)
	settingsMu.Unlock()
}

// ==================== PERMISSION CHECKS ====================

// PermissionResult holds the outcome of a permission check.
type PermissionResult struct {
	Allowed bool
	Reason  string
}

// CheckPermission verifies a user can perform an action in a group.
// It checks Telegram native admin status first, then bot admin roles.
func CheckPermission(bot *tgbotapi.BotAPI, chatID int64, userID int64, permission string) PermissionResult {
	if chatID == 0 || userID == 0 {
		return PermissionResult{false, "Invalid chat or user ID"}
	}

	// 1. Check Telegram native admin/creator status
	member, err := bot.GetChatMember(tgbotapi.GetChatMemberConfig{
		ChatConfigWithUser: tgbotapi.ChatConfigWithUser{ChatID: chatID, UserID: userID},
	})
	if err == nil {
		status := member.Status
		if status == "creator" || status == "administrator" {
			return PermissionResult{true, ""}
		}
	}

	// 2. Check bot admin role permissions
	perms := GetBotAdminPermissions(chatID, userID)
	if perms == nil {
		return PermissionResult{false, "❌ You don't have permission to do this."}
	}

	switch permission {
	case "ban":
		if !perms.CanBan {
			return PermissionResult{false, "❌ Your role doesn't allow banning members."}
		}
	case "mute":
		if !perms.CanMute {
			return PermissionResult{false, "❌ Your role doesn't allow muting members."}
		}
	case "kick":
		if !perms.CanKick {
			return PermissionResult{false, "❌ Your role doesn't allow kicking members."}
		}
	case "delete":
		if !perms.CanDelete {
			return PermissionResult{false, "❌ Your role doesn't allow deleting messages."}
		}
	case "warn":
		if !perms.CanWarn {
			return PermissionResult{false, "❌ Your role doesn't allow issuing warnings."}
		}
	case "manage_filters":
		if !perms.CanManageFilters {
			return PermissionResult{false, "❌ Your role doesn't allow managing filters."}
		}
	case "view_logs":
		if !perms.CanViewLogs {
			return PermissionResult{false, "❌ Your role doesn't allow viewing moderation logs."}
		}
	case "change_settings":
		if !perms.CanChangeSettings {
			return PermissionResult{false, "❌ Your role doesn't allow changing group settings."}
		}
	}

	return PermissionResult{true, ""}
}

// IsAdminOrRole checks if user is a Telegram admin or has any bot admin role.
func IsAdminOrRole(bot *tgbotapi.BotAPI, chatID int64, userID int64) bool {
	if chatID == 0 || userID == 0 {
		return false
	}
	member, err := bot.GetChatMember(tgbotapi.GetChatMemberConfig{
		ChatConfigWithUser: tgbotapi.ChatConfigWithUser{ChatID: chatID, UserID: userID},
	})
	if err == nil && (member.Status == "creator" || member.Status == "administrator") {
		return true
	}
	return GetBotAdminPermissions(chatID, userID) != nil
}

// ==================== BOT ADMIN ROLES ====================

var (
	roleCache   = make(map[int64]map[int64]*models.AdminPermissions)
	roleCacheMu sync.RWMutex
)

// GetBotAdminPermissions returns the bot-level permissions for a user in a group, or nil if none.
func GetBotAdminPermissions(chatID int64, userID int64) *models.AdminPermissions {
	roleCacheMu.RLock()
	if chatRoles, ok := roleCache[chatID]; ok {
		if perms, ok2 := chatRoles[userID]; ok2 {
			roleCacheMu.RUnlock()
			return perms
		}
	}
	roleCacheMu.RUnlock()

	// Load from DB
	var role string
	var permJSON []byte
	err := database.Pool.QueryRow(context.Background(), `
		SELECT role, permissions FROM admin_roles
		WHERE chat_id = $1 AND user_id = $2
	`, chatID, userID).Scan(&role, &permJSON)

	if err != nil {
		return nil // not a bot admin
	}

	perms := models.PermissionsForRole(role)
	if len(permJSON) > 0 {
		// Override with custom permissions if set
		json.Unmarshal(permJSON, &perms)
	}

	roleCacheMu.Lock()
	if roleCache[chatID] == nil {
		roleCache[chatID] = make(map[int64]*models.AdminPermissions)
	}
	roleCache[chatID][userID] = &perms
	roleCacheMu.Unlock()

	return &perms
}

// SetBotAdminRole assigns a bot admin role to a user in a group.
func SetBotAdminRole(chatID int64, userID int64, role string, setBy int64) error {
	perms := models.PermissionsForRole(role)
	permJSON, _ := json.Marshal(perms)

	_, err := database.Pool.Exec(context.Background(), `
		INSERT INTO admin_roles (chat_id, user_id, role, permissions, set_by, created_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (chat_id, user_id) DO UPDATE
		SET role = $3, permissions = $4, set_by = $5
	`, chatID, userID, role, permJSON, setBy)
	if err != nil {
		return err
	}

	// Invalidate cache
	roleCacheMu.Lock()
	if roleCache[chatID] != nil {
		delete(roleCache[chatID], userID)
	}
	roleCacheMu.Unlock()

	return nil
}

// RemoveBotAdminRole removes any bot admin role from a user.
func RemoveBotAdminRole(chatID int64, userID int64) error {
	_, err := database.Pool.Exec(context.Background(),
		"DELETE FROM admin_roles WHERE chat_id = $1 AND user_id = $2", chatID, userID)
	if err != nil {
		return err
	}

	roleCacheMu.Lock()
	if roleCache[chatID] != nil {
		delete(roleCache[chatID], userID)
	}
	roleCacheMu.Unlock()

	return nil
}

// ==================== GROUP REGISTRY ====================

// UpsertGroup records or updates a group in the groups table.
func UpsertGroup(chat *tgbotapi.Chat) {
	if chat == nil || chat.ID >= 0 {
		return
	}
	_, err := database.Pool.Exec(context.Background(), `
		INSERT INTO groups (chat_id, title, username, chat_type, is_active, updated_at)
		VALUES ($1, $2, $3, $4, true, NOW())
		ON CONFLICT (chat_id) DO UPDATE
		SET title = $2, username = $3, chat_type = $4, is_active = true, updated_at = NOW()
	`, chat.ID, chat.Title, chat.UserName, chat.Type)
	if err != nil {
		log.Printf("⚠️ UpsertGroup error for %d: %v", chat.ID, err)
	}
}

// UpsertUser records or updates a user in the users table.
func UpsertUser(user *tgbotapi.User) {
	if user == nil {
		return
	}
	_, err := database.Pool.Exec(context.Background(), `
		INSERT INTO users (user_id, username, first_name, last_name, is_bot, last_seen)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (user_id) DO UPDATE
		SET username = $2, first_name = $3, last_name = $4, is_bot = $5, last_seen = NOW()
	`, user.ID, user.UserName, user.FirstName, user.LastName, user.IsBot)
	if err != nil {
		log.Printf("⚠️ UpsertUser error for %d: %v", user.ID, err)
	}
}
