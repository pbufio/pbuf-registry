# multi-stage build for go lang application
# 1. build stage
FROM golang:1.21 as builder

WORKDIR /app
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o pbuf-registry ./cmd/...

# 2. run stage
FROM bash:alpine3.18

WORKDIR /app

COPY --from=builder /app/pbuf-registry /app/pbuf-registry

CMD ["/app/pbuf-registry"]