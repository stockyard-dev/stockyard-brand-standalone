FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build \
    -ldflags="-s -w -X main.version=$(git describe --tags --always 2>/dev/null || echo dev)" \
    -o /bin/brand ./cmd/brand/

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata curl

COPY --from=builder /bin/brand /usr/local/bin/brand

# Environment variables — override at runtime
# DATA_DIR should be backed by a persistent volume in production
ENV PORT="8750" \
    DATA_DIR="/data" \
    BRAND_ADMIN_KEY="changeme" \
    BRAND_LICENSE_KEY=""

EXPOSE 8750

# Healthcheck — adjust interval for your use case
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -sf http://localhost:8750/health || exit 1

ENTRYPOINT ["brand"]
