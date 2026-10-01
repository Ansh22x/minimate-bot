// Package models defines shared data structures used across services and handlers.
package models

import "time"

// ==================== GROUP ====================

// Group represents a Telegram chat the bot manages.
type Group struct {
	ChatID      int64     `db:"chat_id"`
	Title       string    `db:"title"`
	Username    string    `db:"username"`
	ChatType    string    `db:"chat_type"`
	MemberCount int       `db:"member_count"`
	IsActive    bool      `db:"is_active"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

// ==================== USER ====================

// User represents a Telegram user seen by the bot.
type User struct {
	UserID    int64     `db:"user_id"`
	Username  string    `db:"username"`
	FirstName string    `db:"first_name"`
	LastName  string    `db:"last_name"`
	IsBot     bool      `db:"is_bot"`
	IsPremium bool      `db:"is_premium"`
	FirstSeen time.Time `db:"first_seen"`
	LastSeen  time.Time `db:"last_seen"`
}

// DisplayName returns a friendly display name for the user.
func (u *User) DisplayName() string {
	if u.FirstName != "" {
		return u.FirstName
	}
	if u.Username != "" {
		return "@" + u.Username
	}
	return "Unknown"
}

// ==================== MODERATION LOG ====================

// ModerationAction constants
const (
	ActionBan         = "ban"
	ActionTempBan     = "tempban"
	ActionUnban       = "unban"
	ActionKick        = "kick"
	ActionMute        = "mute"
	ActionTempMute    = "tempmute"
	ActionUnmute      = "unmute"
	ActionRestrict    = "restrict"
	ActionUnrestrict  = "unrestrict"
	ActionWarn        = "warn"
	ActionUnwarn      = "unwarn"
	ActionResetWarns  = "resetwarns"
	ActionDeleteMsg   = "delete"
	ActionLockdown    = "lockdown"
	ActionUnlockdown  = "unlockdown"
	ActionAutoMute    = "auto_mute"
	ActionAutoBan     = "auto_ban"
	ActionRaidAction  = "raid_action"
)

// ModLog is a single moderation log entry.
type ModLog struct {
	ID        int64                  `db:"id"`
	ChatID    int64                  `db:"chat_id"`
	UserID    int64                  `db:"user_id"`
	ModID     int64                  `db:"mod_id"`
	Action    string                 `db:"action"`
	Reason    string                 `db:"reason"`
	Extra     map[string]interface{} `db:"extra"`
	CreatedAt time.Time              `db:"created_at"`
	ExpiresAt *time.Time             `db:"expires_at"`
	Undone    bool                   `db:"undone"`
	Undoneby  *int64                 `db:"undone_by"`
	UndoneAt  *time.Time             `db:"undone_at"`
}

// ==================== TEMP ACTION ====================

// TempActionType constants
const (
	TempActionMute     = "mute"
	TempActionBan      = "ban"
	TempActionRestrict = "restrict"
)

// TempAction represents a time-limited moderation action to be undone by a worker.
type TempAction struct {
	ID         int64      `db:"id"`
	ChatID     int64      `db:"chat_id"`
	UserID     int64      `db:"user_id"`
	ActionType string     `db:"action_type"`
	LogID      *int64     `db:"log_id"`
	ExpiresAt  time.Time  `db:"expires_at"`
	Done       bool       `db:"done"`
	CreatedAt  time.Time  `db:"created_at"`
}

// ==================== WARN ====================

// WarnEntry is a single warning issued to a user.
type WarnEntry struct {
	ID        int64      `db:"id"`
	ChatID    int64      `db:"chat_id"`
	UserID    int64      `db:"user_id"`
	ModID     int64      `db:"mod_id"`
	Reason    string     `db:"reason"`
	Active    bool       `db:"active"`
	CreatedAt time.Time  `db:"created_at"`
	ExpiresAt *time.Time `db:"expires_at"`
}

// ==================== GROUP SETTINGS ====================

// GroupSettings holds per-group configuration in a structured form.
// Stored as JSONB in the database.
type GroupSettings struct {
	// Anti-spam
	AntiSpamEnabled    bool `json:"antispam_enabled"`
	FloodLimit         int  `json:"flood_limit"`          // messages
	FloodWindow        int  `json:"flood_window"`          // seconds
	FloodMuteDuration  int  `json:"flood_mute_duration"`   // seconds
	RepeatLimit        int  `json:"repeat_limit"`          // same message N times
	EmojiLimit         int  `json:"emoji_limit"`           // per message
	StickerLimit       int  `json:"sticker_limit"`         // per window
	MentionLimit       int  `json:"mention_limit"`         // per message
	MediaLimit         int  `json:"media_limit"`           // per window

	// Anti-raid
	AntiRaidEnabled    bool   `json:"antiraid_enabled"`
	RaidSensitivity    string `json:"raid_sensitivity"`      // LOW, MEDIUM, HIGH, CUSTOM
	RaidJoinLimit      int    `json:"raid_join_limit"`       // joins in window
	RaidWindow         int    `json:"raid_window"`           // seconds

	// Warn thresholds
	WarnLimit          int    `json:"warn_limit"`            // default 3
	WarnThreshold3     string `json:"warn_threshold_3"`      // action at 3 warns
	WarnThreshold5     string `json:"warn_threshold_5"`
	WarnThreshold7     string `json:"warn_threshold_7"`
	WarnThreshold10    string `json:"warn_threshold_10"`
	WarnExpiry         int    `json:"warn_expiry"`           // days, 0=never
	WarnAutoExpire     bool   `json:"warn_auto_expire"`

	// Welcome
	WelcomeAutoDelete  int  `json:"welcome_auto_delete"`   // seconds, 0=off

	// Cleanup
	DeleteJoinMessages bool `json:"delete_join_messages"`
	DeleteLeaveMessages bool `json:"delete_leave_messages"`
	DeleteCmdMessages  bool `json:"delete_cmd_messages"`

	// Log channel
	LogChannelID       int64 `json:"log_channel_id"`
}

// DefaultGroupSettings returns sensible defaults for a new group.
func DefaultGroupSettings() GroupSettings {
	return GroupSettings{
		AntiSpamEnabled:   false,
		FloodLimit:        5,
		FloodWindow:       3,
		FloodMuteDuration: 300,
		RepeatLimit:       3,
		EmojiLimit:        15,
		StickerLimit:      5,
		MentionLimit:      5,
		MediaLimit:        10,

		AntiRaidEnabled:  false,
		RaidSensitivity:  "MEDIUM",
		RaidJoinLimit:    10,
		RaidWindow:       30,

		WarnLimit:       3,
		WarnThreshold3:  "mute:300",
		WarnThreshold5:  "mute:3600",
		WarnThreshold7:  "tempban:86400",
		WarnThreshold10: "ban",
		WarnExpiry:      0,
		WarnAutoExpire:  false,

		WelcomeAutoDelete: 0,

		DeleteJoinMessages:  false,
		DeleteLeaveMessages: false,
		DeleteCmdMessages:   false,

		LogChannelID: 0,
	}
}

// ==================== ADMIN ROLES ====================

// Role constants
const (
	RoleOwner          = "owner"
	RoleHeadModerator  = "head_moderator"
	RoleModerator      = "moderator"
	RoleChatModerator  = "chat_moderator"
	RoleLogManager     = "log_manager"
	RoleSettingsManager = "settings_manager"
)

// AdminPermissions defines what a bot admin role can do.
type AdminPermissions struct {
	CanBan            bool `json:"can_ban"`
	CanMute           bool `json:"can_mute"`
	CanKick           bool `json:"can_kick"`
	CanDelete         bool `json:"can_delete"`
	CanWarn           bool `json:"can_warn"`
	CanManageFilters  bool `json:"can_manage_filters"`
	CanViewLogs       bool `json:"can_view_logs"`
	CanChangeSettings bool `json:"can_change_settings"`
	CanManageMods     bool `json:"can_manage_mods"`
	CanManageVerify   bool `json:"can_manage_verify"`
	CanViewAnalytics  bool `json:"can_view_analytics"`
}

// PermissionsForRole returns the default permissions for a given role.
func PermissionsForRole(role string) AdminPermissions {
	switch role {
	case RoleOwner, RoleHeadModerator:
		return AdminPermissions{true, true, true, true, true, true, true, true, true, true, true}
	case RoleModerator:
		return AdminPermissions{CanBan: true, CanMute: true, CanKick: true, CanDelete: true, CanWarn: true, CanViewLogs: true}
	case RoleChatModerator:
		return AdminPermissions{CanDelete: true, CanWarn: true, CanViewLogs: true}
	case RoleLogManager:
		return AdminPermissions{CanViewLogs: true, CanViewAnalytics: true}
	case RoleSettingsManager:
		return AdminPermissions{CanManageFilters: true, CanChangeSettings: true, CanManageVerify: true}
	default:
		return AdminPermissions{}
	}
}

// ==================== PAGINATION ====================

// Page holds pagination context for dashboard lists.
type Page struct {
	Items    interface{}
	Current  int
	Total    int
	PageSize int
}

func (p Page) HasPrev() bool { return p.Current > 1 }
func (p Page) HasNext() bool { return p.Current < p.Total }
