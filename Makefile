.PHONY: help build vet test race bench lint vuln deps tidy
.DEFAULT_GOAL := help

help:
	@echo "Usage: make <target> [ROOT=<dir>]"
	@echo ""
	@echo "Targets:"
	@echo "  build        Build all packages"
	@echo "  vet          Run go vet"
	@echo "  test         Run all tests"
	@echo "  race         Run all tests with the race detector"
	@echo "  bench        Run benchmarks"
	@echo "  lint         Run golangci-lint"
	@echo "  vuln         Run govulncheck"
	@echo "  deps         Update dependencies"
	@echo "  tidy         Tidy go.mod"
	@echo ""
	@echo "CI runs on GitHub Actions (.github/workflows/test.yml)."

build:
	go build ./...

vet:
	go vet ./...

test:
	go test ./...

race:
	go test -race ./...

bench:
	go test -bench=. ./...

lint:
	golangci-lint run

vuln:
	govulncheck ./...

deps:
	go get -u ./...

tidy:
	go mod tidy
