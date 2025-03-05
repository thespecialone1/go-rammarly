# syntax=docker/dockerfile:1
FROM golang:1.24-alpine AS builder
WORKDIR /app

# Cache dependency downloads.
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the code.
COPY . .

# Run your build command.
RUN go mod tidy && go build -v -o app ./cmd/app/main.go

# Final image.
FROM alpine:latest
WORKDIR /app

# Install necessary packages.
RUN apk --no-cache add ca-certificates sqlite

# Copy the compiled binary and assets.
COPY --from=builder /app/app .
COPY --from=builder /app/static ./static
COPY --from=builder /app/templates ./templates

# (Optional) Pre-create an empty sqlite DB if needed.
# COPY --from=builder /app/db.sqlite .

# Expose the port defined in .env (production uses PORT=8080).
EXPOSE 8080

CMD ["./app"]
