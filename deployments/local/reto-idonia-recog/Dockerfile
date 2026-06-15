FROM golang:1.26-alpine

RUN apk add --no-cache git build-base

WORKDIR /app

RUN go install github.com/air-verse/air@latest && \
    go install github.com/go-delve/delve/cmd/dlv@latest && \
    go install github.com/joho/godotenv/cmd/godotenv@latest