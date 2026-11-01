# syntax=docker/dockerfile:1.7
# core-api image (the admin module is part of it). Build from the repo root:
#   docker build -f core-api/Dockerfile -t ghcr.io/<org>/swiftdrop/core-api:<git-sha> core-api
# Rules: pinned versions, no "latest", non-root 10001, read-only root FS (SEC-012).

FROM golang:1.22.12-alpine3.21 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Static binary, no C library needed; -trimpath removes local paths from the binary.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/core-api ./cmd/api \
 && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/seed ./cmd/seed

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/core-api /core-api
# Seed is in the image for local and staging only; it refuses APP_ENV=production.
COPY --from=build /out/seed /seed
COPY migrations /migrations
USER 10001:10001
EXPOSE 8080
# distroless has no curl or shell, so the binary checks itself.
HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=3 CMD ["/core-api", "-healthcheck"]
ENTRYPOINT ["/core-api"]
