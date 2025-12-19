# multi-stage build for go lang application
# 1. build stage
FROM golang:1.25 as builder

WORKDIR /app
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o ./pbuf-migrations ./.
RUN CGO_ENABLED=0 GOOS=linux go build -o ./pbuf-registry ./cmd/...

# 2. run stage
FROM bash:alpine3.23

WORKDIR /app

COPY --from=builder /app/pbuf-migrations /app/pbuf-migrations
COPY --from=builder /app/pbuf-registry /app/pbuf-registry

CMD ["/app/pbuf-registry"]