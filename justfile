set dotenv-load

build:
    go build -o osrs-flips cmd/main.go
    go build -o osrs-flips-bot cmd/bot/main.go
    nerdctl build -t osrs-flips-bot:latest .

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
    rm -f osrs-flips osrs-flips-bot coverage.out coverage.html

collector:
    go run cmd/collector/main.go
