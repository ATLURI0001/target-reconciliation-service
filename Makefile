SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c
GO ?= go
GOFMT ?= gofmt
.PHONY: doctor init build test race lint integration up down smoke helm kind-up kind-down demo clean metrics-up metrics-down envtest lab-up lab-down lab-demo

doctor:
	./scripts/doctor.sh
init:
	./scripts/init-local.sh
build:
	mkdir -p bin
	$(GO) build -trimpath -o bin/target-reconciliation-service ./cmd/target-reconciliation-service
	$(GO) build -trimpath -o bin/trsctl ./cmd/trsctl
test:
	$(GO) test ./...
race:
	$(GO) test -race ./...
lint:
	@format_errors="$$(find cmd internal controllers tests -name '*.go' -print0 | xargs -0 $(GOFMT) -l)"; if [[ -n "$$format_errors" ]]; then printf 'Format these files with gofmt:\n%s\n' "$$format_errors" >&2; exit 1; fi
	$(GO) vet ./...
integration:
	@test -n "$$TEST_DATABASE_URL" || { echo 'Set TEST_DATABASE_URL to an isolated PostgreSQL database; integration tests may create schema.' >&2; exit 1; }
	$(GO) test -count=1 -v ./internal/store ./tests
up: init
	docker compose up --build -d
	@echo 'API is bound to http://127.0.0.1:8080. Kubernetes integration is disabled in Compose; see docs/local-lab.md.'
down:
	docker compose down
smoke:
	curl --fail --silent --show-error http://127.0.0.1:8080/healthz
	curl --fail --silent --show-error http://127.0.0.1:8080/readyz
helm:
	cmp config/crd/reliabilitytargets.yaml deploy/helm/target-reconciliation-service/crds/reliabilitytargets.yaml
	helm lint deploy/helm/target-reconciliation-service
	helm template target-reconciliation-service deploy/helm/target-reconciliation-service --include-crds > /tmp/target-reconciliation-service-rendered.yaml
kind-up:
	kind create cluster --name target-reconciliation-service-lab --config deploy/kind/config.yaml
kind-down:
	kind delete cluster --name target-reconciliation-service-lab
demo:
	./scripts/demo.sh
clean:
	rm -rf bin

metrics-up: init
	docker compose --profile metrics up --build -d
metrics-down:
	docker compose --profile metrics down
envtest:
	@set -e; assets="$$($(GO) run sigs.k8s.io/controller-runtime/tools/setup-envtest@v0.0.0-20260305142021-f9589b9f2b9d use -p path 1.35.0)"; KUBEBUILDER_ASSETS="$$assets" $(GO) test -count=1 -v ./controllers -run TestEnvtestAtomicPatch

lab-up:
	./scripts/lab-up.sh
lab-demo:
	./scripts/lab-demo.sh
lab-down:
	./scripts/lab-down.sh
