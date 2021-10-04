ARCH ?= $(shell uname -m)
HOST_ARCH ?= $(shell uname -m)

VERSION := v0.23.0

SDK_TAG := bottlerocket/sdk-$(ARCH):$(VERSION)-$(HOST_ARCH)
TOOLCHAIN_TAG := bottlerocket/toolchain-$(ARCH):$(VERSION)-$(HOST_ARCH)

all: sdk toolchain

sdk:
	@DOCKER_BUILDKIT=1 docker build . \
		--tag $(SDK_TAG) \
		--target sdk-final \
		--squash \
		--build-arg ARCH=$(ARCH) \
		--build-arg HOST_ARCH=$(HOST_ARCH)

toolchain:
	@DOCKER_BUILDKIT=1 docker build . \
		--tag $(TOOLCHAIN_TAG) \
		--target toolchain-final \
		--squash \
		--build-arg ARCH=$(ARCH) \
		--build-arg HOST_ARCH=$(HOST_ARCH)

.PHONY: all sdk toolchain
