# syntax=docker/dockerfile:1
#
# Optional container image for mtg-mcp. Using Docker is never required — the
# native `make build` / `./mtg-mcp` flow is unchanged. This Dockerfile has two
# selectable targets:
#   * runtime (default) — a tiny distroless image holding just the stdio binary.
#                         Drive it with `docker run -i --rm` from an MCP client.
#   * proxy             — Node + mcp-proxy wrapping the binary to expose the
#                         stdio server over Streamable HTTP (claude.ai connector).
# The build context is the repo root; run these from there:
#   Build the stdio image:  docker build -f docker/Dockerfile -t mtg-mcp:latest .
#   Build the proxy image:  docker build -f docker/Dockerfile --target proxy -t mtg-mcp:proxy .
# (or just `make docker-build` / `docker compose up`.)

# ---- Builder ----------------------------------------------------------------
# Pinned to satisfy the `go 1.26.4` directive in go.mod. GOTOOLCHAIN=local turns
# an unexpected toolchain bump into a hard build failure instead of a silent
# download. --platform=$BUILDPLATFORM runs the compiler natively (fast) and
# cross-compiles to TARGETOS/TARGETARCH below, so arm64 hosts get a native image.
FROM --platform=$BUILDPLATFORM golang:1.26.4-alpine AS builder
ENV GOTOOLCHAIN=local CGO_ENABLED=0
WORKDIR /src

# Cache-friendly: resolve deps before copying the rest of the source.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

# buildx injects TARGETOS/TARGETARCH; on the legacy builder they are empty and
# the build falls back to the native target — both are correct.
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_DATE=unknown
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -buildvcs=false \
      -ldflags="-w -s -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildDate=${BUILD_DATE}" \
      -o /out/mtg-mcp -v . \
 && mkdir -p /out/data

# ---- Proxy target: stdio -> Streamable HTTP via mcp-proxy (needs Node) -------
# Built explicitly with `--target proxy`. Long-lived; publishes an HTTP port.
FROM node:22-alpine AS proxy
# ca-certificates for the Go binary's outbound HTTPS calls; pin mcp-proxy.
RUN apk add --no-cache ca-certificates \
 && npm install -g mcp-proxy@latest \
 && mkdir -p /data && chown node:node /data
COPY --from=builder /out/mtg-mcp /usr/local/bin/mtg-mcp
USER node
WORKDIR /data
EXPOSE 8080
# DEFAULT (stateful) mode — never pass --stateless: it breaks the MCP Apps
# tool-call -> resources/read round-trip. Internal port fixed at 8080; remap on
# the host side. The binary logs to stderr, so mcp-proxy's stdout stays clean.
ENTRYPOINT ["mcp-proxy", "--port", "8080", "--", "/usr/local/bin/mtg-mcp"]

# ---- Runtime target: pure stdio MCP command (DEFAULT stage = smallest) -------
FROM gcr.io/distroless/static-debian12:nonroot AS runtime
LABEL org.opencontainers.image.source="https://github.com/nathanmartins/mtg-mcp"
LABEL org.opencontainers.image.description="MTG Commander MCP server (stdio)"
COPY --from=builder /out/mtg-mcp /usr/local/bin/mtg-mcp
# Writable, nonroot-owned CWD for the mandatory mtg-commander-server.log (the app
# os.Exit(1)s if it can't open it). distroless has no shell to chown at build
# time, so the dir is created + chowned in the builder stage and copied here.
COPY --from=builder --chown=65532:65532 /out/data /data
WORKDIR /data
# `docker run -i --rm <image>` is then a valid MCP stdio command. Do not add -t:
# a TTY corrupts JSON-RPC framing. distroless static already ships CA certs.
ENTRYPOINT ["/usr/local/bin/mtg-mcp"]
