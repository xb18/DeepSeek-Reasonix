---
owner: @SivanCola
backup: @esengine
status: active
reviewed: 2026-10-17
---

# Using Reasonix from DeepSeek Harness

<a href="./ACP.md">ACP</a>
&nbsp;·&nbsp;
<a href="https://github.com/deepseek-ai/deepseek-harness/tree/main/packages/subagent/subagent-acp">DSH subagent-acp</a>
&nbsp;·&nbsp;
<a href="https://reasonix.io">Reasonix website</a>

## Purpose

DeepSeek Harness (`dsh`) can delegate a task to any Agent Client Protocol agent through its `subagent-acp` provider. Reasonix speaks ACP with `reasonix acp`, so a Harness agent can hand work to Reasonix as a subagent with no code on either side, only configuration.

Verified against `dsh` 0.2.1-rc.2 and `@deepseek-ai/dsh-subagent-acp` 0.2.1-rc.2, with a real DeepSeek model in the Reasonix child.

## Preconditions

1. Reasonix MUST be installed on the machine that runs `dsh`, and its model provider MUST be set up there: run `reasonix setup` once.
2. `reasonix` MUST be reachable by the command you configure (an absolute path is the safest).
3. The provider key MUST live in Reasonix's own home (`<Reasonix home>/.env`, see [CONFIG_PATHS](./CONFIG_PATHS.md)). Reasonix does not read provider keys from environment variables, so the `env` block of the Harness row cannot supply one.
4. The provider package version MUST equal the `dsh` version (`dsh --version`). Name it exactly: the `latest` tag points at an old `0.0.1-rc.1` and `next` may resolve to an older pre-release, and `dsh` refuses both as incompatible.

## Steps

1. Install the provider into the Harness profile you use:

```sh
dsh plugin --profile <profile> add @deepseek-ai/dsh-subagent-acp@<dsh version>
```

2. Add two rows to that profile's `cordis.patch.yml` (or `$DSH_HOME/cordis.patch.yml`):

```yaml
- insert:
    - id: subagent-acp
      name: '@deepseek-ai/dsh-subagent-acp'
      config:
        providerName: reasonix
        command: reasonix
        args: ['acp']
        permission: reject
    - id: tool-subagent-reasonix
      name: '@deepseek-ai/dsh-tool-subagent'
      config:
        provider: reasonix
        toolName: reasonix
        maxDepth: provider-managed
```

3. Start `dsh`. The model now has a `reasonix` tool that takes a `prompt`, a short `description` and an optional `cwd`.

`maxDepth: provider-managed` is required: the provider cannot enforce a depth cap in the child, and `dsh` refuses to load the tool row without it.

## Ready-made package

The same two rows ship as a Harness bundle in this repository, `integrations/dsh-plugin`, so steps 1 and 2 can be one command. It is not on npm; `pnpm` installs the subdirectory through its `path:` selector (see [pnpm package sources](https://pnpm.io/package-sources)):

```sh
dsh plugin --profile <profile> add github:esengine/DeepSeek-Reasonix#path:/integrations/dsh-plugin
```

The bundle pins `@deepseek-ai/dsh-subagent-acp` and `dsh` to the same version (`0.2.1-rc.2` today) and uses `permission: reject`. Change the policy in your own `cordis.patch.yml`, never in the package.

## Finding it in Discover

DSH ships an optional "Custom plugin markets" feature with no default index; a user adds index URLs themselves. This repository publishes one:

1. In the DSH Web UI open Plugins, select More beside Experimental plugins and switch on Custom plugin markets.
2. In the Discover tab add this index URL:

```text
https://raw.githubusercontent.com/esengine/DeepSeek-Reasonix/studio/integrations/dsh-plugin/market-index.json
```

3. Select "Reasonix subagent" and install. The entry page shows the author, the website, the source repository and the exact pinned commit.

The index entry pins a commit, so it is re-pinned whenever the package changes (see the Studio release runbook).

## Configuration reference

| Key | Where | Meaning |
| --- | --- | --- |
| `command`, `args` | `subagent-acp` | What `dsh` spawns for every run. `['acp', '--model', '<name>']` selects the child model; `['acp', '--preset', 'delivery']` selects the execution setting. |
| `cwd` | `subagent-acp` | Child workspace. Omitted: the delegating session's working directory. |
| `permission` | `subagent-acp` | `reject` (default) or `allow`. See the next section. |
| `env` | `subagent-acp` | Extra child environment, layered over a credential-scrubbed copy of the parent's. Use it for `REASONIX_HOME` when the child should use a different home. |
| `maxDepth` | `tool-subagent` | MUST be `provider-managed`. |

## Permission policy

`dsh` never shows the child's permission cards to a person. The `permission` key answers every `session/request_permission` automatically:

| Policy | Effect on Reasonix |
| --- | --- |
| `reject` | `dsh` answers `cancelled`. Reasonix treats it as a refusal: the tool call fails with "blocked by permission policy", the model is told the call was declined, and the turn goes on. Writes and shell commands that need approval do not run. |
| `allow` | `dsh` selects the first `allow_once` or `allow_always` option of every request. Every approval card Reasonix raises is accepted without a person looking at it. |

Choosing `allow` MUST be a deliberate decision to let the Harness agent, and whoever steers it, run whatever Reasonix's tools can do in that workspace. It answers approval cards only; Reasonix's sandbox is configured separately in Reasonix.

1. Use `reject` for read-only research and review tasks.
2. Use `allow` only for a task you would approve wholesale, in a workspace you can discard or restore, and set `cwd` to that workspace.

## What the parent sees

| Event | Parent receives |
| --- | --- |
| Child answers | The child's final message text, as the closing message of a finished background subagent. |
| Child turn pauses as unverified (files written, no verification command ran) | The same final text, ending in a host-written `[paused] Not verified, …` line. The turn itself ends normally. |
| Child turn fails (model endpoint down, provider refusal) | A failed subagent with category `transport`. `dsh` does not put the child's error message into the parent model's view. |
| Parent cancels | The child is sent `session/cancel`, the turn ends as `cancelled`, and the process is reaped. |

## Limits

1. One fresh `reasonix acp` process per delegation; nothing from the parent conversation crosses, only the prompt text and the workspace path.
2. The child accepts no follow-up messages.
3. Only message text crosses back. The child's reasoning, tool traffic and plans stay in its own session.
4. `dsh` advertises no filesystem or terminal client capability, so Reasonix reads and writes the workspace directly.
5. The child pays for its own context; its tokens do not enter the parent conversation.
6. An install without credentials fails `session/new` with the ACP "Authentication required" error (`-32000`), which `dsh` reports as a failed start.
