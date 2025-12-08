# Build Stage
FROM golang:1.24-alpine AS builder

# Install necessary build tools
RUN apk add --no-cache git make

# Set working directory
WORKDIR /app

# 1. Copy go.mod and go.sum first to leverage Docker layer caching
COPY go.mod go.sum ./
RUN go mod download

# 2. Copy the entire source code
COPY . .

# 3. Build the specific service
# APP_PATH is passed via --build-arg (e.g., cmd/color_game/microservices/gms)
ARG APP_PATH
RUN if [ -z "$APP_PATH" ]; then echo "APP_PATH is required" && exit 1; fi

# Build the binary named "app"
# We turn off CGO for a purely static binary (easier for Alpine)
RUN CGO_ENABLED=0 GOOS=linux go build -o app ./$APP_PATH

# Runtime Stage
FROM alpine:3.19

# Install runtime dependencies
# graphviz is needed for pprof graph generation
# go is needed for 'go tool pprof'
RUN apk add --no-cache ca-certificates graphviz go tzdata

# Set timezone to Asia/Taipei (Optional but recommended)
ENV TZ=Asia/Taipei

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/app .

# Copy any necessary config files or assets
# Adjust this part if you have specific config folders
# COPY --from=builder /app/config ./config

# Expose ports (Documentary only, K8s yaml defines actual ports)
EXPOSE 8080 9090

# Run the application
CMD ["./app"]
