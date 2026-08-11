.PHONY: test lint sync sync-full serve build

test:
	go test ./...

lint:
	go vet ./...

sync:
	go run ./cmd/bluesky-importer sync

sync-full:
	go run ./cmd/bluesky-importer sync --full

serve:
	hugo server -D

build:
	hugo --minify
