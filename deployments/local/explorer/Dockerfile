FROM golang:1.26-alpine AS builder

WORKDIR /app

RUN go mod init ledger-explorer

COPY ledger-explorer/ .

RUN go mod tidy

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o explorer explorer.go

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/explorer .
COPY --from=builder /app/index.html .

EXPOSE 8080

CMD ["./explorer"]