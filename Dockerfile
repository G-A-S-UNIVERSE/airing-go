# syntax=docker/dockerfile:1

FROM golang:1.23-alpine AS builder
WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY cmd/ cmd/
COPY internal/ internal/
COPY pkg/ pkg/

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/airing ./cmd/airing

FROM alpine:3.20
RUN apk add --no-cache ca-certificates \
    && addgroup -S airing \
    && adduser -S -G airing airing

COPY --from=builder /out/airing /usr/local/bin/airing

USER airing
EXPOSE 8090

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8090/health || exit 1

ENTRYPOINT ["/usr/local/bin/airing"]
