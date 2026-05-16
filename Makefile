.PHONY: run generate build test clojure-compat-report clean lint install-golangci-lint

run: build
	./lg

generate:
	go run ./cmd/lgbgen

build: lg.go pkg/**/*
	go build -ldflags="-s -w" -o lg .

test: pkg/**/*
	go test -count=1 -v ./test

clojure-compat-report:
	@./scripts/clojure_compat_report.sh

clean:
	rm ./lg

lint: install-golangci-lint
	golangci-lint run 

install-golangci-lint:
	which golangci-lint || GO111MODULE=off go get -u github.com/golangci/golangci-lint/cmd/golangci-lint
