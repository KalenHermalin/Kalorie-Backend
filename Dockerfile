# Stage 1: Build
# UPDATED: Using Go 1.25 for the build environment
FROM golang:1.25-alpine AS builder

# Install certificates so Go can verify Google/GitHub SSL connections
RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Leverage Docker cache for dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Compile the binary
# CGO_ENABLED=0 ensures a portable, static binary
RUN CGO_ENABLED=0 GOOS=linux go build -o main ./cmd/...

# Stage 2: Runtime
FROM alpine:latest

# Bring over the CA certificates so OAuth exchanges work
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

WORKDIR /root/

COPY --from=builder /app/migrations /root/migrations/
# Copy the binary from the builder stage
COPY --from=builder /app/main .

# Heroku will assign a dynamic $PORT; your code must use os.Getenv("PORT")
CMD ["./main"]
