# syntax=docker/dockerfile:1

FROM ubuntu:24.04 AS frontend-build

ARG NODE_VERSION=24.15.0

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl xz-utils \
    && rm -rf /var/lib/apt/lists/* \
    && node_arch="$(dpkg --print-architecture)" \
    && case "$node_arch" in \
    amd64) node_arch="x64" ;; \
    arm64) node_arch="arm64" ;; \
    *) echo "Unsupported Node.js architecture: $node_arch" >&2; exit 1 ;; \
    esac \
    && curl -fsSL --retry 5 --retry-all-errors -o /tmp/node.tar.xz \
    "https://nodejs.org/dist/v${NODE_VERSION}/node-v${NODE_VERSION}-linux-${node_arch}.tar.xz" \
    && tar -xJf /tmp/node.tar.xz -C /usr/local --strip-components=1 \
    && rm /tmp/node.tar.xz

WORKDIR /src/frontend

COPY frontend/package*.json ./
RUN npm ci

COPY frontend/ ./
RUN npm run build

FROM ubuntu:24.04 AS server-build

ARG GO_VERSION=1.27.4
ARG GO_SHA256_AMD64=7226736c2339719cda28fbf3a54d71aa63bcefe376e98cef00034268dab24840
ARG GO_SHA256_ARM64=3392582988a273a87ab5043ee1b2c9c17a2c3c00580bf9326c47a2e0aaaeac92
ARG VERSION=dev
ENV PATH="/usr/local/go/bin:${PATH}"
ENV GOTOOLCHAIN=local

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl \
    && rm -rf /var/lib/apt/lists/* \
    && go_arch="$(dpkg --print-architecture)" \
    && case "$go_arch" in \
    amd64) go_sha256="$GO_SHA256_AMD64" ;; \
    arm64) go_sha256="$GO_SHA256_ARM64" ;; \
    *) echo "Unsupported Go architecture: $go_arch" >&2; exit 1 ;; \
    esac \
    && curl -fsSL --retry 5 --retry-all-errors -o /tmp/go.tar.gz \
    "https://go.dev/dl/go${GO_VERSION}.linux-${go_arch}.tar.gz" \
    && printf '%s  /tmp/go.tar.gz\n' "$go_sha256" | sha256sum -c - \
    && tar -xzf /tmp/go.tar.gz -C /usr/local \
    && rm /tmp/go.tar.gz

WORKDIR /src

COPY go.mod go.sum ./
RUN go version && go mod download

COPY . ./
COPY --from=frontend-build /src/frontend/dist ./frontend/dist
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags="-s -w -X github.com/voilelab/plainshelf/internal/version.Version=${VERSION}" \
    -o /out/plainshelf-srv ./cmd/plainshelf-srv

FROM ubuntu:24.04 AS runtime

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --create-home --home-dir /home/plainshelf --shell /usr/sbin/nologin plainshelf \
    && mkdir -p /data /etc/plainshelf \
    && chown -R plainshelf:plainshelf /data /home/plainshelf

COPY --from=server-build /out/plainshelf-srv /usr/local/bin/plainshelf-srv
COPY docker/config.yaml /etc/plainshelf/config.yaml

USER plainshelf
WORKDIR /data
VOLUME ["/data"]
EXPOSE 20000

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD curl -fsS http://127.0.0.1:20000/health || exit 1

ENTRYPOINT ["/usr/local/bin/plainshelf-srv"]
CMD ["-conf", "/etc/plainshelf/config.yaml"]
