# Channelog

Channelog is a webhook service that generates changelogs for Kubernetes resources using an OpenAI compatible API and pushes updates to a Git repository. The project includes Makefile targets and Dockerfiles for building and deploying the service.

## Required Environment Variables

The service configuration is entirely sourced from environment variables. The following variables are required unless noted otherwise:

| Variable               | Description                                                                    | Default |
|------------------------|--------------------------------------------------------------------------------|---------|
| `GIT_REPO`             | URL of the Git repository to update.                                           | –       |
| `GIT_BRANCH`           | Branch to commit changes to.                                                   | –       |
| `USERNAME`             | Git username used for commits.                                                | –       |
| `USER_EMAIL`           | Git email address used for commits.                                           | –       |
| `GIT_TOKEN`            | Git token for HTTPS authentication (optional).                                | –       |
| `OPENAI_API_URL`       | Base URL for the OpenAI compatible API.                                       | `https://api.openai.com/v1` |
| `OPENAI_MODEL`         | Model name to request from the API.                                           | `gpt-4` |
| `OPENAI_API_KEY`       | API key used by the OpenAI client.                                            | –       |
| `SYSTEM_PROMPT`        | System prompt text passed to completions (optional).                          | empty   |
| `USER_MESSAGE_TEMPLATE`| Template for user messages sent to the API (optional).                        | empty   |
| `ADDR`                 | Listen address for the HTTPS server.                                          | `:8443` |
| `DEFAULT_RABBITMQ_QUEUE` | Default queue name for RabbitMQ (optional, used by deployment manifests).   | –       |
| `RABBITMQ_URL`         | RabbitMQ connection URL (optional, used by deployment manifests).            | –       |
| `LOCATION`             | Deployment location string (optional).                                        | –       |

These variables can be provided directly or via Kubernetes secrets. See `deploy/testenv/secret_test.yaml.template` for an example template.

## Building and Running

### Local Build

```bash
# Run go tests and build the binary
make test

# Build the Docker image on the remote build server and push it
make remote-build-and-push ENV=test
```

The `ENV` variable selects the Dockerfile and image repository. Use `ENV=production` to build with `Dockerfile` and push to the production registry.

### Running the Container

After building, run the container and mount the TLS certificates generated in `tls/certs`:

```bash
docker run -p 8443:8443 \
  -v $(pwd)/tls/certs:/certs:ro \
  -e GIT_REPO=... -e GIT_BRANCH=... -e USERNAME=... -e USER_EMAIL=... \
  -e OPENAI_API_KEY=... \
  channelog
```

Alternatively, you can run the service locally with Go:

```bash
cd code
go run ./cmd/main.go --tlsCertFile=../tls/certs/server.crt --tlsKeyFile=../tls/certs/server.key
```

## Kubernetes Deployment

The `deploy/` directory contains manifests for running the service on Kubernetes.

1. **Generate certificates** (stored under `tls/certs`):
   ```bash
   make cert-generate
   ```
2. **Update manifests** with the generated certificates:
   ```bash
   make cert-update ENV=test        # or ENV=production
   ```
3. **Deploy to a cluster** using the provided manifests:
   ```bash
   make k8s-deploy-update ENV=test  # patches image and applies config
   ```

The `deploy/` directory contains production manifests, and `deploy/testenv/` contains the test environment equivalents. Secrets with the required environment variables should be created from `deploy/testenv/secret_test.yaml.template` (for testing) or `deploy/secret.yaml` (for production).

