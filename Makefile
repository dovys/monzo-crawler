SHELL = /bin/bash
MAKEFLAGS+=-s
.DEFAULT_GOAL:=install
BINARY=crawler
CMD_PATH=cmd/main.go

install:
	which dep > /dev/null || { \
		echo "Installing github.com/golang/dep"; \
		go get -v -u github.com/golang/dep/...; \
	}
	echo "Installing dependencies"
	dep ensure -v

build:
	go build -o $(BINARY) $(CMD_PATH)

test:
	go test -race -v -cover

.PHONY: install build test
