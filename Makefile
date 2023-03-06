TOP := $(dir $(abspath $(firstword $(MAKEFILE_LIST))))

ARCH ?= $(shell uname -m)
HOST_ARCH ?= $(shell uname -m)

VERSION := $(shell cat $(TOP)VERSION)

SDK_TAG := bottlerocket/sdk-$(ARCH):$(VERSION)-$(HOST_ARCH)
TOOLCHAIN_TAG := bottlerocket/toolchain-$(ARCH):$(VERSION)-$(HOST_ARCH)

all: sdk toolchain

sdk:
	@DOCKER_BUILDKIT=1 docker build . \
		--tag $(SDK_TAG) \
		--target sdk-golden \
		--build-arg ARCH=$(ARCH) \
		--build-arg HOST_ARCH=$(HOST_ARCH)

toolchain:
	@DOCKER_BUILDKIT=1 docker build . \
		--tag $(TOOLCHAIN_TAG) \
		--target toolchain-golden \
		--build-arg ARCH=$(ARCH) \
		--build-arg HOST_ARCH=$(HOST_ARCH)

publish:
	@test $${REGISTRY?not set!}
	@test $${SDK_NAME?not set!}
	$(TOP)publish-sdk --registry=$(REGISTRY) --sdk-name=$(SDK_NAME) --version=$(VERSION)

.PHONY: all sdk toolchain publish
