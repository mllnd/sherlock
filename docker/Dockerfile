FROM golang:1.22-alpine AS builder

LABEL org.opencontainers.image.title="sherlock" \
      org.opencontainers.image.description="Prometheus exporter for Redfish-enabled BMCs" \
      org.opencontainers.image.source="https://github.com/mllnd/sherlock" \
      org.opencontainers.image.authors="mllnd"

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o sherlock ./cmd/sherlock

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/sherlock .

RUN adduser -D -g '' sherlock && \
    chown -R sherlock:sherlock /app

USER sherlock

EXPOSE 9290

# Run the application
CMD ["./sherlock"]
