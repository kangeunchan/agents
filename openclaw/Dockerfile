# syntax=docker/dockerfile:1.7
ARG OPENCLAW_BASE_IMAGE=ghcr.io/openclaw/openclaw@sha256:c6528e72dbeeebca3c928e6d6109ef991d249b4939a093c7f9e28708d24fc059
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS openclawctl-builder
WORKDIR /src

COPY openclawctl/go.mod ./openclawctl/
COPY openclawctl/go.sum ./openclawctl/
WORKDIR /src/openclawctl
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY openclawctl/ .
ARG TARGETOS TARGETARCH
RUN --mount=type=cache,target=/root/.cache/go-build \
	CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags='-s -w' -o /out/openclawctl ./cmd/openclawctl

FROM ${OPENCLAW_BASE_IMAGE}
USER root
COPY --from=openclawctl-builder /out/openclawctl /usr/local/bin/openclawctl
COPY openclaw/scripts/container-entrypoint.sh /usr/local/bin/openclaw-entrypoint.sh
RUN chmod +x /usr/local/bin/openclawctl /usr/local/bin/openclaw-entrypoint.sh \
	&& mkdir -p /etc/openclaw-config /var/lib/openclaw \
	&& chown -R node:node /etc/openclaw-config /var/lib/openclaw

ENV OPENCLAW_CONFIG_PATH=/etc/openclaw-config/openclaw.json
ENV OPENCLAW_STATE_DIR=/var/lib/openclaw
ENV OPENCLAW_GIT_USER_NAME=OpenClaw
ENV OPENCLAW_GIT_USER_EMAIL=openclaw@local

USER node
EXPOSE 18789
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 CMD ["node", "-e", "const c=new AbortController();setTimeout(()=>c.abort(),4500);fetch('http://127.0.0.1:18789/health',{signal:c.signal}).then(r=>process.exit(r.ok?0:1)).catch(()=>process.exit(1));"]
ENTRYPOINT ["/usr/local/bin/openclaw-entrypoint.sh"]
CMD ["node", "openclaw.mjs", "gateway", "--bind", "lan", "--port", "18789"]
