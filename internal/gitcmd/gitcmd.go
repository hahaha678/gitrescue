// Package gitcmd 是对 git 命令的一层薄封装。
//
// 所有调用都是只读的：强制关掉交互提示和可选锁，避免碰到用户仓库。
package gitcmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ErrNotRepo 表示目标目录不在 git 仓库里。
var ErrNotRepo = errors.New("not a git repository")

// Repo 指向一个已存在的仓库。
type Repo struct {
	WorkTree string // 仓库根目录（绝对路径）
	GitDir   string // .git 目录（absolute-git-dir，worktree 里不是 .git）
}

// Open 在 dir 下定位仓库，会向上查找父目录。
func Open(dir string) (*Repo, error) {
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	top, err := run(dir, 10*time.Second, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, ErrNotRepo
	}
	gitDir, err := run(dir, 10*time.Second, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return nil, ErrNotRepo
	}
	return &Repo{
		WorkTree: strings.TrimSpace(top),
		GitDir:   strings.TrimSpace(gitDir),
	}, nil
}

// Run 执行 git 子命令并返回 stdout。
func (r *Repo) Run(args ...string) (string, error) {
	return run(r.WorkTree, 30*time.Second, args...)
}

// RunT 同 Run，但可以指定超时。
func (r *Repo) RunT(timeout time.Duration, args ...string) (string, error) {
	return run(r.WorkTree, timeout, args...)
}

// Try 执行命令，失败时把错误吞掉。用于"探测状态"这类场景。
func (r *Repo) Try(args ...string) (string, bool) {
	out, err := run(r.WorkTree, 30*time.Second, args...)
	return out, err == nil
}

// Output 返回 stdout/stderr 和错误，stdout 即使命令失败也会返回。
// git fsck 这类"有输出但退出码非 0"的命令需要它。
func (r *Repo) Output(timeout time.Duration, args ...string) (string, string, error) {
	return runAll(r.WorkTree, timeout, args...)
}

// Path 拼接 .git 目录下的路径。
func (r *Repo) Path(parts ...string) string {
	return filepath.Join(append([]string{r.GitDir}, parts...)...)
}

// Exists 判断 .git 下的某个路径是否存在。
func (r *Repo) Exists(parts ...string) bool {
	_, err := os.Stat(r.Path(parts...))
	return err == nil
}

func run(dir string, timeout time.Duration, args ...string) (string, error) {
	out, _, err := runAll(dir, timeout, args...)
	return out, err
}

func runAll(dir string, timeout time.Duration, args ...string) (string, string, error) {
	if len(args) == 0 {
		return "", "", errors.New("gitcmd: empty command")
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	full := make([]string, 0, len(args)+3)
	full = append(full, "-C", dir, "--no-pager")
	full = append(full, args...)

	cmd := exec.CommandContext(ctx, "git", full...)
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0", // 绝不弹认证提示
		"GIT_PAGER=cat",
		"GIT_OPTIONAL_LOCKS=0", // 不写 index 锁
		"LC_ALL=C",             // 输出稳定，便于解析
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return stdout.String(), stderr.String(), fmt.Errorf("git %s: 超时（%s）", args[0], timeout)
	}
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return stdout.String(), stderr.String(), fmt.Errorf("git %s: %s", args[0], msg)
	}
	return stdout.String(), stderr.String(), nil
}
