package services

import (
	"testing"

	"minimate-bot/models"
)

func TestPermissionsForRole(t *testing.T) {
	ownerPerms := models.PermissionsForRole(models.RoleOwner)
	if !ownerPerms.CanBan || !ownerPerms.CanMute || !ownerPerms.CanDelete {
		t.Errorf("Owner role should have full permissions, got %+v", ownerPerms)
	}

	modPerms := models.PermissionsForRole(models.RoleModerator)
	if !modPerms.CanBan || !modPerms.CanMute {
		t.Errorf("Moderator should have ban and mute permissions, got %+v", modPerms)
	}
	if modPerms.CanChangeSettings {
		t.Errorf("Moderator should NOT have change settings permission")
	}

	chatModPerms := models.PermissionsForRole(models.RoleChatModerator)
	if chatModPerms.CanBan {
		t.Errorf("Chat moderator should NOT have ban permission")
	}
	if !chatModPerms.CanDelete || !chatModPerms.CanWarn {
		t.Errorf("Chat moderator should have delete and warn permissions")
	}
}

func TestCountEmojis(t *testing.T) {
	testCases := []struct {
		text     string
		expected int
	}{
		{"Hello world", 0},
		{"Hello 😀 world 🚀", 2},
		{"🎉✨🔥👍🛡️", 5},
		{"No emojis here 12345!@#", 0},
	}

	for _, tc := range testCases {
		got := countEmojis(tc.text)
		if got != tc.expected {
			t.Errorf("countEmojis(%q) = %d; want %d", tc.text, got, tc.expected)
		}
	}
}
