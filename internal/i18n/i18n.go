// Package i18n 提供极简的双语支持。
//
// 消息直接以 (中文, English) 成对写在调用处，不维护 key 表，
// 这样改文案时不用同时跳两个文件。
package i18n

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Lang 是界面语言。
type Lang int

const (
	// ZH 中文（默认）。
	ZH Lang = iota
	// EN English.
	EN
)

var current = ZH

// Set 按名字切换语言，成功返回 true。
func Set(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "zh", "zh-cn", "zh_cn", "cn", "chinese", "中文":
		current = ZH
		return true
	case "en", "en-us", "en_us", "english":
		current = EN
		return true
	}
	return false
}

// FromEnv 读取 GITRESCUE_LANG，没设置时保持默认。
func FromEnv() {
	if v := os.Getenv("GITRESCUE_LANG"); v != "" {
		Set(v)
	}
}

// Get 返回当前语言。
func Get() Lang { return current }

// T 按当前语言二选一。
func T(zh, en string) string {
	if current == EN {
		return en
	}
	return zh
}

// Ago 把 Unix 时间戳格式化成"3 分钟前"这样的相对时间。
func Ago(unix int64) string {
	if unix <= 0 {
		return T("未知时间", "unknown time")
	}
	d := time.Since(time.Unix(unix, 0))
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return T("刚刚", "just now")
	case d < time.Hour:
		return fmt.Sprintf(T("%d 分钟前", "%dm ago"), int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf(T("%d 小时前", "%dh ago"), int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf(T("%d 天前", "%dd ago"), int(d.Hours()/24))
	case d < 365*24*time.Hour:
		return fmt.Sprintf(T("%d 个月前", "%dmo ago"), int(d.Hours()/24/30))
	default:
		return fmt.Sprintf(T("%d 年前", "%dy ago"), int(d.Hours()/24/365))
	}
}
