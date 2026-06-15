FROM golang:1.26-alpine AS builder

# Install SSL ca certificates, tzdata (for timezones) and git
RUN apk update && apk add --no-cache git ca-certificates tzdata && update-ca-certificates

# Create a non-root user and group to run the application
ENV USER=appuser
ENV UID=10001
RUN adduser \
    --disabled-password \
    --gecos "" \
    --home "/nonexistent" \
    --shell "/sbin/nologin" \
    --no-create-home \
    --uid "${UID}" \
    "${USER}"

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .

# Build the Go binary with strict security and optimization flags
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath \
    -ldflags="-w -s" \
    -o /go/bin/microservice ./cmd/api


FROM scratch

COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

COPY --from=builder /go/bin/microservice /microservice

COPY --from=builder /app/web /web

USER appuser:appuser

EXPOSE 8080

# Run the binary directly
ENTRYPOINT ["/microservice"]