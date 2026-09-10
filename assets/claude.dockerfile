#--- claude
FROM node:24-trixie-slim AS base

RUN apt-get update && apt-get install -y --no-install-recommends \
      git ca-certificates curl less \
      ripgrep jq make procps unzip xz-utils patch diffutils file tree \
      python3 fd-find netcat-openbsd dnsutils strace \
    && rm -rf /var/lib/apt/lists/*

# install Claude Code
RUN npm install -g @anthropic-ai/claude-code

# set up agent user
ARG UID=1000
ARG GID=1000

# on linux when mounting it take user node:1000, so set agent as 1000
RUN (userdel -r node 2>/dev/null || true) \
 && groupadd -o -g ${GID} agent \
 && useradd -o -m -u ${UID} -g ${GID} -s /bin/bash agent

USER agent

# install rtk
RUN curl -fsSL https://raw.githubusercontent.com/rtk-ai/rtk/refs/heads/master/install.sh | sh
ENV PATH="/home/agent/.local/bin:${PATH}"

ENTRYPOINT ["sleep", "infinity"]

#--- claude: go

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
