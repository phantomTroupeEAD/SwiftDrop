# core-api (merchants slice). Multi-stage, static binary, distroless, non-root (SRS SEC-016).
FROM golang:1.22-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/seed ./cmd/seed

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/api /app/api
COPY --from=build /out/seed /app/seed
USER 10001:10001
EXPOSE 8080
# No shell in distroless: the binary checks its own /livez.
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 CMD ["/app/api", "-healthcheck"]
ENTRYPOINT ["/app/api"]
