# Build Stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build tools and git
RUN apk add --no-cache git

# Copy dependency manifests and download modules
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build statically compiled binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o minimate-bot .

# Final Lightweight Runtime Stage
FROM alpine:latest

WORKDIR /app

# Install CA certificates for secure HTTPS/TLS Telegram API requests
RUN apk --no-cache add ca-certificates tzdata

# Copy binary from builder
COPY --from=builder /app/minimate-bot .

# Copy Intro video if present
COPY Intro.mp4* ./

# Expose HTTP port for Render Web Service health checking
EXPOSE 8080

# Run the bot
CMD ["./minimate-bot"]
