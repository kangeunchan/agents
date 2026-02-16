#!/bin/sh
set -eu

OPENCLAW_GIT_USER_NAME="${OPENCLAW_GIT_USER_NAME:-OpenClaw}"
OPENCLAW_GIT_USER_EMAIL="${OPENCLAW_GIT_USER_EMAIL:-openclaw@local}"

if [ -z "${GIT_AUTHOR_NAME:-}" ]; then
  export GIT_AUTHOR_NAME="$OPENCLAW_GIT_USER_NAME"
fi
if [ -z "${GIT_AUTHOR_EMAIL:-}" ]; then
  export GIT_AUTHOR_EMAIL="$OPENCLAW_GIT_USER_EMAIL"
fi
if [ -z "${GIT_COMMITTER_NAME:-}" ]; then
  export GIT_COMMITTER_NAME="$OPENCLAW_GIT_USER_NAME"
fi
if [ -z "${GIT_COMMITTER_EMAIL:-}" ]; then
  export GIT_COMMITTER_EMAIL="$OPENCLAW_GIT_USER_EMAIL"
fi

if command -v git >/dev/null 2>&1; then
  git config --global user.name "$OPENCLAW_GIT_USER_NAME" >/dev/null 2>&1 || true
  git config --global user.email "$OPENCLAW_GIT_USER_EMAIL" >/dev/null 2>&1 || true
fi

if [ "$#" -eq 0 ]; then
  set -- node openclaw.mjs gateway --bind lan --port 18789
fi

exec "$@"
