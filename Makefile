TOP := $(dir $(abspath $(firstword $(MAKEFILE_LIST))))

HOST_ARCH ?= $(shell uname -m)
DOCKER_ARCH ?= $(lastword $(subst :, ,$(filter $(HOST_ARCH):%,x86_64:amd64 aarch64:arm64)))
UPSTREAM_SOURCE_FALLBACK ?= false

VERSION := $(shell cat $(TOP)VERSION)
SHORT_SHA := $(shell git rev-parse --short=8 HEAD)

IMAGE_NAME ?= bottlerocket-sdk:$(VERSION)-$(SHORT_SHA)-$(DOCKER_ARCH)

all: sdk

sdk:
	@DOCKER_BUILDKIT=1 docker build . \
		--tag $(IMAGE_NAME) \
		--target sdk-golden \
		--build-arg HOST_ARCH=$(HOST_ARCH) \
		--build-arg UPSTREAM_SOURCE_FALLBACK=$(UPSTREAM_SOURCE_FALLBACK)

publish:
	@test $${REGISTRY?not set!}
	@test $${REPOSITORY?not set!}
	$(TOP)publish-sdk --registry=$(REGISTRY) --repository=$(REPOSITORY) --tag=$(VERSION) --short-sha=$(SHORT_SHA)

.PHONY: all sdk publish
