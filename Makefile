NAME=actop
VERSION=$(shell cat VERSION)
BUILD=$(shell git rev-parse --short HEAD)
LD_FLAGS="-w -X main.version=$(VERSION) -X main.build=$(BUILD)"

build:
	go mod download
	CGO_ENABLED=0 go build -ldflags $(LD_FLAGS) -o actop .

run-dev:
	go build -ldflags $(LD_FLAGS) -o actop .
	./actop

test:
	go test ./...

lint:
	golangci-lint run --timeout=5m

fixtures:
	container ls -a --format json > cli/testdata/ls.json
	container stats --no-stream --format json > cli/testdata/stats.json
	container inspect $$(container ls --format json | python3 -c "import sys,json;print(json.load(sys.stdin)[0]['id'])") > cli/testdata/inspect.json
	@echo "WARNING: anonymize secrets in cli/testdata/*.json before committing!"

clean:
	rm -f actop

.PHONY: build run-dev test lint fixtures clean
