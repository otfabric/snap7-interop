# The snap7-interop-otfabric image, from the repository root:
#   make -f own/go-s7comm/Makefile image smoke interop
#
# Apart from the top-level Makefile on purpose: this image is not in the
# harness release and not built by `make images`.

IMAGE ?= ghcr.io/otfabric/snap7-interop-otfabric:dev
# The harness release of the reference images to test against.
VERSION ?= $(shell git tag -l 'v[0-9]*.[0-9]*.[0-9]*' --sort=-v:refname | head -1)
REFERENCES ?= native python
BUILD_PLATFORM ?= linux/$(shell uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
BASE_REGISTRY ?= docker.io/library

# The latest go-s7comm release, as the workflow builds it.
UPSTREAM_VERSION ?= $(shell git ls-remote --tags --refs https://github.com/otfabric/go-s7comm.git 'v[0-9]*' | sed 's,.*refs/tags/,,' | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$$' | sort -V | tail -1)
UPSTREAM_SHA ?= $(shell sha=$$(git ls-remote --tags https://github.com/otfabric/go-s7comm.git 'refs/tags/$(UPSTREAM_VERSION)^{}' | cut -f1); [ -n "$$sha" ] || sha=$$(git ls-remote --tags https://github.com/otfabric/go-s7comm.git 'refs/tags/$(UPSTREAM_VERSION)' | cut -f1); echo $$sha)

.PHONY: image smoke interop

image:
	docker build --platform=$(BUILD_PLATFORM) --build-arg BASE_REGISTRY=$(BASE_REGISTRY) --build-arg UPSTREAM_VERSION=$(UPSTREAM_VERSION) --build-arg UPSTREAM_SHA=$(UPSTREAM_SHA) --build-arg ADAPTER_VERSION=$(UPSTREAM_VERSION) --tag $(IMAGE) --file own/go-s7comm/Dockerfile .

export VERSION BUILD_PLATFORM
export ADAPTER_IMAGE_otfabric = $(IMAGE)

# The container-contract smoke test (scripts/smoke.sh).
smoke:
	$(MAKE) probe-bin
	HOST_PORT=3102 scripts/smoke.sh otfabric

# Every case of tests/cases.tsv against each reference image of $(VERSION)
# beside this image (scripts/interop.sh); known differences in tests/known-bugs.tsv.
interop:
	$(MAKE) probe-bin
	for r in $(REFERENCES); do REPORT_DIR=reports/own/$$r scripts/interop.sh $$r otfabric || exit 1; done
