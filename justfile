set dotenv-load

# Build all binaries and images
build: build-bot build-collector

# Build bot binary and Docker image
build-bot:
    go build -o osrs-flips-bot cmd/bot/main.go
    nerdctl build -t osrs-flips-bot:latest -f Dockerfile .

# Build collector binary and Docker image
build-collector:
    go build -o osrs-flips-collector cmd/collector/main.go
    nerdctl build -t osrs-flips-collector:latest -f Dockerfile.collector .

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

lint:
    golangci-lint run ./...

fmt:
    go fmt ./...

vet:
    go vet ./...

clean:
    rm -f osrs-flips osrs-flips-bot osrs-flips-collector coverage.out coverage.html

collector:
    go run cmd/collector/main.go

# Run collector in backfill mode (fetches historical data)
backfill *ARGS:
    go run cmd/collector/main.go -backfill {{ARGS}}
