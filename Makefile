TOP := $(dir $(abspath $(firstword $(MAKEFILE_LIST))))

HOST_ARCH ?= $(shell uname -m)
UPSTREAM_SOURCE_FALLBACK ?= false

VERSION := $(shell cat $(TOP)VERSION)
SHORT_SHA := $(shell git rev-parse --short=8 HEAD)

SDK_TAG := bottlerocket/sdk:$(VERSION)-$(SHORT_SHA)-$(HOST_ARCH)

all: sdk

sdk:
	@DOCKER_BUILDKIT=1 docker build . \
		--tag $(SDK_TAG) \
		--target sdk-golden \
		--build-arg HOST_ARCH=$(HOST_ARCH) \
		--build-arg UPSTREAM_SOURCE_FALLBACK=$(UPSTREAM_SOURCE_FALLBACK)

publish:
	@test $${REGISTRY?not set!}
	@test $${REPOSITORY?not set!}
	$(TOP)publish-sdk --registry=$(REGISTRY) --repository=$(REPOSITORY) --tag=$(VERSION) --short-sha=$(SHORT_SHA)

.PHONY: all sdk publish
