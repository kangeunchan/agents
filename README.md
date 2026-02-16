# OpenClaw YAML-First Controller

`openclawctl` is a Go CLI that manages OpenClaw using a strict YAML manifest (`openclaw/manifast.yaml`).

## Repository Layout

- `openclawctl/`: CLI source code, tests, CI workflow.
- `openclaw/`: OpenClaw runtime image and dev compose files.
- `openclaw/manifast.yaml`: primary declarative config source.

## Prerequisites

- Go `1.25.x`
- Docker with Buildx
- OpenClaw gateway token (`OPENCLAW_GATEWAY_TOKEN`)

## Quick Start

```bash
export OPENCLAW_GATEWAY_TOKEN='your-token'

# Install CLI
cd openclawctl
make install

# Ensure ~/.local/bin is in PATH (one-time)
# zsh/bash
export PATH="$HOME/.local/bin:$PATH"
# fish
set -Ux fish_user_paths $HOME/.local/bin $fish_user_paths

# Validate and render
openclawctl --file ../openclaw/manifast.yaml validate
openclawctl --file ../openclaw/manifast.yaml render -o ~/.openclawctl/openclaw.json

# Start dev runtime
openclawctl --file ../openclaw/manifast.yaml --profile dev up

# Health
openclawctl --file ../openclaw/manifast.yaml status --json
```

## OAuth Flow (Codex)

```bash
# Check provider plugin readiness
openclawctl oauth providers --provider openai-codex

# Check model authorization status
openclawctl oauth status --provider openai-codex

# Interactive OAuth login (TTY required)
openclawctl oauth login --provider openai-codex
```

## Pairing Requests

```bash
openclawctl devices list --json
openclawctl devices approve <requestId>
openclawctl devices reject <requestId>
```

## Runbook

See `openclaw/scripts/onboarding-runbook.md` for end-to-end onboarding, OAuth login, apply, rollback, and shutdown procedures.

## CI / Quality Gates

From `openclawctl/`:

```bash
make ci
make buildx-builder-check
make buildx-check
```

`make ci` includes:

- `gofmt` check
- `go vet`
- `staticcheck`
- `go test`
- `go test -race`
- coverage gate (default: `>=25%`)

## Security Defaults

- Strict YAML decode with unknown-key rejection.
- `${ENV_VAR}` placeholder expansion with unresolved-variable failure.
- Container binds `lan`, while host publishing remains loopback-only (`127.0.0.1:18789`).
- Config apply rollback backup is short-lived and auto-cleaned after apply/rollback.

## License

MIT. See `LICENSE`.
