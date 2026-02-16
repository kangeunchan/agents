# OpenClaw Onboarding Runbook

## 1. Prerequisites

- Docker daemon running.
- Go `1.25.x` installed.
- `OPENCLAW_GATEWAY_TOKEN` exported.
- If using Discord channel integration:
  - `DISCORD_BOT_TOKEN` exported.
  - Optional channel pinning by env:
    - `OPENCLAW_DISCORD_GROUP_POLICY=allowlist`
    - `OPENCLAW_DISCORD_GUILD_ID=<guild snowflake>`
    - `OPENCLAW_DISCORD_CHANNEL_ID=<channel snowflake>`
- Optional but recommended for clean `git commit` metadata from exec tool:
  - `OPENCLAW_GIT_USER_NAME` exported.
  - `OPENCLAW_GIT_USER_EMAIL` exported.

Example:

```bash
export OPENCLAW_GIT_USER_NAME="$(git config --global user.name)"
export OPENCLAW_GIT_USER_EMAIL="$(git config --global user.email)"
```

## 2. Install `openclawctl`

```bash
cd ~/Develop/agents/openclawctl
make install
```

If your shell cannot find it, add `~/.local/bin` to PATH.

## 3. Validate and Render

```bash
openclawctl --file ~/Develop/agents/openclaw/manifast.yaml validate
openclawctl --file ~/Develop/agents/openclaw/manifast.yaml render -o ~/.openclawctl/openclaw.json
```

## 4. Start Gateway (Dev)

```bash
openclawctl --file ~/Develop/agents/openclaw/manifast.yaml --profile dev up
openclawctl --file ~/Develop/agents/openclaw/manifast.yaml --profile dev status --json
```

Expected: docker `running/healthy`, gateway `healthy`, UI `reachable`.

## 5. Pairing / Approvals

```bash
openclawctl devices list --json
openclawctl devices approve <requestId>
```

## 6. OAuth Login (Codex)

```bash
openclawctl oauth status --provider openai-codex
openclawctl oauth login --provider openai-codex
```

## 7. Apply Config Safely

```bash
openclawctl diff
openclawctl apply --mode auto
```

`--mode auto` uses `config.apply` RPC first, then file fallback only if needed.

## 8. UI Access

Open:

- `http://127.0.0.1:18789/`
- `http://127.0.0.1:18789/#token=<OPENCLAW_GATEWAY_TOKEN>`

## 9. Discord Command Authorization

If Discord replies `You are not authorized to use this command.`:

1. Ensure command access groups are disabled at root `commands.useAccessGroups: false` in `manifast.yaml`.
2. If needed, explicitly set operator allowlist:

```yaml
commands:
  allowFrom:
    discord:
      - "${OPENCLAW_DISCORD_OPERATOR_ID}"
```

3. Re-apply:

```bash
openclawctl --file ~/Develop/agents/openclaw/manifast.yaml --profile dev apply --mode auto
```

## 10. Rollback

1. Restore previous manifest.
2. Re-render config.
3. Re-apply.

```bash
openclawctl render -o ~/.openclawctl/openclaw.json
openclawctl apply --mode auto
```

## 11. Shutdown

```bash
openclawctl --file ~/Develop/agents/openclaw/manifast.yaml --profile dev down
```
