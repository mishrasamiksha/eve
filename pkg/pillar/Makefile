# Copyright (c) 2018 Zededa, Inc.
# SPDX-License-Identifier: Apache-2.0

# Goals
# 1. Build go provision binaries for arm64 and amd64
# 2. Build on Linux as well on Mac

ARCH         ?= amd64
#ARCH        ?= arm64
DISTDIR      := dist/$(ARCH)
BUILD_VERSION ?=

DOCKER_ARGS=
DOCKER_TAG=lfedge/eve-pillar:local
ifneq ($(GOARCH),)
DOCKER_ARGS:=--build-arg GOARCH=$(GOARCH)
DOCKER_TAG:=$(DOCKER_TAG)-$(GOARCH)
endif

APPS = zedbox
APPS1 = $(notdir $(wildcard cmd/*))

# find all GOFILES
GOFILES = $(shell find . -path ./vendor -prune -o -name '*go' -print)

.PHONY: all clean build test build-docker build-docker-git shell

all: build

$(DISTDIR):
	mkdir -p $(DISTDIR)

build: $(APPS) $(APPS1)

TAGS=
ifeq ($(RSTATS),y)
	TAGS:=-tags rstats
endif

LDFLAGS=-X=main.Version=$(BUILD_VERSION)
ifneq ($(DEV),y)
	LDFLAGS+=-s -w
endif
ifeq ($(RSTATS),y)
	LDFLAGS+=-X=github.com/lf-edge/eve/pkg/pillar/rstats.Endpoint=$(RSTATS_ENDPOINT)
	LDFLAGS+=-X=github.com/lf-edge/eve/pkg/pillar/rstats.Tag=$(RSTATS_TAG)
endif
LDFLAGS:=-ldflags "$(LDFLAGS)"

GCFLAGS=
ifeq ($(DEV),y)
	GCFLAGS:=-gcflags=all="-N -l"
endif

$(APPS): $(DISTDIR)/$(APPS)
$(DISTDIR)/$(APPS): $(DISTDIR)
	@echo "Building $@"
	GO111MODULE=on GOOS=linux go build -mod=vendor $(TAGS) $(GCFLAGS) $(LDFLAGS) -o $@ ./$(@F)

$(APPS1): $(DISTDIR)
	@echo $@
	@rm -f $(DISTDIR)/$@
	@ln -s $(APPS) $(DISTDIR)/$@

shell:
	make -C ../.. shell

build-docker-$(APPS): $(DISTDIR)
	docker build -f Dockerfile.in --target=build -t $(APPS)-builder .
	# all of this goes away when we switch to full buildkit-based docker builds
	docker container create --name $(APPS)-extract $(APPS)-builder
	docker container cp $(APPS)-extract:/dist/$(APPS) $(DISTDIR)/$(APPS)
	docker container rm -f $(APPS)-extract


build-docker:
	docker build $(DOCKER_ARGS) -t $(DOCKER_TAG) .

build-docker-git:
	git archive HEAD | docker build $(DOCKER_ARGS) -t $(DOCKER_TAG) -

test:
	go test -mod=vendor ./...

clean:
	@rm -rf $(DISTDIR)

fmt:
	@gofmt -w -s $(GOFILES)

fmt-check:
	@gofmt -l $(GOFILES)

fmt-check-details:
	@gofmt -d $(GOFILES)

vet:
	go vet ./...
