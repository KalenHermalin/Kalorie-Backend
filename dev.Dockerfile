# Use the official Go image
FROM golang:1.25-alpine

WORKDIR /app

# Install 'air' for live-reloading code changes
RUN go install github.com/air-verse/air@latest

# Copy dependencies first for better caching
COPY go.mod go.sum ./
RUN go mod download

# We do NOT copy code here; docker-compose will mount it
EXPOSE 8080

# Start air using your config file
CMD ["air", "-c", ".air.toml"]
