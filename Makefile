COVERAGE_MIN ?= 70
DIFF_COVERAGE_MIN ?= 80
GO ?= go

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
	@$(GO) test -race ./...

bdd:
	@$(GO) test ./app -run TestFeatures -count=1 -v

coverage:
	@COVERAGE_MIN=$(COVERAGE_MIN) sh scripts/check-coverage.sh

diff-coverage:
	@p=$$(mktemp); $(GO) test -coverprofile=$$p ./... >/dev/null && $(GO) run ./scripts/diffcover --profile $$p --min $(DIFF_COVERAGE_MIN); s=$$?; rm -f $$p; exit $$s

vuln:
	@if command -v govulncheck >/dev/null 2>&1; then govulncheck ./...; else echo "govulncheck is required: go install golang.org/x/vuln/cmd/govulncheck@latest"; exit 1; fi

validate-skills:
	@sh scripts/validate-skills.sh

run:
	@$(GO) run ./app

gate-build:
	@mkdir -p bin && $(GO) build -o bin/gate ./gate
