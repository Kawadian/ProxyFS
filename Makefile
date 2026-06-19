.PHONY: dev build test test-unit test-integration test-e2e lint fmt compose-up compose-down smoke security sbom clean embed-frontend

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo local)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GO      ?= go
NPM     ?= npm
DOCKER  ?= docker
COMPOSE ?= docker compose
BIN     ?= bin/lxcfh
IMAGE   ?= lxcfh/hub:latest
SAMBA_IMAGE ?= lxcfh/samba:latest

export LXCFH_VERSION=$(VERSION)
export LXCFH_COMMIT=$(COMMIT)
export LXCFH_BUILD_DATE=$(DATE)

dev: embed-frontend
	$(GO) run ./cmd/lxcfh

embed-frontend:
	@chmod +x scripts/embed-frontend.sh
	@./scripts/embed-frontend.sh

build: embed-frontend
	@mkdir -p bin
	CGO_ENABLED=1 $(GO) build -trimpath -ldflags "-s -w -X github.com/lxcfh/lxcfh/internal/version.Version=$(VERSION)" -o $(BIN) ./cmd/lxcfh
	CGO_ENABLED=1 $(GO) build -trimpath -ldflags "-s -w" -o bin/lxcfh-fuse ./cmd/fusemount

test: test-unit test-integration

test-unit:
	@mkdir -p coverage
	$(GO) test ./internal/config/... ./internal/migrate/... ./internal/crypto/... ./internal/auth/... ./internal/health/... ./internal/vfs/... -count=1 -short -race -coverprofile=coverage/unit.out

test-integration:
	$(GO) test ./test/integration/... -count=1 -tags=integration -timeout=5m

test-e2e:
	$(COMPOSE) --profile test --profile smb build
	./scripts/smoke.sh

lint:
	@command -v golangci-lint >/dev/null 2>&1 || { echo "install golangci-lint: https://golangci-lint.run"; exit 1; }
	golangci-lint run ./...
	@cd frontend && $(NPM) run build

fmt:
	gofmt -w .
	@test -z "$$(gofmt -l .)" || (echo "gofmt needed"; exit 1)

compose-up:
	@test -f secrets/dev/master.key || cp secrets/dev/master.key.example secrets/dev/master.key
	$(COMPOSE) --env-file .env.example up -d --build hub

compose-up-full:
	@test -f secrets/dev/master.key || cp secrets/dev/master.key.example secrets/dev/master.key
	SMB_ENABLED=true $(COMPOSE) --env-file .env.example --profile smb up -d --build

compose-down:
	$(COMPOSE) --profile test --profile smb down -v --remove-orphans

smoke:
	@chmod +x scripts/smoke.sh
	@./scripts/smoke.sh

security:
	@command -v trivy >/dev/null 2>&1 || { echo "install trivy: https://aquasecurity.github.io/trivy"; exit 1; }
	$(DOCKER) build -t $(IMAGE) --build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) --build-arg BUILD_DATE=$(DATE) .
	$(DOCKER) build -f Dockerfile.samba -t $(SAMBA_IMAGE) .
	trivy image --severity HIGH,CRITICAL --exit-code 1 $(IMAGE)
	trivy image --severity HIGH,CRITICAL --exit-code 1 $(SAMBA_IMAGE)

sbom:
	@mkdir -p .sbom
	@command -v syft >/dev/null 2>&1 || { echo "install syft: https://github.com/anchore/syft"; exit 1; }
	$(DOCKER) build -t $(IMAGE) .
	syft $(IMAGE) -o spdx-json > .sbom/hub.spdx.json
	$(DOCKER) build -f Dockerfile.samba -t $(SAMBA_IMAGE) .
	syft $(SAMBA_IMAGE) -o spdx-json > .sbom/samba.spdx.json

clean:
	rm -rf bin cmd/lxcfh/static web/dist coverage .sbom
	$(GO) clean -cache -testcache 2>/dev/null || true
