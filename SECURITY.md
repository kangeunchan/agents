# Security Policy

## Supported Scope

Security fixes apply to:

- `openclawctl` CLI commands and config handling.
- `openclaw` Docker packaging and runtime defaults.

## Reporting a Vulnerability

Please do not open a public issue for active security vulnerabilities.

Send details to the maintainer with:

- Affected component and version/commit.
- Reproduction steps and expected impact.
- Any proof-of-concept or logs (redact secrets).

## Secret Handling Rules

- Never commit plain secrets to YAML.
- Use `${ENV_VAR}` placeholders only.
- Runtime config is generated into `~/.openclawctl/openclaw.json`.
- Rollback backup files are temporary and removed after successful apply.

## Hardening Defaults

- Strict YAML decode (unknown keys fail).
- Duplicate YAML key detection.
- Env placeholder resolution with unresolved variable failure.
- Gateway token required for runtime startup.
