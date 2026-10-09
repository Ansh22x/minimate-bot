package handlers

import (
	"testing"
	"time"
)

func TestToSmallCaps(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"SPAMPRO", "sᴘᴀᴍᴘʀᴏ"},
		{"spampro", "sᴘᴀᴍᴘʀᴏ"},
		{"MiniMate", "ᴍɪɴɪᴍᴀᴛᴇ"},
		{"v.01", "ᴠ.01"},
		{"System Status", "sʏsᴛᴇᴍ sᴛᴀᴛᴜs"},
	}

	for _, tt := range tests {
		got := toSmallCaps(tt.input)
		if got != tt.expected {
			t.Errorf("toSmallCaps(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestFormatUptime(t *testing.T) {
	d := 7*24*time.Hour + 14*time.Hour + 26*time.Minute
	got := formatUptime(d)
	expected := "7d 14h 26m"
	if got != expected {
		t.Errorf("formatUptime(%v) = %q, want %q", d, got, expected)
	}

	d2 := 45 * time.Second
	got2 := formatUptime(d2)
	expected2 := "45s"
	if got2 != expected2 {
		t.Errorf("formatUptime(%v) = %q, want %q", d2, got2, expected2)
	}
}

func TestGetSystemMetrics(t *testing.T) {
	metrics := getSystemMetrics()
	if metrics.CPUCores <= 0 {
		t.Errorf("expected CPUCores > 0, got %d", metrics.CPUCores)
	}
	if metrics.GoVersion == "" {
		t.Errorf("expected GoVersion to be non-empty")
	}
	if metrics.Hostname == "" {
		t.Errorf("expected Hostname to be non-empty")
	}
}

func TestRenderPingReport(t *testing.T) {
	stats := getSystemMetrics()
	botName := "sᴘᴀᴍᴘʀᴏ"
	latency := int64(314)
	latencyRating := "🟡 ᴀᴠᴇʀᴀɢᴇ"

	report := formatPingReport(botName, latency, latencyRating, stats)
	t.Logf("\nGenerated Ping Report:\n%s\n", report)
}
