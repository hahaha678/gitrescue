// Package render 负责把检查结果画到终端上，或者转成 JSON。
package render

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"gitrescue/internal/i18n"
	"gitrescue/internal/scan"
	"gitrescue/internal/style"
)

// width 是报告正文的宽度（显示列数，不是字节数）。
const width = 68

// JSON 输出机器可读的结果。
func JSON(w io.Writer, r scan.Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(r)
}

// Human 输出给人看的报告。
func Human(w io.Writer, r scan.Report, explain bool) {
	bw := bufio.NewWriter(w)
	defer bw.Flush()

	head := style.Bold("gitrescue")
	switch {
	case r.Branch != "" && r.Head != "":
		head += style.Dim("  ") + style.Cyan(r.Branch) + style.Dim(" @ "+r.Head)
	case r.Branch != "":
		head += style.Dim("  ") + style.Cyan(r.Branch)
	case r.Head != "":
		head += style.Dim("  detached @ ") + style.Cyan(r.Head)
	}
	fmt.Fprintln(bw, head)
	fmt.Fprintln(bw, style.Dim(shortenPath(r.Repo)))
	fmt.Fprintln(bw, style.Gray(strings.Repeat("─", width)))
	fmt.Fprintln(bw)

	if len(r.Findings) == 0 {
		fmt.Fprintf(bw, "  %s  %s\n\n", style.Green("✓"),
			style.Green(i18n.T("没发现问题，可以放心继续", "no problems found")))
	}
	for _, f := range r.Findings {
		renderFinding(bw, f, explain)
	}

	if len(r.Notes) > 0 {
		for _, n := range r.Notes {
			fmt.Fprintln(bw, style.Dim(i18n.T("提示：", "note: ")+n))
		}
		fmt.Fprintln(bw)
	}

	fmt.Fprintln(bw, style.Gray(strings.Repeat("─", width)))
	fmt.Fprintln(bw, summary(r))
}

func summary(r scan.Report) string {
	var parts []string
	if d := r.Count(scan.SeverityDanger); d > 0 {
		parts = append(parts, style.Red(fmt.Sprintf(i18n.T("%d 处需要立刻处理", "%d need(s) action"), d)))
	}
	if w := r.Count(scan.SeverityWarn); w > 0 {
		parts = append(parts, style.Yellow(fmt.Sprintf(i18n.T("%d 处值得看一眼", "%d worth a look"), w)))
	}
	if len(parts) == 0 {
		parts = append(parts, style.Green(i18n.T("一切正常", "all clear")))
	}
	parts = append(parts, style.Dim(i18n.T("只读检查，没有改动任何东西", "read-only, nothing was modified")))
	return strings.Join(parts, style.Dim(" · "))
}

func renderFinding(w io.Writer, f scan.Finding, explain bool) {
	sym, color := symbolFor(f.Severity)
	fmt.Fprintf(w, "  %s  %s\n", color(sym), style.Bold(f.Title))

	for _, d := range f.Detail {
		fmt.Fprintf(w, "     %s\n", style.Dim(d))
	}

	if len(f.Commands) > 0 {
		fmt.Fprintln(w)
		// 标签长短不一，按最长的那个对齐，命令本身就能排成一条线。
		labelWidth := 0
		for _, c := range f.Commands {
			if n := displayWidth(c.Label); n > labelWidth {
				labelWidth = n
			}
		}
		for _, c := range f.Commands {
			fmt.Fprintf(w, "     %s   %s\n", style.Gray(pad(c.Label, labelWidth)), style.Cyan(c.Line))
		}
	}

	if explain && f.Learn != "" {
		fmt.Fprintln(w)
		for _, line := range wrap(f.Learn, width-6) {
			fmt.Fprintf(w, "     %s\n", style.Gray(line))
		}
	}
	fmt.Fprintln(w)
}

func symbolFor(s scan.Severity) (string, func(string) string) {
	switch s {
	case scan.SeverityDanger:
		return "✗", style.Red
	case scan.SeverityWarn:
		return "!", style.Yellow
	case scan.SeverityInfo:
		return "·", style.Cyan
	default:
		return "✓", style.Green
	}
}

// pad 用空格把 s 补齐到 width 个显示列。
func pad(s string, width int) string {
	d := displayWidth(s)
	if d >= width {
		return s
	}
	return s + strings.Repeat(" ", width-d)
}

func displayWidth(s string) int {
	n := 0
	for _, r := range s {
		n += runeWidth(r)
	}
	return n
}

// runeWidth 估算一个字符在终端里占几列。够用就行，不追求完整 Unicode 表。
func runeWidth(r rune) int {
	switch {
	case r == 0 || r < 32 || (r >= 0x7f && r < 0xa0):
		return 0
	case r < 0x1100:
		return 1
	case unicode.Is(unicode.Han, r),
		unicode.Is(unicode.Hiragana, r),
		unicode.Is(unicode.Katakana, r),
		unicode.Is(unicode.Hangul, r):
		return 2
	case r >= 0x2e80 && r <= 0x303e, // CJK 部首、标点
		r >= 0x3041 && r <= 0x33ff,
		r >= 0xfe30 && r <= 0xfe6f,
		r >= 0xff00 && r <= 0xff60,
		r >= 0xffe0 && r <= 0xffe6:
		return 2
	default:
		return 1
	}
}

// wrap 按显示宽度折行。没有空格的 CJK 文本直接在任意字符处断开。
func wrap(s string, limit int) []string {
	var lines []string
	var cur []rune
	used := 0

	for _, r := range s {
		rw := runeWidth(r)
		if used+rw > limit && len(cur) > 0 {
			if idx := lastSpace(cur); idx > len(cur)/2 {
				lines = append(lines, string(cur[:idx]))
				cur = cur[idx+1:]
				used = displayWidth(string(cur))
			} else {
				lines = append(lines, string(cur))
				cur = cur[:0]
				used = 0
			}
		}
		cur = append(cur, r)
		used += rw
	}
	if len(cur) > 0 {
		lines = append(lines, string(cur))
	}
	return lines
}

func lastSpace(rs []rune) int {
	for i := len(rs) - 1; i >= 0; i-- {
		if rs[i] == ' ' {
			return i
		}
	}
	return -1
}

func shortenPath(p string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return p
	}
	rel, err := filepath.Rel(home, p)
	if err != nil {
		return p
	}
	if rel == "." {
		return "~"
	}
	if strings.HasPrefix(rel, "..") {
		return p
	}
	return "~" + string(filepath.Separator) + rel
}
