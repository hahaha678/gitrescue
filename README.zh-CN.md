# gitrescue

**把 git 搞坏了？先别慌，看看现在到底怎么了。**

`gitrescue` 是一个**只读**的仓库诊断工具。`git reset --hard` 之后、rebase 卡在半路、
HEAD 莫名其妙游离了——跑一下，它会告诉你当前真实的仓库状态，以及一条可以直接
复制的恢复命令。

不联网。不用 AI。不写任何东西。只看。

[English](README.md)

```
gitrescue  detached @ 13ef966
~/code/my-project
────────────────────────────────────────────────────────────────────

  ✗  HEAD 处于游离状态，有 1 个提交没挂在任何分支上
     一旦切到别的分支，这些提交就只能靠 reflog 找回。
     13ef966  wip: 试验性的改动

     存成一个分支保留下来       git switch -c rescue/detached-13ef966
     什么都不想要，回到原分支   git switch -

  !  1 个提交不在任何分支上
     它们不在任何分支、tag 或远程分支上，只靠 reflog 记着。
     95bfbae  3 分钟前  fix: 登录超时时间设错了

     先看看改了什么   git show 95bfbae
     用分支把它接住   git branch rescue/95bfbae 95bfbae

  ·  1 个 stash 还挂在仓库里
     stash@{0}  3 分钟前  WIP on main: f603faf feat: 加个人中心入口

     看内容                   git stash show -p stash@{0}
     取出来并删掉这条 stash   git stash pop

────────────────────────────────────────────────────────────────────
1 处需要立刻处理 · 2 处值得看一眼 · 只读检查，没有改动任何东西
```

## 检查什么

| 检查项 | 能发现什么 |
| --- | --- |
| 未完成的操作 | 卡住的 rebase / merge / cherry-pick / revert / bisect，附带进度和继续/跳过/放弃三件套 |
| 游离 HEAD | 提交挂在没有任何分支指向的 HEAD 上 |
| 丢失的提交 | 只被 reflog 记住的提交，比如 `reset --hard` 之后 |
| 没推送的工作 | 只存在于这台机器上的提交 |
| stash | 被忘掉的 stash，以及放了太久变成黑盒的那些 |
| 工作区 | 还有多少没提交 |

## 安装

每个 [release](https://github.com/hahaha678/gitrescue/releases) 都附带 Linux、macOS、
Windows（amd64 / arm64）的预编译二进制，不需要装 Go。

```sh
go install github.com/hahaha678/gitrescue@latest
```

或者从源码构建：

```sh
git clone https://github.com/hahaha678/gitrescue
cd gitrescue
go build -o gitrescue .
```

需要 Go 1.24+，以及 `PATH` 里的 `git`。

## 用法

```sh
gitrescue                 # 检查当前仓库
gitrescue -C ../other     # 检查别的目录
gitrescue --explain       # 每条发现后面补上「为什么会这样」
gitrescue --json          # 机器可读，给脚本和 CI 用
gitrescue --strict        # 发现问题时退出码为 1
gitrescue --deep          # 大仓库上给 git fsck 更长的超时
gitrescue --lang en       # 英文输出（默认中文）
```

退出码：`0` 正常，`1` 在 `--strict` 下表示有需要处理的问题，`2` 不是仓库或参数错误。

## 为什么坚持只读

git 的每一次恢复都是写操作，而一个猜错的工具只会让事情更糟。`gitrescue` 从不执行
任何会修改仓库的命令：它只用 `rev-parse`、`log`、`rev-list`、`status`、
`stash list`、`fsck`，以及读 `.git/` 下的状态文件。同时设置
`GIT_OPTIONAL_LOCKS=0` 和 `GIT_TERMINAL_PROMPT=0`，既不会抢锁，也不会卡在认证提示上。

你拿到的每条命令都可以先读懂，再自己决定要不要执行。

## 设计说明

- **零依赖**：只用标准库。终端样式是手写的 ANSI 序列，Windows 的虚拟终端模式通过
  `kernel32` 打开，不引第三方包。
- **双语**：文案在调用处直接写成 `(中文, English)` 成对的形式（`i18n.T`），
  不需要维护 key 表。
- **可测试**：`internal/scan` 的测试会在临时目录里用 `git` 真实构造出各种"事故现场"。

## 参与贡献

欢迎 issue 和 PR。一个新的检查项需要三样东西：一个能可靠暴露该状态的命令或状态文件、
一条足够安全的恢复命令、一个能从零构造出这个状态的测试。

```sh
go test ./...
go vet ./...
```

## 发布流程

打 tag 之后由 GoReleaser 自动构建：

```sh
git tag -a v0.1.0 -m "v0.1.0"
git push origin v0.1.0
```

`release` workflow 会先跑测试，再交叉编译 linux / darwin / windows 的 amd64 和
arm64，打成 `tar.gz`（Windows 是 `.zip`），生成 `checksums.txt`，最后发布 GitHub
release。想在本地完整跑一遍但不发布：

```sh
goreleaser release --snapshot --clean --skip=publish
```

版本号通过 `-X main.version={{ .Version }}` 注入，所以 `gitrescue --version`
和 tag 一致。

## 许可证

MIT
