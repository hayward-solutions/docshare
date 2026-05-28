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

# Runtime image: debian-slim instead of distroless so we can install
# pandoc, which the editor's "Export as DOCX/ODT/HTML/EPUB" feature shells
# out to. PDF export goes through Gotenberg (the chromium HTML route), so
# LaTeX is not needed and the image stays modest.
FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        pandoc \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /app/bin/server /app/server

EXPOSE 8080

ENTRYPOINT ["/app/server"]
