FROM golang:1.23-alpine AS builder
RUN apk add --no-cache git
WORKDIR /app
COPY . .
RUN go mod tidy && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /poller ./cmd/poller && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /recalc ./cmd/recalc

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /poller /poller
COPY --from=builder /recalc /recalc
ENTRYPOINT ["/poller"]
