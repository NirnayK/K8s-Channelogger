########################################
# Builder stage: compile the Go binary  #
########################################
FROM golang:1.24-alpine AS builder

# Set the module root
WORKDIR /workspace

# Copy go.mod and go.sum, download dependencies
COPY code/go.mod code/go.sum ./
RUN go mod download

# Copy all source code into module root
COPY code/ ./

# Build a statically-linked binary including all imports
RUN CGO_ENABLED=0 \
    GOOS=linux \
    go build -a -ldflags="-s -w" -o /workspace/webhook ./cmd/main.go

#############################################
# Final stage: minimal runtime environment   #
#############################################
FROM alpine:3.18 AS runner

# Install only CA certs for TLS validation
RUN apk add --no-cache ca-certificates

# Expose the port you listen on (adjust if needed)
EXPOSE 8443

# Allow mounting certs at /certs
VOLUME ["/certs"]

# Copy the built binary
COPY --from=builder /workspace/webhook /usr/local/bin/webhook

# Switch to non-root user for security
USER nobody:nogroup

# Entrypoint with flags pointing at the mounted TLS cert files
ENTRYPOINT ["/usr/local/bin/webhook", "--tlsCertFile=/certs/server.crt", "--tlsKeyFile=/certs/server.key"]
