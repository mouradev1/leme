# Linux build environment for leme (Wails v2 needs GTK3 + WebKitGTK headers,
# so it can't be cross-compiled from macOS). Ubuntu 22.04 = glibc 2.35, so the
# binary runs on Ubuntu 22.04+, Debian 12+, Fedora 36+, Mint 21+ and newer.
FROM ubuntu:22.04

ARG TARGETARCH
ARG GO_VERSION=1.26.9
ARG NODE_VERSION=20.20.2
ARG WAILS_VERSION=v2.16.0

RUN apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
      build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev \
      curl ca-certificates xz-utils git dpkg-dev file \
 && rm -rf /var/lib/apt/lists/*

RUN curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${TARGETARCH}.tar.gz" | tar -C /usr/local -xz
RUN NODE_ARCH=$([ "$TARGETARCH" = "amd64" ] && echo x64 || echo arm64) \
 && curl -fsSL "https://nodejs.org/dist/v${NODE_VERSION}/node-v${NODE_VERSION}-linux-${NODE_ARCH}.tar.xz" \
    | tar -C /usr/local --strip-components=1 -xJ

ENV PATH=/usr/local/go/bin:/root/go/bin:$PATH
RUN go install github.com/wailsapp/wails/v2/cmd/wails@${WAILS_VERSION}
