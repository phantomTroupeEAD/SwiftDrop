# syntax=docker/dockerfile:1.7
# Stage 1: Build static standalone binary
FROM golang:1.22-alpine AS builder

WORKDIR /build

RUN apk add --no-cache ca-certificates tzdata git

COPY go.mod go.sum* ./
RUN go mod download 2>/dev/null || true

COPY . .

ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

RUN go build -trimpath -ldflags="-w -s" -o /bin/tracking ./cmd/tracking

# Stage 2: Hardened minimal runtime image
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata wget && \
  addgroup -g 10001 -S swiftdrop && \
  adduser -u 10001 -S swiftdrop -G swiftdrop

WORKDIR /app

COPY --from=builder /bin/tracking /app/tracking

USER swiftdrop:swiftdrop

EXPOSE 8081

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --quiet --tries=1 --spider http://localhost:8081/health/live || exit 1

ENTRYPOINT ["/app/tracking"]

