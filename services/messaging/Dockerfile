# Stage 1: Build static binaries
FROM golang:1.22-alpine AS builder

WORKDIR /build

RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

RUN go build -ldflags="-w -s" -o /bin/relay ./cmd/relay && \
    go build -ldflags="-w -s" -o /bin/worker ./cmd/worker && \
    go build -ldflags="-w -s" -o /bin/reconcile ./cmd/reconcile && \
    go build -ldflags="-w -s" -o /bin/devemit ./cmd/devemit && \
    go build -ldflags="-w -s" -o /bin/fake-receiver ./cmd/fake-receiver

# Stage 2: Minimal hardened runtime image
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -g 10001 -S swiftdrop && \
    adduser -u 10001 -S swiftdrop -G swiftdrop

COPY --from=builder /bin/relay /usr/local/bin/relay
COPY --from=builder /bin/worker /usr/local/bin/worker
COPY --from=builder /bin/reconcile /usr/local/bin/reconcile
COPY --from=builder /bin/devemit /usr/local/bin/devemit
COPY --from=builder /bin/fake-receiver /usr/local/bin/fake-receiver

USER swiftdrop:swiftdrop

ENTRYPOINT ["/usr/local/bin/worker"]

