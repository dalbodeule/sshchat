# Stage 1: Build the Go application
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum first to leverage Docker cache
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the application source code
COPY . .

# Build the Go application
# CGO_ENABLED=0 disables CGO, creating a statically linked binary
# -o /app/main specifies the output path and name of the executable
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /app/main .

# Stage 2: Create a minimal runtime image
FROM alpine:latest

# Install ca-certificates for HTTPS support if needed
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the built executable from the builder stage
COPY --from=builder /app/main .

# Expose the port your application listens on (e.g., 2222)
EXPOSE 2222
ENV PORT=2222
ENV ROOT_PATH="/app/data"

# Command to run the application when the container starts
CMD ["./main"]