FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o ipam .

FROM alpine:latest

RUN apk --no-cache add ca-certificates bash

WORKDIR /app
COPY --from=builder /app/ipam /usr/local/bin/ipam

EXPOSE 8080 5000

CMD ["ipam", "server"]