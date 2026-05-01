FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /poller ./cmd/poller

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /poller /poller
ENTRYPOINT ["/poller"]
