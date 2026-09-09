#--- opencode

FROM node:24-slim AS base

RUN apt-get update && apt-get install -y --no-install-recommends \
      git ca-certificates curl less \
    && rm -rf /var/lib/apt/lists/*

# install Claude Code
RUN npm i -g opencode-ai

# set up agent user
RUN useradd -m -s /bin/bash agent
USER agent

USER agent
ENTRYPOINT ["sleep", "infinity"]

#--- opencode: go

FROM base AS go

USER root

# install go
ARG GO_VERSION=1.27.0
RUN set -eux; \
    arch="$(dpkg --print-architecture)"; \
    case "$arch" in \
      amd64) goarch="amd64" ;; \
      arm64) goarch="arm64" ;; \
      *) echo "unsupported arch: $arch" >&2; exit 1 ;; \
    esac; \
    curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${goarch}.tar.gz" -o /tmp/go.tgz; \
    tar -C /usr/local -xzf /tmp/go.tgz; \
    rm /tmp/go.tgz

ENV PATH="/usr/local/go/bin:${PATH}"

USER agent
ENTRYPOINT ["sleep", "infinity"]
