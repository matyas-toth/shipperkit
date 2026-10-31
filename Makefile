GO ?= go

.PHONY: dev build test test-e2e test-docker check

dev:
	npm run dev

build:
	npm run build

test:
	$(GO) test ./...
	npm run test --prefix web

test-e2e:
	npm run test:e2e

test-docker:
	npm run test:docker

check:
	$(GO) vet ./...
	npm run check

