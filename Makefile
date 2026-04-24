.PHONY: all
all: build

.PHONY: build
build:
	go build -o ./bin/fumi ./cmd/fumi
	go build -o ./bin/fumi-host ./cmd/fumi-host

.PHONY: test
test:
	go test ./...

.PHONY: check
check:
	go vet ./...
	go test ./...

.PHONY: keygen
keygen:
	@test -f .dev/key.pem || (mkdir -p .dev && openssl genrsa -out .dev/key.pem 2048)

.PHONY: build-dev
build-dev: keygen
	./scripts/build-dev.sh

.PHONY: clean
clean:
	rm -rf ./bin chrome-extension/dist
