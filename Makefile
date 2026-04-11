.PHONY: build test lint docs

build:
	go build ./...

test:
	go test ./...

lint:
	go vet ./...

docs:
	ENTREE_GEN_DOCS=1 go run -tags=docs ./cmd/entree docs/cli.md
