---
owner: @esengine
backup: @SivanCola
status: active
reviewed: 2026-10-17
---

# dsh-plugin-reasonix

English | [中文](#中文)

[Website](https://reasonix.io) · [Documentation](../../docs/DSH.md) · [Source](../../README.md)

Use [Reasonix](../../README.md) as a subagent in [DeepSeek Harness](https://github.com/deepseek-ai/deepseek-harness) (`dsh`). The package is configuration only: one patch layer that points Harness's `subagent-acp` provider at `reasonix acp` and exposes it to the model as a `reasonix` tool. It contains no code.

## Before you install

1. Install Reasonix and run `reasonix setup` once on the same machine. Reasonix reads its provider key from its own home, not from environment variables, so Harness cannot pass one.
2. Make sure `reasonix` is on the `PATH` of the process that runs `dsh`.

## Install

```sh
dsh plugin --profile <profile> add github:esengine/DeepSeek-Reasonix#path:/integrations/dsh-plugin
```

The package is not on npm; `pnpm` installs the `integrations/dsh-plugin` subdirectory of this repository (the `path:` selector, see [pnpm package sources](https://pnpm.io/package-sources)).

## What it changes

| Row | Effect |
| --- | --- |
| `subagent-acp-reasonix` | Spawns `reasonix acp` for each delegation; `permission: reject`. |
| `tool-subagent-reasonix` | Adds the `reasonix` tool (`prompt`, `description`, optional `cwd`); `maxDepth: provider-managed`, which the provider requires. |

## Permissions

- `permission: reject` is the default. Every approval card Reasonix raises is declined, so the child can read and research but cannot write files or run commands that need approval.
- `permission: allow` makes Harness accept every card automatically. The delegated task can then do anything Reasonix's tools can do in that workspace.
- Change the policy only in your own `cordis.patch.yml`, for a task and workspace you would approve wholesale.

Details, what the parent receives, and limits: [Using Reasonix from DeepSeek Harness](../../docs/DSH.md).

## Versions

The provider version must equal your `dsh` version. This release pins `dsh` 0.2.1-rc.2.

---

## 中文

[官网](https://reasonix.io) · [文档](../../docs/DSH.md) · [源码](../../README.md)

把 [Reasonix](https://github.com/esengine/DeepSeek-Reasonix) 作为子智能体接入 [DeepSeek Harness](https://github.com/deepseek-ai/deepseek-harness)（`dsh`）。本包只有配置，没有代码：一层补丁，让 Harness 的 `subagent-acp` 提供方去启动 `reasonix acp`，并以 `reasonix` 工具的形式暴露给模型。

**安装前**

1. 在同一台机器上安装 Reasonix 并运行一次 `reasonix setup`。Reasonix 只从自己的主目录读取模型密钥，不读环境变量，所以 Harness 无法替它传密钥。
2. 保证运行 `dsh` 的进程的 `PATH` 里有 `reasonix`。

**安装**

```sh
dsh plugin --profile <profile> add github:esengine/DeepSeek-Reasonix#path:/integrations/dsh-plugin
```

包不在 npm 上：`pnpm` 通过 `path:` 选择器安装本仓库的 `integrations/dsh-plugin` 子目录（见 [pnpm package sources](https://pnpm.io/package-sources)）。

**权限**

- 默认 `permission: reject`：Reasonix 弹出的每个审批卡都会被拒绝，子智能体只能读和调研，不能写文件或执行需要批准的命令。
- `permission: allow` 会让 Harness 自动同意所有审批卡，被委派的任务可以在该工作区里做 Reasonix 工具能做的任何事。
- 只在你自己的 `cordis.patch.yml` 里，为你愿意整体放行的任务和工作区才改成 `allow`。

详细说明见 [Using Reasonix from DeepSeek Harness](../../docs/DSH.md)。

**版本**：提供方版本必须与你的 `dsh` 版本一致，本版本固定为 `dsh` 0.2.1-rc.2。
