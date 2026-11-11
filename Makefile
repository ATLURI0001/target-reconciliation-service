SHELL := /bin/bash
GO ?= go
.PHONY: doctor init build test race lint integration up down smoke helm kind-up kind-down demo clean metrics-up metrics-down envtest

doctor:
	./scripts/doctor.sh
init:
	./scripts/init-local.sh
build:
	mkdir -p bin
	$(GO) build -trimpath -o bin/resilienceplane ./cmd/resilienceplane
	$(GO) build -trimpath -o bin/rpctl ./cmd/rpctl
test:
	$(GO) test ./...
race:
	$(GO) test -race ./...
lint:
	test -z "$$($(GO) fmt ./...)"
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
	cmp config/crd/reliabilitytargets.yaml deploy/helm/resilienceplane/crds/reliabilitytargets.yaml
	helm lint deploy/helm/resilienceplane
	helm template resilienceplane deploy/helm/resilienceplane --include-crds > /tmp/resilienceplane-rendered.yaml
kind-up:
	kind create cluster --name resilienceplane-lab --config deploy/kind/config.yaml
kind-down:
	kind delete cluster --name resilienceplane-lab
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
