FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=$(git describe --tags --always 2>/dev/null || echo dev)" \
    -o /bin/brand ./cmd/brand/

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /bin/brand /usr/local/bin/brand
ENV PORT=8750 \
    DATA_DIR=/data \
    BRAND_ADMIN_KEY=""
EXPOSE 8750
ENTRYPOINT ["brand"]
