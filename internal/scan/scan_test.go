package scan_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gitrescue/internal/gitcmd"
	"gitrescue/internal/scan"
)

// git 在 dir 里跑一条 git 命令。用一次性的身份和环境，
// 避免读到开发机上真实的 ~/.gitconfig。
func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
		"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// newRepo 建一个空仓库并返回目录。
func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	return dir
}

func commit(t *testing.T, dir, file, content, msg string) {
	t.Helper()
	write(t, dir, file, content)
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-qm", msg)
}

func run(t *testing.T, dir string) scan.Report {
	t.Helper()
	repo, err := gitcmd.Open(dir)
	if err != nil {
		t.Fatalf("open repo: %v", err)
	}
	return scan.Run(repo, false)
}

func find(rep scan.Report, kind string) (scan.Finding, bool) {
	for _, f := range rep.Findings {
		if f.Kind == kind {
			return f, true
		}
	}
	return scan.Finding{}, false
}

func TestEmptyRepo(t *testing.T) {
	rep := run(t, newRepo(t))
	if _, ok := find(rep, "empty-repo"); !ok {
		t.Fatalf("empty repo should report an empty-repo finding, got %+v", rep.Findings)
	}
	if rep.MaxSeverity() > scan.SeverityInfo {
		t.Errorf("empty repo should not be alarming, got %v", rep.MaxSeverity())
	}
}

func TestLostCommitsAfterHardReset(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "f.txt", "1", "first")
	commit(t, dir, "f.txt", "2", "second")
	git(t, dir, "reset", "-q", "--hard", "HEAD~1")

	rep := run(t, dir)
	f, ok := find(rep, "lost-commits")
	if !ok {
		t.Fatalf("a hard reset should leave a recoverable commit, got %+v", rep.Findings)
	}
	if !strings.Contains(strings.Join(f.Detail, "\n"), "second") {
		t.Errorf("details should name the dropped commit, got %v", f.Detail)
	}
	if len(f.Commands) == 0 || !strings.Contains(f.Commands[len(f.Commands)-1].Line, "git branch rescue/") {
		t.Errorf("expected a rescue branch command, got %+v", f.Commands)
	}
}

func TestDetachedHeadWithCommits(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "f.txt", "1", "first")
	git(t, dir, "checkout", "-q", "--detach")
	commit(t, dir, "g.txt", "2", "work on detached head")

	rep := run(t, dir)
	f, ok := find(rep, "detached")
	if !ok {
		t.Fatalf("detached HEAD with unique commits must be reported, got %+v", rep.Findings)
	}
	if f.Severity != scan.SeverityDanger {
		t.Errorf("detached HEAD with unique commits should be danger, got %v", f.Severity)
	}
}

func TestDetachedHeadWithoutCommitsIsQuiet(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "f.txt", "1", "first")
	git(t, dir, "checkout", "-q", "--detach")

	if _, ok := find(run(t, dir), "detached"); ok {
		t.Error("a read-only detach should not be reported")
	}
}

func TestRebaseInProgress(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "f.txt", "base", "base")
	git(t, dir, "checkout", "-q", "-b", "topic")
	commit(t, dir, "f.txt", "topic", "topic change")
	git(t, dir, "checkout", "-q", "main")
	commit(t, dir, "f.txt", "main", "main change")
	git(t, dir, "checkout", "-q", "topic")

	// 冲突的 rebase 会停下来。
	cmd := exec.Command("git", "-C", dir, "rebase", "main")
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull)
	_ = cmd.Run()

	f, ok := find(run(t, dir), "rebase")
	if !ok {
		t.Fatal("stopped rebase must be reported")
	}
	if f.Severity != scan.SeverityDanger {
		t.Errorf("stopped rebase should be danger, got %v", f.Severity)
	}
	if !strings.Contains(f.Title, "topic") {
		t.Errorf("title should name the branch, got %q", f.Title)
	}
}

func TestCleanRepoHasNoAlarms(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "a.txt", "1", "first")
	commit(t, dir, "b.txt", "2", "second")

	rep := run(t, dir)
	if rep.MaxSeverity() > scan.SeverityInfo {
		t.Errorf("a clean local repo should not raise warnings, got %+v", rep.Findings)
	}
	if _, ok := find(rep, "working-copy"); ok {
		t.Error("clean tree should not report uncommitted changes")
	}
}
