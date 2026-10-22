# gitrescue

**You broke git. Let's find out how bad it is.**

`gitrescue` is a read-only diagnosis of your repository. Run it when something
feels wrong — after a `git reset --hard`, a rebase that stopped three commits
ago, a HEAD that detached itself — and it tells you the actual state you are in
and the exact command to get out.

No network. No AI. No writes. It only reads.

[中文说明](README.zh-CN.md)

```
gitrescue  detached @ 13ef966
~/code/my-project
────────────────────────────────────────────────────────────────────

  ✗  HEAD is detached with 1 commit(s) on no branch
     Switching to another branch now leaves them reachable only via the reflog.
     13ef966  wip: experimental change

     save them on a branch   git switch -c rescue/detached-13ef966
     discard and go back     git switch -

  !  1 commit(s) live on no branch
     Nothing points at them: no branch, tag or remote. Only the reflog remembers.
     95bfbae  3m ago  fix: login timeout was wrong

     inspect the newest one   git show 95bfbae
     keep it on a branch      git branch rescue/95bfbae 95bfbae

  ·  1 stash entry still on this repo
     stash@{0}  3m ago  WIP on main: f603faf feat: add profile entry

     inspect          git stash show -p stash@{0}
     apply and drop   git stash pop

────────────────────────────────────────────────────────────────────
1 need(s) action · 2 worth a look · read-only, nothing was modified
```

## What it checks

| Check | What it catches |
| --- | --- |
| Unfinished operations | paused rebase / merge / cherry-pick / revert / bisect, with progress and the continue-skip-abort set |
| Detached HEAD | commits sitting on a HEAD that no branch points at |
| Lost commits | commits reachable only through the reflog, e.g. after `reset --hard` |
| Unpushed work | commits that exist only on this machine |
| Stashes | forgotten stashes, and ones old enough to be mystery boxes |
| Working tree | how much is uncommitted |

## Install

```sh
go install github.com/yourname/gitrescue@latest
```

Or from source:

```sh
git clone https://github.com/yourname/gitrescue
cd gitrescue
go build -o gitrescue .
```

Requires Go 1.24+ and `git` on `PATH`.

## Usage

```sh
gitrescue                 # scan the current repo
gitrescue -C ../other     # scan another directory
gitrescue --explain       # add the "why does this happen" for each finding
gitrescue --json          # machine-readable, for scripts and CI
gitrescue --strict        # exit 1 when something needs action
gitrescue --deep          # longer timeout for git fsck on big repos
gitrescue --lang en       # English output (default is Chinese)
```

Exit codes: `0` normal, `1` with `--strict` and something needs action, `2` not a
repository or usage error.

## Why read-only

Every recovery in git is a write, and a tool that guesses wrong can make things
worse. `gitrescue` never runs a mutating command: it uses `rev-parse`, `log`,
`rev-list`, `status`, `stash list` and `fsck`, plus the state files under
`.git/`. It also sets `GIT_OPTIONAL_LOCKS=0` and `GIT_TERMINAL_PROMPT=0`, so it
will not take a lock or block on a credential prompt.

You get commands you can read, understand and run yourself.

## Design notes

- **Zero dependencies.** Standard library only — the TUI-ish output is hand
  written ANSI, and Windows virtual-terminal mode is enabled via `kernel32`.
- **Bilingual.** Messages are declared as `(中文, English)` pairs at the call
  site (`i18n.T`), so there is no key table to keep in sync.
- **Testable.** `internal/scan` is tested against real temporary repositories
  built with `git` in the test.

## Contributing

Issues and PRs are welcome. A useful new check needs three things: a state file
or command that reveals it reliably, a recovery command that is safe to suggest,
and a test that builds the broken state from scratch.

```sh
go test ./...
go vet ./...
```

## License

MIT
