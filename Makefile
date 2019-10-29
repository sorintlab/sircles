PROJDIR=$(dir $(realpath $(firstword $(MAKEFILE_LIST))))

# change to project dir so we can express all as relative paths
$(shell cd $(PROJDIR))

PROJ=sircles
ORG_PATH=github.com/sorintlab
REPO_PATH=$(ORG_PATH)/$(PROJ)

VERSION ?= $(shell scripts/git-version.sh)

PACKAGES := $(shell go list ./... | grep -v /vendor/)

WEBAPP_SRC := $(shell find web/src -type f)
SEMANTIC_SRC ?= $(shell find web/semantic/src -type f)

LD_FLAGS="-w -X $(REPO_PATH)/version.Version=$(VERSION)"

$(shell mkdir -p bin )
$(shell mkdir -p tools/bin )

export GOBIN=$(PROJDIR)/bin

SIRCLES_SRC = $(shell find . -name '*.go' | grep -v "webbundle/bindata.go")
SIRCLES_DEPS =
SIRCLES_TAGS =

SIRCLES_WEBBUNDLE_SRC = $(shell find . -name '*.go')
SIRCLES_WEBBUNDLE_DEPS = webbundle/bindata.go
SIRCLES_WEBBUNDLE_TAGS = webbundle

ifndef NOWEBBUNDLE
	SIRCLES_SRC = $(SIRCLES_WEBBUNDLE_SRC)
	SIRCLES_DEPS = $(SIRCLES_WEBBUNDLE_DEPS)
	SIRCLES_TAGS = $(SIRCLES_WEBBUNDLE_TAGS)
endif

.PHONY: all
all: build

.PHONY: build
build: bin/sircles

.PHONY: test
test: tools/bin/gocovmerge
	@scripts/test.sh

bin/sircles: $(SIRCLES_DEPS) $(SIRCLES_SRC)
	go install $(if $(SIRCLES_TAGS),-tags $(SIRCLES_TAGS)) -ldflags $(LD_FLAGS) $(REPO_PATH)/cmd/sircles

# build the binary inside a docker container. Now we use a glibc based golang image to match the fedora base image used in the Dockerfile of the demo but can be changed to use a musl libc based image like alpine
bin/sircles-dockerdemo: $(SIRCLES_WEBBUNDLE_DEPS) $(SIRCLES_WEBBUNDLE_SRC)
	docker run --rm -v "$(PROJDIR)":/go/src/$(REPO_PATH) -w /go/src/$(REPO_PATH) golang:1.11 go build -tags webbundle -ldflags $(LD_FLAGS) -o /go/src/${REPO_PATH}/bin/sircles-dockerdemo $(REPO_PATH)/cmd/sircles

.PHONY: dockerdemo
dockerdemo: bin/sircles-dockerdemo
	docker build . -t sirclesdemo

.PHONY: dist-web
dist-web: web/dist/app.js

web/node_modules: web/package.json
	cd web && npm install

web/semantic/dist/semantic.min.css: web/semantic.json web/node_modules/semantic-ui/package.json $(SEMANTIC_SRC)
	cd web/semantic && $$(npm bin)/gulp build

web/dist/app.js: web/node_modules web/semantic/dist/semantic.min.css $(WEBAPP_SRC)
	cd web && rm -rf dist && mkdir dist && npm run build

.PHONY: dist-web-bindata
dist-web-bindata: webbundle/bindata.go

# TODO(sgotti) vendor tools (won't use glide since it's not a dependency)
tools/bin/go-bindata:
	GOBIN=$(PROJDIR)/tools/bin go get github.com/go-bindata/go-bindata/go-bindata

tools/bin/gocovmerge:
	GOBIN=$(PROJDIR)/tools/bin go get github.com/wadey/gocovmerge

webbundle/bindata.go: tools/bin/go-bindata web/dist/app.js
	./tools/bin/go-bindata -o webbundle/bindata.go -tags webbundle -pkg webbundle -prefix 'web/dist/' -nocompress=true web/dist/...
