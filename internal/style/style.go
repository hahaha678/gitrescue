// Package style 做终端配色。零依赖，只有 ANSI 转义序列。
package style

import (
	"os"
	"strings"
)

var enabled = detect()

func detect() bool {
	if os.Getenv("FORCE_COLOR") != "" {
		return true
	}
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if strings.EqualFold(os.Getenv("TERM"), "dumb") {
		return false
	}
	// 输出被重定向到文件或管道时不要塞满转义序列。
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// Enabled 报告当前是否输出颜色。
func Enabled() bool { return enabled }

// Disable 关掉颜色。
func Disable() { enabled = false }

func wrap(code, s string) string {
	if !enabled || s == "" {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

func Bold(s string) string   { return wrap("1", s) }
func Dim(s string) string    { return wrap("2", s) }
func Red(s string) string    { return wrap("91", s) }
func Yellow(s string) string { return wrap("93", s) }
func Green(s string) string  { return wrap("92", s) }
func Cyan(s string) string   { return wrap("96", s) }
func Gray(s string) string   { return wrap("90", s) }
