FROM mcr.microsoft.com/devcontainers/go:1.27-bookworm

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        qemu-system-x86 \
        qemu-utils \
        make \
        procps \
        iproute2 \
        ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /workspaces/hyve