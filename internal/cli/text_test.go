package cli

import (
	"testing"
	"unicode/utf8"
)

func TestTruncateTextASCII(t *testing.T) {
	tests := []struct {
		name  string
		value string
		limit int
		want  string
	}{
		{name: "under limit", value: "cat", limit: 4, want: "cat"},
		{name: "at limit", value: "cats", limit: 4, want: "cats"},
		{name: "over limit", value: "cats!", limit: 4, want: "cats..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := truncateText(tt.value, tt.limit); got != tt.want {
				t.Errorf("truncateText(%q, %d) = %q, want %q", tt.value, tt.limit, got, tt.want)
			}
		})
	}
}

func TestTruncateTextMultibyte(t *testing.T) {
	value := "héllo wörld 日本語テキスト"
	const limit = 13

	got := truncateText(value, limit)
	want := string([]rune(value)[:limit]) + "..."
	if got != want {
		t.Errorf("truncateText(%q, %d) = %q, want %q", value, limit, got, want)
	}
	if !utf8.ValidString(got) {
		t.Errorf("truncateText(%q, %d) returned invalid UTF-8: %q", value, limit, got)
	}
	if gotRunes := utf8.RuneCountInString(got); gotRunes != limit+3 {
		t.Errorf("truncateText(%q, %d) has %d runes, want %d", value, limit, gotRunes, limit+3)
	}

	atLimit := "日本語"
	if got := truncateText(atLimit, utf8.RuneCountInString(atLimit)); got != atLimit {
		t.Errorf("truncateText(%q, %d) = %q, want unchanged value", atLimit, utf8.RuneCountInString(atLimit), got)
	}
}
