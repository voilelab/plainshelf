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
    && curl -fsSL "https://nodejs.org/dist/v${NODE_VERSION}/node-v${NODE_VERSION}-linux-${node_arch}.tar.xz" \
        | tar -xJ -C /usr/local --strip-components=1

WORKDIR /src/frontend

COPY frontend/package*.json ./
RUN npm ci

COPY frontend/ ./
RUN npm run build

FROM ubuntu:24.04 AS server-build

ARG GO_VERSION=1.26.1
ENV PATH="/usr/local/go/bin:${PATH}"

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl \
    && rm -rf /var/lib/apt/lists/* \
    && go_arch="$(dpkg --print-architecture)" \
    && case "$go_arch" in \
        amd64) go_arch="amd64" ;; \
        arm64) go_arch="arm64" ;; \
        *) echo "Unsupported Go architecture: $go_arch" >&2; exit 1 ;; \
    esac \
    && curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${go_arch}.tar.gz" \
        | tar -xz -C /usr/local

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
COPY --from=frontend-build /src/frontend/dist ./frontend/dist
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/plainshelf-srv ./cmd/plainshelf-srv

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
