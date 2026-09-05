PWD := $(shell pwd)
GOPATH := $(shell go env GOPATH)
LDFLAGS := $(shell go run buildscripts/gen-ldflags.go)

TARGET_GOARCH ?= $(shell go env GOARCH)
TARGET_GOOS ?= $(shell go env GOOS)

VERSION ?= $(shell git describe --tags)
TAG ?= "minio/mc:$(VERSION)"

GOLANGCI = $(GOPATH)/bin/golangci-lint

all: build

getdeps:
	@mkdir -p ${GOPATH}/bin
	@echo "Installing tools" && go install tool

verifiers: getdeps vet lint

new-release:
	@if [ "$$(git rev-parse --abbrev-ref HEAD)" != "master" ]; then echo "Error: new-release must be run on the master branch"; exit 1; fi; \
	RELEASE_TIME=$$(date -u +"%Y-%m-%dT%H-%M-%SZ"); \
	echo "Creating release commit and tag for RELEASE.$${RELEASE_TIME}"; \
	git commit --allow-empty -m "chore: release $${RELEASE_TIME}"; \
	git tag "RELEASE.$${RELEASE_TIME}"; \
	git push origin HEAD; \
	git push origin "RELEASE.$${RELEASE_TIME}"

vet:
	@echo "Running $@"
	@GO111MODULE=on go vet github.com/minio/mc/...

lint-fix: getdeps ## runs golangci-lint suite of linters with automatic fixes
	@echo "Running $@ check"
	@$(GOLANGCI) run --build-tags kqueue --timeout=10m --config ./.golangci.yml --fix

lint: getdeps
	@echo "Running $@ check"
	@$(GOLANGCI) run --build-tags kqueue --timeout=10m --config ./.golangci.yml

# Builds mc, runs the verifiers then runs the tests.
check: test
test: verifiers build
	@echo "Running unit tests"
	@GO111MODULE=on CGO_ENABLED=0 go test -tags kqueue ./... 1>/dev/null
	@echo "Running functional tests"
	@GO111MODULE=on MC_TEST_RUN_FULL_SUITE=true go test -race -v --timeout 20m ./... -run Test_FullSuite

INTEGRATION_BACKENDS ?= minio-legacy rustfs garage seaweedfs versitygw aistor
MC_INTEGRATION_PORT ?= 9000

# Runs the full integration suite against each supported local object store.
integration: build
	@set -e; \
	for backend in $(INTEGRATION_BACKENDS); do \
		echo "Running integration tests against $$backend"; \
		MC_INTEGRATION_PORT="$(MC_INTEGRATION_PORT)" \
			.github/scripts/run-integration-tests.sh "$$backend" "$(PWD)/mc"; \
	done

# Runs the integration suite against one backend, for example:
# make integration-garage MC_INTEGRATION_PORT=19000
integration-%: build
	@MC_INTEGRATION_PORT="$(MC_INTEGRATION_PORT)" \
		.github/scripts/run-integration-tests.sh "$*" "$(PWD)/mc"

test-race: verifiers build
	@echo "Running unit tests under -race"
	@GO111MODULE=on go test -race -v --timeout 20m ./... 1>/dev/null

# Verify mc binary
verify:
	@echo "Verifying build with race"
	@GO111MODULE=on CGO_ENABLED=1 go build -race -tags kqueue -trimpath --ldflags "$(LDFLAGS)" -o $(PWD)/mc 1>/dev/null
	@echo "Running functional tests"
	@GO111MODULE=on MC_TEST_RUN_FULL_SUITE=true go test -race -v --timeout 20m ./... -run Test_FullSuite

# Builds mc locally.
build:
	@echo "Building mc binary to './mc'"
	@GO111MODULE=on GOOS=$(TARGET_GOOS) GOARCH=$(TARGET_GOARCH) CGO_ENABLED=0 go build -trimpath -tags kqueue --ldflags "$(LDFLAGS)" -o $(PWD)/mc

# Builds MinIO and installs it to $GOPATH/bin.
install: build
	@echo "Installing mc binary to '$(GOPATH)/bin/mc'"
	@mkdir -p $(GOPATH)/bin && cp -f $(PWD)/mc $(GOPATH)/bin/mc
	@echo "Installation successful. To learn more, try \"mc --help\"."

clean:
	@echo "Cleaning up all the generated files"
	@find . -name '*.test' | xargs rm -fv
	@find . -name '*~' | xargs rm -fv
	@rm -rvf mc
	@rm -rvf build
	@rm -rvf release
