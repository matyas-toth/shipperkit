GO ?= go

.PHONY: dev build test test-e2e test-docker test-deploy test-package check

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

test-deploy:
	npm run test:deploy

test-package:
	npm run test:package

check:
	$(GO) vet ./...
	npm run check

