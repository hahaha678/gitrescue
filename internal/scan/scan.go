// Package scan 检查一个 git 仓库当前的健康状况。
//
// 设计原则：只读。所有检测都通过 git 的查询命令和 .git 目录下的状态文件完成，
// 不会修改仓库、不会写 index、不会联网。
package scan

import (
	"encoding/json"
	"sort"
	"strings"
	"time"

	"gitrescue/internal/gitcmd"
)

// Severity 描述一条发现的紧急程度。
type Severity int

const (
	// SeverityOK 表示没有问题。
	SeverityOK Severity = iota
	// SeverityInfo 是提示，不影响继续工作。
	SeverityInfo
	// SeverityWarn 值得看一眼，可能是隐患。
	SeverityWarn
	// SeverityDanger 正在丢工作的现场，需要立刻处理。
	SeverityDanger
)

func (s Severity) String() string {
	switch s {
	case SeverityDanger:
		return "danger"
	case SeverityWarn:
		return "warn"
	case SeverityInfo:
		return "info"
	default:
		return "ok"
	}
}

// Command 是一条可以复制粘贴执行的恢复命令。
type Command struct {
	Label string `json:"label"`
	Line  string `json:"line"`
}

// Finding 是一条检查结果。
type Finding struct {
	Kind     string // 机器可读的类型，如 "rebase"、"lost-commits"
	Severity Severity
	Title    string
	Detail   []string
	Commands []Command
	Learn    string // --explain 时显示的背景知识
}

// MarshalJSON 把 Severity 转成可读的字符串。
func (f Finding) MarshalJSON() ([]byte, error) {
	type dto struct {
		Kind     string    `json:"kind"`
		Level    string    `json:"level"`
		Title    string    `json:"title"`
		Detail   []string  `json:"detail,omitempty"`
		Commands []Command `json:"commands,omitempty"`
		Learn    string    `json:"learn,omitempty"`
	}
	return json.Marshal(dto{
		Kind:     f.Kind,
		Level:    f.Severity.String(),
		Title:    f.Title,
		Detail:   f.Detail,
		Commands: f.Commands,
		Learn:    f.Learn,
	})
}

// Report 是一次完整检查的结果。
type Report struct {
	Repo     string    `json:"repo"`
	Branch   string    `json:"branch,omitempty"`
	Head     string    `json:"head,omitempty"`
	Detached bool      `json:"detached"`
	Findings []Finding `json:"findings"`
	Notes    []string  `json:"notes,omitempty"`
	Elapsed  string    `json:"elapsed"`
}

// MaxSeverity 返回最高紧急程度。
func (r Report) MaxSeverity() Severity {
	max := SeverityOK
	for _, f := range r.Findings {
		if f.Severity > max {
			max = f.Severity
		}
	}
	return max
}

// Count 返回某个紧急程度以上的发现数量。
func (r Report) Count(min Severity) int {
	n := 0
	for _, f := range r.Findings {
		if f.Severity >= min {
			n++
		}
	}
	return n
}

// Run 依次跑完所有检测。deep 为 true 时会给 git fsck 更长的超时。
func Run(repo *gitcmd.Repo, deep bool) Report {
	start := time.Now()
	rep := Report{Repo: repo.WorkTree}

	if out, ok := repo.Try("symbolic-ref", "-q", "--short", "HEAD"); ok {
		rep.Branch = strings.TrimSpace(out)
	} else {
		rep.Detached = true
	}
	if out, ok := repo.Try("rev-parse", "--short", "HEAD"); ok {
		rep.Head = strings.TrimSpace(out)
	}

	// 空仓库没有 HEAD，后面大部分检查都不适用。
	if rep.Head == "" {
		rep.Findings = append(rep.Findings, emptyRepo(repo)...)
		rep.Elapsed = time.Since(start).Round(time.Millisecond).String()
		return rep
	}

	rep.Findings = append(rep.Findings, inProgress(repo)...)
	rep.Findings = append(rep.Findings, detachedHead(repo, rep.Detached)...)
	rep.Findings = append(rep.Findings, unpushed(repo, rep.Branch)...)
	rep.Findings = append(rep.Findings, stashed(repo)...)

	lostFindings, note := lostCommits(repo, deep)
	rep.Findings = append(rep.Findings, lostFindings...)
	if note != "" {
		rep.Notes = append(rep.Notes, note)
	}

	rep.Findings = append(rep.Findings, workingCopy(repo)...)

	// 紧急的排前面，同级保持检测顺序。
	sort.SliceStable(rep.Findings, func(i, j int) bool {
		return rep.Findings[i].Severity > rep.Findings[j].Severity
	})
	rep.Elapsed = time.Since(start).Round(time.Millisecond).String()
	return rep
}
