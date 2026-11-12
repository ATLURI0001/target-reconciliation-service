#!/usr/bin/env bash
# Pinned Linux/amd64 tools for disposable GitHub-hosted runners; no sudo or curl|sh.
set -euo pipefail
[[ "$(uname -s)" == Linux && "$(uname -m)" == x86_64 ]] || { echo 'This CI installer supports Linux/amd64 only. Install the documented tools natively on other platforms.' >&2; exit 1; }
cd "$(dirname "$0")/.."
mkdir -p .tools/bin
workspace=$(mktemp -d)
trap 'rm -rf "$workspace"' EXIT
fetch() {
  local url="$1" output="$2" checksum="$3"
  curl --fail --location --silent --show-error --retry 3 --connect-timeout 15 --max-time 180 "$url" --output "$output"
  printf '%s  %s\n' "$checksum" "$output" | sha256sum --check --status
}
fetch https://get.helm.sh/helm-v3.17.3-linux-amd64.tar.gz "$workspace/helm.tar.gz" ee88b3c851ae6466a3de507f7be73fe94d54cbf2987cbaa3d1a3832ea331f2cd
tar -xzf "$workspace/helm.tar.gz" -C "$workspace" linux-amd64/helm
install -m 0755 "$workspace/linux-amd64/helm" .tools/bin/helm
fetch https://github.com/kubernetes-sigs/kind/releases/download/v0.31.0/kind-linux-amd64 "$workspace/kind" eb244cbafcc157dff60cf68693c14c9a75c4e6e6fedaf9cd71c58117cb93e3fa
install -m 0755 "$workspace/kind" .tools/bin/kind
# kubectl checksum is pinned below after retrieval from the official release endpoint.
fetch https://dl.k8s.io/release/v1.35.0/bin/linux/amd64/kubectl "$workspace/kubectl" a2e984a18a0c063279d692533031c1eff93a262afcc0afdc517375432d060989
install -m 0755 "$workspace/kubectl" .tools/bin/kubectl
if [[ -n "${GITHUB_PATH:-}" ]]; then printf '%s\n' "$PWD/.tools/bin" >> "$GITHUB_PATH"; fi
printf '%s\n' 'Installed verified Helm 3.17.3, Kind 0.31.0, and kubectl 1.35.0 in .tools/bin.'
