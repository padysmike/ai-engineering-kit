COVERAGE_MIN ?= 70
DIFF_COVERAGE_MIN ?= 80
GO ?= go

# SKIP_GATE_TESTS=1 is for agent sandboxes that refuse the local git pushes the
# gate's tests make. The gate itself never passes it to its checks.
TEST_PACKAGES = ./...
ifeq ($(SKIP_GATE_TESTS),1)
TEST_PACKAGES = $(or $(shell $(GO) list ./... | grep -v '/gate$$'),$(error could not list the packages))
WARN_GATE_SKIPPED = @echo "warning: SKIP_GATE_TESTS=1: $@ leaves out gate/, so its tests and changed lines are not checked"
endif

.PHONY: check fmt-check vet lint test bdd coverage diff-coverage vuln validate-skills run gate-build

check: fmt-check vet lint test bdd coverage diff-coverage validate-skills ## everything the gate runs

fmt-check:
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

vet:
	@$(GO) vet ./...

lint:
	@if command -v golangci-lint >/dev/null 2>&1; then golangci-lint run ./...; \
	elif [ "$$REQUIRE_LINT" = "1" ]; then echo "golangci-lint is required but not installed"; exit 1; \
	else echo "warning: golangci-lint not installed, skipping lint"; fi

test:
	$(WARN_GATE_SKIPPED)
	@$(GO) test -race $(TEST_PACKAGES)

bdd:
	@$(GO) test ./app -run TestFeatures -count=1 -v

coverage:
	$(WARN_GATE_SKIPPED)
	@COVERAGE_MIN=$(COVERAGE_MIN) TEST_PACKAGES="$(TEST_PACKAGES)" sh scripts/check-coverage.sh

diff-coverage:
	$(WARN_GATE_SKIPPED)
	@p=$$(mktemp); $(GO) test -coverprofile=$$p $(TEST_PACKAGES) >/dev/null && $(GO) run ./scripts/diffcover --profile $$p --min $(DIFF_COVERAGE_MIN); s=$$?; rm -f $$p; exit $$s

vuln:
	@if command -v govulncheck >/dev/null 2>&1; then govulncheck ./...; else echo "govulncheck is required: go install golang.org/x/vuln/cmd/govulncheck@latest"; exit 1; fi

validate-skills:
	@sh scripts/validate-skills.sh

run:
	@$(GO) run ./app

gate-build:
	@mkdir -p bin && $(GO) build -o bin/gate ./gate
