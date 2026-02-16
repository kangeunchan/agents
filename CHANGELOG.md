# Changelog

All notable changes to this project are documented in this file.

## [Unreleased]

### Added

- Strict YAML loader switched to `yaml.v3` with duplicate-key detection.
- Strongly typed `plugins` and `skills` manifest schema.
- OAuth command expansion: `oauth login|status|providers`.
- Device pairing management commands: `devices list|approve|reject`.
- Observe mode for status: `status -o/--observe`.
- Colorized CLI output for status, diff, and logs.
- CI hardening: `staticcheck`, race tests, coverage gate, buildx checks.
- Repository docs: `LICENSE`, `CONTRIBUTING.md`, `SECURITY.md`.

### Changed

- Runtime config path default moved to `~/.openclawctl/openclaw.json`.
- Renderer aligned to official OpenClaw config shape:
  - `gateway.auth.token` placeholder output
  - `gateway.controlUi` defaults
  - `models.providers` mapping for custom providers
  - `auth.profiles/order` generation for Codex OAuth
  - `logging.consoleStyle` mapping
- Diff normalization now ignores redacted live secret tokens.
- Docker/Compose config mount model changed to directory mount so `config.apply` can use atomic replace.
- Docker runtime defaults updated for container-safe binding (`lan` in container + loopback host publish).

### Fixed

- `staticcheck` path issue in local `make ci` environments.
- Dockerfile ARG placement bug causing compose build failures.
- `diff` noise from redacted token values.
- `status` UI false negatives caused by auth-header EOF behavior on HTTP UI probe.
- `apply --mode auto` now succeeds via RPC on current dev runtime setup.
