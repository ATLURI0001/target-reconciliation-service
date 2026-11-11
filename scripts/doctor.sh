#!/usr/bin/env bash
set -uo pipefail
missing=0
check() {
  local name="$1" help="$2"
  if command -v "$name" >/dev/null 2>&1; then printf 'FOUND %-10s %s\n' "$name" "$(command -v "$name")"
  else printf 'MISSING %-10s %s\n' "$name" "$help"; missing=1; fi
}
check git 'https://git-scm.com/downloads'
check go 'Go 1.27.4 or newer: https://go.dev/doc/install (GNU Go is a different program)'
if command -v go >/dev/null 2>&1; then
  if ! go version; then printf '%s\n' 'Go toolchain is not usable; GNU Go is not the Go compiler.'; missing=1; fi
fi
check docker 'Install Docker Engine or Docker Desktop with Compose v2.'
if command -v docker >/dev/null 2>&1; then
  docker compose version || missing=1
  docker info >/dev/null 2>&1 || { printf '%s\n' 'Docker daemon is unavailable; start Docker.'; missing=1; }
fi
check kubectl 'https://kubernetes.io/docs/tasks/tools/'
check kind 'https://kind.sigs.k8s.io/docs/user/quick-start/'
check helm 'Helm 3: https://helm.sh/docs/intro/install/'
check python3 'Install Python 3.'
printf '%s\n' 'Linux/macOS: native tools. Windows: run these scripts inside WSL2 with Docker integration.'
exit "$missing"
