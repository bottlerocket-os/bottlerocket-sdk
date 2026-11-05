TOP := $(dir $(abspath $(firstword $(MAKEFILE_LIST))))

HOST_ARCH ?= $(shell uname -m)
DOCKER_ARCH ?= $(lastword $(subst :, ,$(filter $(HOST_ARCH):%,x86_64:amd64 aarch64:arm64)))
DOCKER_ALT_ARCH ?= $(lastword $(subst :, ,$(filter $(HOST_ARCH):%,x86_64:arm64 aarch64:amd64)))
UPSTREAM_SOURCE_FALLBACK ?= false

VERSION := $(shell cat $(TOP)VERSION)
SHORT_SHA := $(shell git rev-parse --short=8 HEAD)

REGISTRY ?=
REPOSITORY ?= bottlerocket-sdk
IMAGE_NAME ?= $(REPOSITORY):$(VERSION)-$(SHORT_SHA)-$(DOCKER_ARCH)
IMAGE_ALT_NAME ?= $(REPOSITORY):$(VERSION)-$(SHORT_SHA)-$(DOCKER_ALT_ARCH)
MANIFEST ?= $(REPOSITORY):$(VERSION)

BUILDX_BUILDER ?= sdk-builder

BUILDX_BUILD_ARGS = $\
	--build-arg HOST_ARCH=$(HOST_ARCH) $\
	--build-arg UPSTREAM_SOURCE_FALLBACK=$(UPSTREAM_SOURCE_FALLBACK) $\
	--target sdk-golden $\
	--provenance=false $\
	--sbom=false $\
	--builder $(BUILDX_BUILDER)

BUILDX_LOAD_ARGS = $\
	--tag $(IMAGE_NAME) \
	--load

BUILDX_PUSH_ARGS = $\
	--output $\
	type=registry,name=$(REGISTRY)/$(IMAGE_NAME),$\
	compression=zstd,compression-level=22,force-compression=true,$\
	oci-mediatypes=true,platform=linux/$(DOCKER_ARCH)

all: build

builder:
	@docker buildx create \
		--name $(BUILDX_BUILDER) \
		--driver docker-container \
		--driver-opt env.BUILDKIT_STEP_LOG_MAX_SIZE=-1 \
		--driver-opt env.BUILDKIT_STEP_LOG_MAX_SPEED=-1 \
		--node $(BUILDX_BUILDER)0

build: builder
	@docker buildx build . \
		$(BUILDX_BUILD_ARGS) \
		$(BUILDX_LOAD_ARGS)

build-push: builder
	@test $${REGISTRY?not set!}
	@docker buildx build . \
		$(BUILDX_BUILD_ARGS) \
		$(BUILDX_PUSH_ARGS)

publish: build-push
	@if docker buildx imagetools inspect $(REGISTRY)/$(IMAGE_ALT_NAME) >/dev/null 2>&1 ; then \
		docker buildx imagetools create \
			--tag $(REGISTRY)/$(MANIFEST) \
			$(REGISTRY)/$(IMAGE_NAME) \
			$(REGISTRY)/$(IMAGE_ALT_NAME) ; \
	else \
		docker buildx imagetools create \
			--tag $(REGISTRY)/$(MANIFEST) \
			$(REGISTRY)/$(IMAGE_NAME) ; \
	fi

.PHONY: all builder build build-push publish
