FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETARCH
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -trimpath \
    -ldflags="-s -w -X github.com/docshare/backend/internal/handlers.Version=${VERSION}" \
    -o /app/bin/server ./cmd/server

FROM gcr.io/distroless/static-debian12

WORKDIR /app

COPY --from=builder /app/bin/server /app/server

EXPOSE 8080

ENTRYPOINT ["/app/server"]
