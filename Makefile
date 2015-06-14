GO ?= go
GOPATH := $(CURDIR)/_vendor:$(GOPATH)

all: build

build:
	@mkdir -p bin
	$(GO) build -v -o bin/gotrust

deps:
	$(GO) get -d
