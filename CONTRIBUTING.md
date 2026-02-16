# Contributing

## Scope

- `openclawctl/`: Go CLI and tests.
- `openclaw/`: Docker runtime packaging and local compose profile.

## Local Setup

1. Install Go `1.25.x` and Docker with Buildx.
2. Export required env:

```bash
export OPENCLAW_GATEWAY_TOKEN='dev-local-token'
```

3. Install CLI binary:

```bash
cd openclawctl
make install
```

By default `make install` writes the binary to `~/.local/bin/openclawctl`.

## Development Workflow

1. Validate manifest:

```bash
openclawctl --file ../openclaw/manifast.yaml validate
```

2. Render config:

```bash
openclawctl --file ../openclaw/manifast.yaml render -o ~/.openclawctl/openclaw.json
```

3. Start runtime and verify:

```bash
openclawctl --file ../openclaw/manifast.yaml --profile dev up
openclawctl --file ../openclaw/manifast.yaml --profile dev status --json
openclawctl --file ../openclaw/manifast.yaml --profile dev diff
openclawctl --file ../openclaw/manifast.yaml --profile dev apply --mode auto
```

4. Stop runtime when done:

```bash
openclawctl --file ../openclaw/manifast.yaml --profile dev down
```

## Quality Gates

Run from `openclawctl/` before opening a PR:

```bash
make ci
make buildx-builder-check
make buildx-check
```

## Commit and PR Guidelines

- Keep changes focused and atomic.
- Include test updates for behavior changes.
- For config/schema changes, update:
  - `openclaw/manifast.yaml`
  - `openclawctl/testdata/golden/*`
  - related docs in `README.md`
- Prefer explicit migration notes in `CHANGELOG.md`.
