package render

import (
	"strings"
	"testing"
)

func TestRuneWidth(t *testing.T) {
	cases := map[rune]int{
		'a':  1,
		'─':  1,
		'中':  2,
		'，':  2,
		'✗':  1,
		'\t': 0,
	}
	for r, want := range cases {
		if got := runeWidth(r); got != want {
			t.Errorf("runeWidth(%q) = %d, want %d", r, got, want)
		}
	}
}

func TestPadUsesDisplayWidth(t *testing.T) {
	got := pad("中文", 8)
	if w := displayWidth(got); w != 8 {
		t.Errorf("padded width = %d, want 8 (%q)", w, got)
	}
	if !strings.HasPrefix(got, "中文") {
		t.Errorf("pad must not reorder text: %q", got)
	}
}

func TestWrapNeverExceedsWidth(t *testing.T) {
	text := strings.Repeat("中文混排 english words 一起折行，", 5)
	for _, limit := range []int{12, 20, 40} {
		for _, line := range wrap(text, limit) {
			if w := displayWidth(line); w > limit {
				t.Errorf("limit %d: line %q has width %d", limit, line, w)
			}
		}
	}
}

func TestWrapKeepsContent(t *testing.T) {
	text := "abcdefghij klmnopqrst uvwxyz"
	joined := strings.Join(wrap(text, 10), "")
	if strings.Join(strings.Fields(joined), "") != strings.Join(strings.Fields(text), "") {
		t.Errorf("wrap lost or duplicated characters: %q", joined)
	}
}
