package scan

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"gitrescue/internal/gitcmd"
	"gitrescue/internal/i18n"
)

// ---------- 公共小工具 ----------

// fileAt 读取 .git 目录下的文本文件，读不到就返回空串。
func fileAt(repo *gitcmd.Repo, rel string) string {
	b, err := os.ReadFile(repo.Path(strings.Split(rel, "/")...))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// logLine 把某个提交压成一行：短 sha、相对时间、标题。
func logLine(repo *gitcmd.Repo, rev string) string {
	if rev == "" {
		return ""
	}
	out, err := repo.Run("log", "-1", "--format=%h%x09%ct%x09%s", rev)
	if err != nil {
		return rev
	}
	parts := strings.SplitN(strings.TrimSpace(out), "\t", 3)
	if len(parts) < 3 {
		return strings.TrimSpace(out)
	}
	ts, _ := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
	return fmt.Sprintf("%s  %s  %s", parts[0], i18n.Ago(ts), strings.TrimSpace(parts[2]))
}

// logLines 用一条命令批量取提交，最多 n 条。
func logLines(repo *gitcmd.Repo, n int, revRange ...string) []string {
	args := []string{"log", fmt.Sprintf("--max-count=%d", n), "--format=%h  %s"}
	args = append(args, revRange...)
	out, err := repo.Run(args...)
	if err != nil {
		return nil
	}
	return splitLines(out)
}

func splitLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

// ---------- 空仓库 ----------

func emptyRepo(repo *gitcmd.Repo) []Finding {
	return []Finding{{
		Kind:     "empty-repo",
		Severity: SeverityInfo,
		Title:    i18n.T("这个仓库还没有任何提交", "this repository has no commits yet"),
		Detail:   splitLines(i18n.T("HEAD 还指向一个不存在的分支，所以没什么可丢的。", "HEAD points at an unborn branch, so there is nothing to lose yet.")),
		Commands: []Command{
			{Label: i18n.T("开始第一次提交", "make the first commit"), Line: "git commit --allow-empty -m \"init\""},
		},
	}}
}

// ---------- 正在进行的操作 ----------

func inProgress(repo *gitcmd.Repo) []Finding {
	var out []Finding

	switch {
	case repo.Exists("rebase-merge"):
		out = append(out, rebaseFinding(repo, "rebase-merge"))
	case repo.Exists("rebase-apply"):
		out = append(out, rebaseFinding(repo, "rebase-apply"))
	}
	if repo.Exists("MERGE_HEAD") {
		out = append(out, mergeFinding(repo))
	}
	if repo.Exists("CHERRY_PICK_HEAD") {
		sha := fileAt(repo, "CHERRY_PICK_HEAD")
		out = append(out, Finding{
			Kind:     "cherry-pick",
			Severity: SeverityDanger,
			Title:    i18n.T("cherry-pick 停在半路，冲突还没解决", "cherry-pick is paused with unresolved conflicts"),
			Detail:   compact(logLine(repo, sha)),
			Commands: []Command{
				{Label: i18n.T("继续", "continue"), Line: "git cherry-pick --continue"},
				{Label: i18n.T("放弃这个提交", "drop this commit"), Line: "git cherry-pick --abort"},
			},
			Learn: i18n.T(
				"冲突解决完要 git add 打过标记，--continue 才知道哪些文件处理好了。--abort 会完整回到 cherry-pick 之前的状态，不会留下半个提交。",
				"Run git add on the resolved files first, otherwise --continue does not know what is done. --abort restores the exact state from before the cherry-pick.",
			),
		})
	}
	if repo.Exists("REVERT_HEAD") {
		sha := fileAt(repo, "REVERT_HEAD")
		out = append(out, Finding{
			Kind:     "revert",
			Severity: SeverityDanger,
			Title:    i18n.T("revert 停在半路，冲突还没解决", "revert is paused with unresolved conflicts"),
			Detail:   compact(logLine(repo, sha)),
			Commands: []Command{
				{Label: i18n.T("继续", "continue"), Line: "git revert --continue"},
				{Label: i18n.T("放弃", "abort"), Line: "git revert --abort"},
			},
		})
	}
	if repo.Exists("BISECT_LOG") {
		start := fileAt(repo, "BISECT_START")
		f := Finding{
			Kind:     "bisect",
			Severity: SeverityWarn,
			Title:    i18n.T("二分查找（bisect）还没结束", "a bisect is still in progress"),
			Commands: []Command{
				{Label: i18n.T("结束并回到原分支", "end and return to the original branch"), Line: "git bisect reset"},
			},
			Learn: i18n.T(
				"bisect 期间 HEAD 是游离的，仓库看着像「少了提交」。不 reset 就继续开发，很容易把新提交落在游离 HEAD 上。",
				"During a bisect HEAD is detached, which is why history looks odd. Keep working without resetting and new commits land on a detached HEAD.",
			),
		}
		if start != "" && start != "HEAD" {
			f.Detail = []string{fmt.Sprintf(i18n.T("原来的分支：%s", "original branch: %s"), start)}
		}
		out = append(out, f)
	}
	return out
}

func rebaseFinding(repo *gitcmd.Repo, dir string) Finding {
	f := Finding{Kind: "rebase", Severity: SeverityDanger}

	branch := strings.TrimPrefix(fileAt(repo, dir+"/head-name"), "refs/heads/")
	if branch != "" {
		f.Title = fmt.Sprintf(i18n.T("rebase 还没走完（分支 %s）", "rebase in progress (branch %s)"), branch)
	} else {
		f.Title = i18n.T("rebase 还没走完", "rebase in progress")
	}

	// 交互式 rebase 用 msgnum/end，git am 用 next/last。
	cur, total := atoi(fileAt(repo, dir+"/msgnum")), atoi(fileAt(repo, dir+"/end"))
	if cur == 0 {
		cur, total = atoi(fileAt(repo, dir+"/next")), atoi(fileAt(repo, dir+"/last"))
	}
	if cur > 0 && total > 0 {
		f.Detail = append(f.Detail, fmt.Sprintf(i18n.T("进度：第 %d 步 / 共 %d 步", "step %d of %d"), cur, total))
	}
	if stopped := fileAt(repo, dir+"/stopped-sha"); stopped != "" {
		f.Detail = append(f.Detail, compact(i18n.T("停在：", "stopped at: ")+logLine(repo, stopped))...)
	}

	f.Commands = []Command{
		{Label: i18n.T("继续", "continue"), Line: "git rebase --continue"},
		{Label: i18n.T("跳过这个提交", "skip this commit"), Line: "git rebase --skip"},
		{Label: i18n.T("放弃，回到 rebase 之前", "abort, restore the pre-rebase state"), Line: "git rebase --abort"},
	}
	f.Learn = i18n.T(
		"rebase 是「拆开重放」：原来的提交还在，只是暂时没有任何分支指向它们，所以 --abort 总能完整退回去。真想放弃重放的结果，--abort 比手动 reset 安全得多。",
		"Rebase replays commits one by one. The originals still exist but no branch points at them, which is why --abort can always restore everything. Prefer --abort over a manual reset.",
	)
	return f
}

func mergeFinding(repo *gitcmd.Repo) Finding {
	sha := fileAt(repo, "MERGE_HEAD")
	return Finding{
		Kind:     "merge",
		Severity: SeverityDanger,
		Title:    i18n.T("merge 还没完成，冲突留在工作区里", "merge is unfinished, conflicts are still in the working tree"),
		Detail:   compact(logLine(repo, sha)),
		Commands: []Command{
			{Label: i18n.T("解决完冲突后继续", "continue after resolving"), Line: "git merge --continue"},
			{Label: i18n.T("放弃整个 merge", "abort the merge"), Line: "git merge --abort"},
		},
		Learn: i18n.T(
			"merge 冲突时 git 会把两边的内容都写进文件，用 <<<<<<< 标记。删掉标记、留下想要的内容，git add 之后再 --continue。想反悔就 --abort，工作区回到 merge 前。",
			"On conflict git writes both sides into the file with <<<<<<< markers. Remove the markers, keep what you want, git add, then --continue. --abort puts the tree back.",
		),
	}
}

func compact(parts ...string) []string {
	var out []string
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ---------- 游离 HEAD ----------

func detachedHead(repo *gitcmd.Repo, detached bool) []Finding {
	if !detached {
		return nil
	}
	out, err := repo.Run("rev-list", "--count", "HEAD", "--not", "--branches", "--tags", "--remotes")
	if err != nil {
		return nil
	}
	unique := atoi(out)
	if unique == 0 {
		// 只是看了一眼某个提交，没有新东西会丢，不打扰。
		return nil
	}

	short := strings.TrimSpace(mustRun(repo, "rev-parse", "--short", "HEAD"))
	return []Finding{{
		Kind:     "detached",
		Severity: SeverityDanger,
		Title:    fmt.Sprintf(i18n.T("HEAD 处于游离状态，有 %d 个提交没挂在任何分支上", "HEAD is detached with %d commit(s) on no branch"), unique),
		Detail: append(
			[]string{i18n.T("一旦切到别的分支，这些提交就只能靠 reflog 找回。", "Switching to another branch now leaves them reachable only via the reflog.")},
			logLines(repo, 3, "HEAD", "--not", "--branches", "--tags", "--remotes")...,
		),
		Commands: []Command{
			{Label: i18n.T("存成一个分支保留下来", "save them on a branch"), Line: "git switch -c rescue/detached-" + short},
			{Label: i18n.T("什么都不想要，回到原分支", "discard and go back"), Line: "git switch -"},
		},
		Learn: i18n.T(
			"游离 HEAD 就是 HEAD 直接指向某个提交，而不是分支。在上面提交依然有效、依然能找回，但没有任何分支名指向它们，切走之后就只能靠 reflog。",
			"A detached HEAD points straight at a commit instead of a branch. Commits made there are real and recoverable, but no branch name leads to them.",
		),
	}}
}

func mustRun(repo *gitcmd.Repo, args ...string) string {
	out, err := repo.Run(args...)
	if err != nil {
		return ""
	}
	return out
}

// ---------- 未推送的提交 ----------

func unpushed(repo *gitcmd.Repo, branch string) []Finding {
	if branch == "" {
		return nil
	}

	if upstream, ok := repo.Try("rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}"); ok {
		up := strings.TrimSpace(upstream)
		n := atoi(mustRun(repo, "rev-list", "--count", up+"..HEAD"))
		if n == 0 {
			return nil
		}
		return []Finding{{
			Kind:     "unpushed",
			Severity: SeverityWarn,
			Title:    fmt.Sprintf(i18n.T("%d 个提交还没推送到 %s", "%d commit(s) not pushed to %s"), n, up),
			Detail:   logLines(repo, 3, up+"..HEAD"),
			Commands: []Command{
				{Label: i18n.T("推送", "push"), Line: "git push"},
			},
			Learn: i18n.T(
				"这些提交只存在于这台机器上。硬盘坏了、或者换个环境，就都没有了。",
				"These commits exist only on this machine. A dead disk or a new machine and they are gone.",
			),
		}}
	}

	// 没有 upstream：数一数哪些提交不在任何远程分支上。
	n := atoi(mustRun(repo, "rev-list", "--count", "HEAD", "--not", "--remotes"))
	if n == 0 {
		return nil
	}

	finding := Finding{
		Kind:     "unpushed",
		Severity: SeverityWarn,
		Title:    fmt.Sprintf(i18n.T("分支 %s 没有对应的远程分支，%d 个提交只在本地", "branch %s has no remote counterpart, %d commit(s) exist only locally"), branch, n),
		Detail:   logLines(repo, 3, "HEAD", "--not", "--remotes"),
		Commands: []Command{
			{Label: i18n.T("建立远程分支并推送", "create the remote branch and push"), Line: "git push -u origin " + branch},
		},
		Learn: i18n.T(
			"git push 默认不会自动建远程分支，第一次需要 -u 把本地分支和远程分支关联起来。",
			"A plain git push does not create a remote branch. The first push needs -u to link them.",
		),
	}

	// 连接口都没配过的仓库，提醒"没推送"意义不大，降级成提示。
	if !hasRemote(repo) {
		finding.Severity = SeverityInfo
		// 中英文里分支名和数量的顺序相反，不能共用一条格式串。
		if i18n.Get() == i18n.EN {
			finding.Title = fmt.Sprintf("%d commit(s) on branch %s exist only locally", n, branch)
		} else {
			finding.Title = fmt.Sprintf("分支 %s 上的 %d 个提交只在本地", branch, n)
		}
		finding.Commands = []Command{
			{Label: i18n.T("先接上远程仓库", "attach a remote first"), Line: "git remote add origin <仓库地址>"},
		}
		finding.Learn = i18n.T(
			"这个仓库还没有配置任何远程地址，所以提交没有任何地方可去。接上远程之后就能 push 了。",
			"This repository has no remote configured yet, so there is nowhere to push. Add one and these commits can leave the machine.",
		)
	}
	return []Finding{finding}
}

func hasRemote(repo *gitcmd.Repo) bool {
	out, err := repo.Run("remote")
	return err == nil && strings.TrimSpace(out) != ""
}

// ---------- stash ----------

func stashed(repo *gitcmd.Repo) []Finding {
	out, err := repo.Run("stash", "list", "--format=%gd%x09%ct%x09%s")
	if err != nil {
		return nil
	}
	lines := splitLines(out)
	if len(lines) == 0 {
		return nil
	}

	var detail []string
	var newest int64
	for i, line := range lines {
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 {
			continue
		}
		ts, _ := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
		if ts > newest {
			newest = ts
		}
		if i < 4 {
			detail = append(detail, fmt.Sprintf("%s  %s  %s", parts[0], i18n.Ago(ts), parts[2]))
		}
	}

	sev := SeverityInfo
	// 放了一个月以上的 stash 基本就是忘了。
	if newest > 0 && time.Since(time.Unix(newest, 0)) > 30*24*time.Hour {
		sev = SeverityWarn
	}

	return []Finding{{
		Kind:     "stash",
		Severity: sev,
		Title:    fmt.Sprintf(i18n.T("%d 个 stash 还挂在仓库里", "%d stash entr(ies) still on this repo"), len(lines)),
		Detail:   detail,
		Commands: []Command{
			{Label: i18n.T("看内容", "inspect"), Line: "git stash show -p stash@{0}"},
			{Label: i18n.T("取出来并删掉这条 stash", "apply and drop"), Line: "git stash pop"},
		},
		Learn: i18n.T(
			"stash 不会过期，也不会被 push。时间一长就成了「不知道是什么」的黑盒，最好的处理方式是要么 pop 要么 drop。",
			"Stashes never expire and are never pushed. Over time they become mystery boxes; either pop them or drop them.",
		),
	}}
}

// ---------- 悬空 / 丢失的提交 ----------

type lostCommit struct {
	sha     string
	when    int64
	subject string
}

func lostCommits(repo *gitcmd.Repo, deep bool) ([]Finding, string) {
	timeout := 20 * time.Second
	if deep {
		timeout = 180 * time.Second
	}

	// --no-reflogs 是关键：让只被 reflog 记住的提交也算作不可达，
	// reset --hard 之后丢掉的提交正好属于这一类。
	stdout, _, err := repo.Output(timeout, "fsck", "--unreachable", "--no-reflogs", "--no-progress")
	if err != nil && strings.TrimSpace(stdout) == "" {
		if deep {
			return nil, i18n.T(
				"git fsck 没有跑成功，跳到后面的检查了",
				"git fsck failed; skipped the dangling-commit check",
			)
		}
		return nil, i18n.T(
			"git fsck 超时或失败，跳过了悬空提交扫描；仓库很大时可以加 --deep 再试",
			"git fsck timed out; skipped the dangling-commit scan. Retry with --deep on large repositories",
		)
	}

	seen := map[string]bool{}
	var shas []string
	for _, line := range splitLines(stdout) {
		line = strings.TrimPrefix(line, "error: ")
		for _, prefix := range []string{"unreachable commit ", "dangling commit "} {
			if rest, ok := strings.CutPrefix(line, prefix); ok {
				sha := strings.TrimSpace(rest)
				if sha != "" && !seen[sha] {
					seen[sha] = true
					shas = append(shas, sha)
				}
			}
		}
	}
	if len(shas) == 0 {
		return nil, ""
	}

	// 一次调用批量取信息，最多 100 个，避免命令行过长。
	if len(shas) > 100 {
		shas = shas[:100]
	}
	args := []string{"log", "--no-walk=unsorted", "--format=%H%x09%ct%x09%s"}
	args = append(args, shas...)
	out, err := repo.Run(args...)
	if err != nil {
		return nil, i18n.T("读不到悬空提交的详细信息", "could not read details for the dangling commits")
	}

	var commits []lostCommit
	for _, line := range splitLines(out) {
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 || strings.TrimSpace(parts[2]) == "" {
			continue
		}
		ts, _ := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
		commits = append(commits, lostCommit{
			sha:     strings.TrimSpace(parts[0]),
			when:    ts,
			subject: strings.TrimSpace(parts[2]),
		})
	}
	if len(commits) == 0 {
		return nil, ""
	}

	sort.Slice(commits, func(i, j int) bool { return commits[i].when > commits[j].when })

	sev := SeverityWarn
	if time.Since(time.Unix(commits[0].when, 0)) > 30*24*time.Hour {
		sev = SeverityInfo
	}

	detail := []string{i18n.T(
		"它们不在任何分支、tag 或远程分支上，只靠 reflog 记着。",
		"Nothing points at them: no branch, tag or remote. Only the reflog remembers.",
	)}
	for i, c := range commits {
		if i >= 4 {
			break
		}
		detail = append(detail, fmt.Sprintf("%s  %s  %s", short(c.sha), i18n.Ago(c.when), c.subject))
	}

	first := commits[0]
	return []Finding{{
		Kind:     "lost-commits",
		Severity: sev,
		Title:    fmt.Sprintf(i18n.T("%d 个提交不在任何分支上", "%d commit(s) live on no branch"), len(commits)),
		Detail:   detail,
		Commands: []Command{
			{Label: i18n.T("先看看改了什么", "inspect the newest one"), Line: "git show " + short(first.sha)},
			{Label: i18n.T("用分支把它接住", "keep it on a branch"), Line: "git branch rescue/" + short(first.sha) + " " + short(first.sha)},
		},
		Learn: i18n.T(
			"提交对象本身不会因为 reset、rebase、amend 而消失，只有 gc 之后才可能被清理。在那之前，reflog 和 fsck 都能把它们找回来。所以「我刚 reset --hard 了」几乎从来不是灾难。",
			"Commit objects survive reset, rebase and amend; only gc can remove them. Until then the reflog and fsck can find them, so a hard reset is almost never fatal.",
		),
	}}, ""
}

func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// ---------- 工作区 ----------

func workingCopy(repo *gitcmd.Repo) []Finding {
	out, err := repo.Run("status", "--porcelain=v1")
	if err != nil {
		return nil
	}
	lines := splitLines(out)
	if len(lines) == 0 {
		return nil
	}

	var tracked, untracked int
	for _, line := range lines {
		if strings.HasPrefix(line, "??") {
			untracked++
		} else {
			tracked++
		}
	}

	return []Finding{{
		Kind:     "working-copy",
		Severity: SeverityInfo,
		Title:    fmt.Sprintf(i18n.T("工作区有 %d 处未提交的改动", "%d uncommitted change(s) in the working tree"), len(lines)),
		Detail: []string{fmt.Sprintf(
			i18n.T("已跟踪文件 %d 个，未跟踪文件 %d 个", "%d tracked file(s), %d untracked"),
			tracked, untracked,
		)},
		Commands: []Command{
			{Label: i18n.T("查看明细", "see the details"), Line: "git status --short"},
		},
	}}
}
