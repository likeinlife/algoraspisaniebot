FROM golang:1.23-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/algobot ./cmd/bot

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S bot \
    && adduser -S -G bot -u 10001 bot \
    && mkdir -p /data \
    && chown bot:bot /data

COPY --from=builder /out/algobot /usr/local/bin/algobot

ENV SQLITE_PATH=/data/bot.db
VOLUME ["/data"]
USER bot

ENTRYPOINT ["/usr/local/bin/algobot"]
