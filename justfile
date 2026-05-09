set dotenv-load

secrets-local:
    sops -d secrets/local.env > .env

edit-secrets HOST:
    sops secrets/{{HOST}}.env

# Build collector binary and Docker image, save as tar
build-collector:
    #!/usr/bin/env bash
    set -euo pipefail
    go build -o osrs-flips-collector ./cmd/collector/
    ~/.local/bin/nerdctl build -t osrs-flips-collector:latest -f Dockerfile.collector .
    ~/.local/bin/nerdctl save osrs-flips-collector:latest -o osrs-flips-collector-latest.tar

# Build UI Docker image, save as tar
build-ui:
    #!/usr/bin/env bash
    set -euo pipefail
    mkdir -p ui/certs
    ~/.local/bin/nerdctl build -t osrs-flips-ui:latest -f ui/Dockerfile ui/
    ~/.local/bin/nerdctl save osrs-flips-ui:latest -o osrs-flips-ui-latest.tar

# Build all images
build: build-collector build-ui

# Deploy to remote host
deploy HOST:
    #!/usr/bin/env bash
    set -euo pipefail
    ssh {{HOST}} -C "mkdir -p ~/images ~/caddyfiles ~/osrs-flips"
    sops -d secrets/{{HOST}}.env | ssh {{HOST}} "cat > ~/osrs-flips/.env"
    scp caddyfiles/{{HOST}} {{HOST}}:~/caddyfiles/osrs-flips.caddy
    scp docker-compose.yml {{HOST}}:~/osrs-flips-compose.yaml
    scp osrs-flips-collector-latest.tar {{HOST}}:~/images/
    scp osrs-flips-ui-latest.tar {{HOST}}:~/images/
    ssh {{HOST}} -C "~/.local/bin/nerdctl load -i ~/images/osrs-flips-collector-latest.tar"
    ssh {{HOST}} -C "~/.local/bin/nerdctl load -i ~/images/osrs-flips-ui-latest.tar"
    ssh {{HOST}} -C "~/.local/bin/nerdctl compose -f ~/osrs-flips-compose.yaml down"
    ssh {{HOST}} -C "~/.local/bin/nerdctl compose -f ~/osrs-flips-compose.yaml up -d --env-file ~/osrs-flips/.env"

# Deploy a tagged build pulled from the registry
deploy-prod HOST TAG:
    #!/usr/bin/env bash
    set -euo pipefail
    REGISTRY="gitea.zen.lofi"
    REPO="sfi/osrs-flips"
    COLLECTOR_IMAGE="$REGISTRY/$REPO-collector:{{TAG}}"
    UI_IMAGE="$REGISTRY/$REPO-ui:{{TAG}}"
    ssh {{HOST}} -C "mkdir -p ~/caddyfiles ~/osrs-flips"
    sops -d secrets/{{HOST}}.env | ssh {{HOST}} "cat > ~/osrs-flips/.env"
    scp caddyfiles/{{HOST}} {{HOST}}:~/caddyfiles/osrs-flips.caddy
    scp docker-compose.yml {{HOST}}:~/osrs-flips-compose.yaml
    ssh {{HOST}} -C "~/.local/bin/nerdctl pull $COLLECTOR_IMAGE"
    ssh {{HOST}} -C "~/.local/bin/nerdctl pull $UI_IMAGE"
    ssh {{HOST}} -C "OSRS_FLIPS_COLLECTOR_IMAGE=$COLLECTOR_IMAGE OSRS_FLIPS_UI_IMAGE=$UI_IMAGE \
        ~/.local/bin/nerdctl compose -f ~/osrs-flips-compose.yaml down"
    ssh {{HOST}} -C "OSRS_FLIPS_COLLECTOR_IMAGE=$COLLECTOR_IMAGE OSRS_FLIPS_UI_IMAGE=$UI_IMAGE \
        ~/.local/bin/nerdctl compose -f ~/osrs-flips-compose.yaml up -d --env-file ~/osrs-flips/.env"

# Run a specific job (example with "Tempting Trades Under 1M")
run JOB_NAME:
    go run cmd/main.go -job="{{JOB_NAME}}"

# Run all enabled jobs
run-all:
    go run cmd/main.go -all

# Show help for CLI options
run-help:
    go run cmd/main.go -help

bot:
    go run cmd/bot/main.go

up:
    nerdctl compose up -d

down:
    nerdctl compose down

logs *ARGS:
    nerdctl compose logs {{ARGS}}

test:
    go test -v ./... | tee test.log

test-coverage:
    go test -v -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out -o coverage.html

lint: lint-collector lint-ui

lint-collector:
    golangci-lint run ./...

lint-ui:
    cd ui && npm run lint

fmt:
    go fmt ./...

vet:
    go vet ./...

clean:
    rm -f osrs-flips osrs-flips-bot osrs-flips-collector coverage.out coverage.html

collector *ARGS:
    go run ./cmd/collector/ {{ARGS}}

# Run collector in backfill mode (fetches historical data)
backfill *ARGS:
    go run ./cmd/collector/ -backfill {{ARGS}}
