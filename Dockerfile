# Stage 1: Build the binary
FROM golang:1.20-alpine AS builder
WORKDIR /app

# Install necessary build tools
RUN apk add --no-cache git

# Copy go.mod and go.sum, then download modules
COPY go.mod go.sum ./
RUN go mod download

# Copy the entire source code.
COPY . .

# Build the app. We assume your main file is at cmd/app/main.go.
RUN CGO_ENABLED=1 GOOS=linux go build -o app ./cmd/app

# Stage 2: Create a minimal image with the binary.
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
# Copy the built binary from the builder stage.
COPY --from=builder /app/app .
# (Optional) Copy the .env file if you want to include it.
# It’s usually better to inject these as environment variables.
COPY .env . 
# Expose the port your app listens on.
EXPOSE 8080
# Run the binary.
CMD ["./app"]
