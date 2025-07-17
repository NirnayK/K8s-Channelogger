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
    go build -a -ldflags="-s -w" -o /workspace/channelog ./cmd/main.go

#############################################
# Final stage: minimal runtime environment   #
#############################################
FROM ubuntu:22.04 AS runner

# Install only what we need for TLS + Git + SSH tools
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      ca-certificates \
      git \
      openssh-client \
      bash \
 && rm -rf /var/lib/apt/lists/*

# Expose the port you listen on (adjust if needed)
EXPOSE 8443

# Allow mounting certs at /certs
VOLUME ["/certs"]

# Copy the built binary
COPY --from=builder /workspace/channelog /usr/local/bin/channelog
COPY scripts/entrypoint-production.sh /usr/local/bin/entrypoint-production.sh
RUN chmod +x /usr/local/bin/entrypoint-production.sh

# Use root for initial setup, but entrypoint can switch users if needed
USER root

# Entrypoint with flags pointing at the mounted TLS cert files
ENTRYPOINT ["/usr/local/bin/entrypoint-production.sh"]
CMD ["--tlsCertFile=/certs/server.crt", "--tlsKeyFile=/certs/server.key"]
