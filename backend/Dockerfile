FROM golang:1.26-alpine AS builder
RUN apk --no-cache add gcc musl-dev linux-headers
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o /flagmate ./cmd/server

FROM alpine:3.19
RUN apk --no-cache add ca-certificates iptables
WORKDIR /app
COPY --from=builder /flagmate .
EXPOSE 8080
ENTRYPOINT ["./flagmate"]
