// gitrescue 是一个只读的 git 事故诊断工具。
//
// 用法：gitrescue [flags]
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/hahaha678/gitrescue/internal/gitcmd"
	"github.com/hahaha678/gitrescue/internal/i18n"
	"github.com/hahaha678/gitrescue/internal/render"
	"github.com/hahaha678/gitrescue/internal/scan"
	"github.com/hahaha678/gitrescue/internal/style"
)

// version 在发布时用 -ldflags "-X main.version=v1.2.3" 覆盖。
var version = "dev"

func main() {
	i18n.FromEnv()

	var (
		dir     = flag.String("C", ".", "在哪个目录检查（默认当前目录）")
		asJSON  = flag.Bool("json", false, "输出 JSON，方便脚本处理")
		explain = flag.Bool("explain", false, "同时解释每种情况的成因和后果")
		noColor = flag.Bool("no-color", false, "不输出颜色（也会识别 NO_COLOR 环境变量）")
		deep    = flag.Bool("deep", false, "给 git fsck 更长的超时，适合大仓库")
		strict  = flag.Bool("strict", false, "发现问题时用退出码 1 结束，可用于 CI")
		lang    = flag.String("lang", "", "界面语言：zh 或 en")
		ver     = flag.Bool("version", false, "打印版本号")
	)
	flag.Usage = usage
	flag.Parse()

	if *ver {
		fmt.Printf("gitrescue %s\n", version)
		return
	}
	if *lang != "" && !i18n.Set(*lang) {
		fmt.Fprintf(os.Stderr, "unknown language %q, expected zh or en\n", *lang)
		os.Exit(2)
	}
	if *noColor || *asJSON {
		style.Disable()
	}

	repo, err := gitcmd.Open(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s%s\n", i18n.T("这里不是 git 仓库：", "not a git repository: "), *dir)
		os.Exit(2)
	}

	report := scan.Run(repo, *deep)

	if *asJSON {
		if err := render.JSON(os.Stdout, report); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	} else {
		render.Human(os.Stdout, report, *explain)
	}

	if *strict && report.MaxSeverity() >= scan.SeverityDanger {
		os.Exit(1)
	}
}

func usage() {
	out := os.Stderr
	fmt.Fprintf(out, "%s\n\n", i18n.T(
		"gitrescue — 只读地诊断 git 仓库，告诉你「现在到底出了什么事」。",
		"gitrescue — a read-only diagnosis of what is actually wrong with your repository.",
	))
	fmt.Fprintf(out, "%s\n", i18n.T("用法：", "usage:"))
	fmt.Fprintf(out, "  gitrescue [选项]\n\n")
	fmt.Fprintf(out, "%s\n", i18n.T("选项：", "flags:"))

	cases := []struct{ name, zh, en string }{
		{"-C <dir>", "在哪个目录检查（默认当前目录）", "directory to scan (default: current)"},
		{"-json", "输出 JSON，方便脚本处理", "output JSON"},
		{"-explain", "解释每种情况的成因和后果", "explain what each finding means"},
		{"-no-color", "不输出颜色", "disable colors"},
		{"-deep", "给 git fsck 更长的超时，适合大仓库", "longer fsck timeout, for large repositories"},
		{"-strict", "发现问题时退出码为 1", "exit with code 1 when something needs action"},
		{"-lang zh|en", "界面语言", "interface language"},
		{"-version", "打印版本号", "print version"},
	}
	for _, c := range cases {
		fmt.Fprintf(out, "  %-14s %s\n", c.name, i18n.T(c.zh, c.en))
	}

	fmt.Fprintf(out, "\n%s\n", i18n.T("示例：", "examples:"))
	fmt.Fprintf(out, "  gitrescue              # 检查当前仓库\n")
	fmt.Fprintf(out, "  gitrescue --explain    # 顺便告诉你为什么\n")
	fmt.Fprintf(out, "  gitrescue --json       # 喂给脚本或 CI\n")
}
