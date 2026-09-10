package handlers

import (
	"testing"
)

func TestLockNormalization(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"links", "url"},
		{"URLS", "url"},
		{"invites", "invitelink"},
		{"forwards", "forward"},
		{"fwdchannel", "forwardchannel"},
		{"stickers", "sticker"},
		{"gif", "gif"},
		{"cjk", "cjk"},
		{"chinese", "cjk"},
		{"russian", "cyrillic"},
		{"arabic", "rtl"},
		{"glitch", "zalgo"},
		{"all", "all"},
		{"media", "media"},
		{"photo", "photo"},
		{"videos", "video"},
		{"roundvideo", "videonote"},
		{"collages", "album"},
		{"guestbot", "bot"},
		{"channels", "anonchannel"},
	}

	for _, tt := range tests {
		got := normalizeLockKey(tt.input)
		if got != tt.expected {
			t.Errorf("normalizeLockKey(%q) = %q; want %q", tt.input, got, tt.expected)
		}
	}
}

func TestScriptRegexes(t *testing.T) {
	// CJK
	if !reCJK.MatchString("こんにちは") {
		t.Errorf("reCJK failed to match Japanese text")
	}
	if !reCJK.MatchString("你好世界") {
		t.Errorf("reCJK failed to match Chinese text")
	}
	if !reCJK.MatchString("안녕하세요") {
		t.Errorf("reCJK failed to match Korean text")
	}
	if reCJK.MatchString("Hello World 123") {
		t.Errorf("reCJK incorrectly matched English text")
	}

	// Cyrillic
	if !reCyrillic.MatchString("Привет мир") {
		t.Errorf("reCyrillic failed to match Russian text")
	}
	if reCyrillic.MatchString("Hello World") {
		t.Errorf("reCyrillic incorrectly matched English text")
	}

	// RTL
	if !reRTL.MatchString("مرحبا بالعالم") {
		t.Errorf("reRTL failed to match Arabic text")
	}
	if !reRTL.MatchString("שלום עולם") {
		t.Errorf("reRTL failed to match Hebrew text")
	}
	if reRTL.MatchString("Hello World") {
		t.Errorf("reRTL incorrectly matched English text")
	}

	// Zalgo (stacked combining marks)
	zalgoSample := "H\u0300\u0301\u0302ello"
	if !reZalgo.MatchString(zalgoSample) {
		t.Errorf("reZalgo failed to match zalgo text: %s", zalgoSample)
	}

	// Email
	if !reEmail.MatchString("contact@test.com") {
		t.Errorf("reEmail failed to match email")
	}

	// Bot Link
	if !reBotLink.MatchString("t.me/SomeSpamBot") {
		t.Errorf("reBotLink failed to match bot link")
	}
}

func TestEmojiDetection(t *testing.T) {
	if !containsEmoji("Hello 😊 World") {
		t.Errorf("containsEmoji failed to detect emoji in string")
	}
	if containsEmoji("Hello World without emoji") {
		t.Errorf("containsEmoji falsely detected emoji")
	}

	if !isEmojiOnly("😀🔥🚀") {
		t.Errorf("isEmojiOnly failed for pure emoji string")
	}
	if !isEmojiOnly("  😀 🔥  ") {
		t.Errorf("isEmojiOnly failed for emoji string with spaces")
	}
	if isEmojiOnly("😀 Hello") {
		t.Errorf("isEmojiOnly falsely validated mixed emoji and text")
	}
}
