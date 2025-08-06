set dotenv-load

build:
    go build -o osrs-flips cmd/main.go
    go build -o osrs-flips-bot cmd/main.go
    nerdctl build -t osrs-flips-bot:latest .

run:
    go run cmd/main.go

bot:
    go run cmd/bot/main.go

up *ARGS:
    nerdctl compose up {{ARGS}}

down *ARGS:
    nerdctl compose down {{ARGS}}

logs *ARGS:
    nerdctl compose logs {{ARGS}}

test:
    go test -v ./...

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

