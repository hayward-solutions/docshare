FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETARCH
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -trimpath \
    -ldflags="-s -w -X github.com/docshare/api/internal/handlers.Version=${VERSION}" \
    -o /app/bin/server ./cmd/server

# Runtime image. We need a real shell + pandoc for the document export
# feature. Debian's `pandoc` package ships its templates (reference.docx
# etc.) as separate on-disk files via `pandoc-data`, which `--sandbox`
# refuses to read — that breaks DOCX/ODT/EPUB exports. We need pandoc
# with `embed_data_files` baked in, so we fetch the official static
# binary release and copy just that into the image.
FROM debian:bookworm-slim

ARG TARGETARCH
ARG PANDOC_VERSION=3.5

RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends ca-certificates curl; \
    case "$TARGETARCH" in \
        amd64) PANDOC_ARCH=amd64 ;; \
        arm64) PANDOC_ARCH=arm64 ;; \
        *) echo "unsupported arch: $TARGETARCH" >&2; exit 1 ;; \
    esac; \
    curl -fsSL "https://github.com/jgm/pandoc/releases/download/${PANDOC_VERSION}/pandoc-${PANDOC_VERSION}-linux-${PANDOC_ARCH}.tar.gz" \
        | tar -xz -C /tmp; \
    install -m 0755 "/tmp/pandoc-${PANDOC_VERSION}/bin/pandoc" /usr/local/bin/pandoc; \
    rm -rf "/tmp/pandoc-${PANDOC_VERSION}"; \
    apt-get purge -y --auto-remove curl; \
    rm -rf /var/lib/apt/lists/*; \
    pandoc --version | head -n 1

WORKDIR /app

COPY --from=builder /app/bin/server /app/server

EXPOSE 8080

ENTRYPOINT ["/app/server"]
