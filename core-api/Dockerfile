# syntax=docker/dockerfile:1.7
# Stage 1: Build static binaries
FROM golang:1.22-alpine AS builder

WORKDIR /build

RUN apk add --no-cache ca-certificates tzdata git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

RUN go build -trimpath -ldflags="-w -s" -o /bin/core-api ./cmd/api && \
    go build -trimpath -ldflags="-w -s" -o /bin/migrate ./cmd/migrate

# Stage 2: Hardened minimal runtime image (SEC-016)
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -g 10001 -S swiftdrop && \
    adduser -u 10001 -S swiftdrop -G swiftdrop

WORKDIR /app

RUN mkdir -p /app/data/proofs && chown -R swiftdrop:swiftdrop /app/data

COPY --from=builder /bin/core-api /app/core-api
COPY --from=builder /bin/migrate /app/migrate
COPY migrations /app/migrations

USER swiftdrop:swiftdrop

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --quiet --tries=1 --spider http://localhost:8080/health/live || exit 1

ENTRYPOINT ["/app/core-api"]

