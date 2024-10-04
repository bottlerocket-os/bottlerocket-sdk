TOP := $(dir $(abspath $(firstword $(MAKEFILE_LIST))))

.PHONY: deny
deny:
	cargo deny --no-default-features check licenses bans sources

.PHONY: clippy
clippy:
	cargo clippy --locked -- -D warnings --no-deps

.PHONY: fmt
fmt:
	cargo fmt --check

.PHONY: test
test:
	cargo test --locked

.PHONY: check
check: fmt clippy deny test

.PHONY: build
build: check
	cargo build --locked
