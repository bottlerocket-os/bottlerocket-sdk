ARCH ?= $(shell uname -m)
HOST_ARCH ?= $(shell uname -m)

VERSION := v0.13.0

SDK_TAG := bottlerocket/sdk-$(ARCH):$(VERSION)-$(HOST_ARCH)
SDK_ARCHIVE := bottlerocket-sdk-$(ARCH)-$(VERSION).$(HOST_ARCH).tar.gz

TOOLCHAIN_TAG := bottlerocket/toolchain-$(ARCH):$(VERSION)-$(HOST_ARCH)
TOOLCHAIN_ARCHIVE := bottlerocket-toolchain-$(ARCH)-$(VERSION).$(HOST_ARCH).tar.xz

all: $(SDK_ARCHIVE) $(TOOLCHAIN_ARCHIVE)

$(SDK_ARCHIVE) :
	@DOCKER_BUILDKIT=1 docker build . \
		--tag $(SDK_TAG) \
		--target sdk-final \
		--squash \
		--build-arg ARCH=$(ARCH)
	@docker image save $(SDK_TAG) | gzip --fast > $(@)

$(TOOLCHAIN_ARCHIVE) :
	@DOCKER_BUILDKIT=1 docker build . \
		--tag $(TOOLCHAIN_TAG) \
		--target toolchain-final \
		--squash \
		--build-arg ARCH=$(ARCH)
	@docker run --rm --entrypoint cat $(TOOLCHAIN_TAG) /tmp/toolchain.tar.xz > $(@)

upload : $(SDK_ARCHIVE) $(TOOLCHAIN_ARCHIVE)
	@aws s3 cp $(SDK_ARCHIVE) s3://thar-upstream-lookaside-cache/$(SDK_TAG).tar.gz
	@aws s3 cp $(TOOLCHAIN_ARCHIVE) s3://thar-upstream-lookaside-cache/$(TOOLCHAIN_TAG).tar.xz

clean:
	@rm -f *.tar.gz *.tar.xz

.PHONY: all upload clean
