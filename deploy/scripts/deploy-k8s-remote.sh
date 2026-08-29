#!/usr/bin/env bash
# Copyright 2026 Zyvor AI Labs
# SPDX-License-Identifier: Apache-2.0
# Deploy relay-pubsub + relay-edge to a remote k3s/Kubernetes host over SSH.
# Both pods use built-in self-signed HTTPS (emptyDir TLS volume, generated on first start).
#
# Usage:
#   ./deploy/scripts/deploy-k8s-remote.sh <host> [user]
#   RELAY_AUTH_TOKEN=… ./deploy/scripts/deploy-k8s-remote.sh <HOST> [USER]
#
# Requires on remote: kubectl, helm, podman (or docker), k3s/kubernetes cluster.
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: $0 <host> [user]" >&2
  exit 1
fi
HOST="$1"
USER="${2:-sus}"
EDGE_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
PUBSUB_ROOT="$(cd "${EDGE_ROOT}/../relay-pubsub" && pwd)"
NS_PUBSUB="${NS_PUBSUB:-relay-pubsub}"
NS_EDGE="${NS_EDGE:-relay-edge}"
IMAGE_TAG="${IMAGE_TAG:-latest}"
RELAY_BASE_URL="${RELAY_BASE_URL:-https://${HOST}:8443}"
BUILDER="${BUILDER:-podman}"
if ! command -v "$BUILDER" >/dev/null 2>&1 && command -v docker >/dev/null 2>&1; then
  BUILDER=docker
fi

TOK="${RELAY_AUTH_TOKEN:-}"
if [[ -z "$TOK" && -f /tmp/lab-relay.jwt ]]; then
  TOK="$(cat /tmp/lab-relay.jwt)"
fi
if [[ -z "$TOK" ]]; then
  echo "warning: RELAY_AUTH_TOKEN unset — pods will fail relay-events publish until secret is set" >&2
  TOK="replace-me"
fi

echo "== zyvor edge stack → k8s on ${USER}@${HOST} =="

# Sync sources
REMOTE_STAGING="/home/${USER}/.deployments/k8s-edge-stack"
ssh -o BatchMode=yes "${USER}@${HOST}" "mkdir -p ${REMOTE_STAGING}"
rsync -az --delete \
  --exclude target --exclude .git --exclude data \
  "${PUBSUB_ROOT}/" "${USER}@${HOST}:${REMOTE_STAGING}/relay-pubsub/"
rsync -az --delete \
  --exclude bin --exclude .git --exclude data \
  "${EDGE_ROOT}/" "${USER}@${HOST}:${REMOTE_STAGING}/relay-edge/"

ssh -o BatchMode=yes "${USER}@${HOST}" bash -s <<REMOTE
set -euo pipefail
STAGING="${REMOTE_STAGING}"
TAG="${IMAGE_TAG}"
NS_P="${NS_PUBSUB}"
NS_E="${NS_EDGE}"
TOK='${TOK}'
RELAY_URL='${RELAY_BASE_URL}'
BUILDER='${BUILDER}'

build_import() {
  local name=\$1 dir=\$2
  local repo="docker.io/library/\${name}"
  echo "Building \${repo}:\${TAG} with \${BUILDER}..."
  "\${BUILDER}" build -t "\${repo}:\${TAG}" "\${dir}"
  local tar=\$(mktemp -t "\${name}.XXXXXX.tar")
  "\${BUILDER}" save "\${repo}:\${TAG}" -o "\${tar}"
  sudo k3s ctr -n k8s.io images import "\${tar}"
  rm -f "\${tar}"
}

build_import relay-pubsub "\${STAGING}/relay-pubsub"
build_import relay-edge "\${STAGING}/relay-edge"

kubectl create namespace "\${NS_P}" --dry-run=client -o yaml | kubectl apply -f -
kubectl create namespace "\${NS_E}" --dry-run=client -o yaml | kubectl apply -f -

kubectl -n "\${NS_P}" create secret generic relay-pubsub-secrets \
  --from-literal=relay-auth-token="\${TOK}" \
  --dry-run=client -o yaml | kubectl apply -f -
kubectl -n "\${NS_E}" create secret generic relay-edge-secrets \
  --from-literal=relay-auth-token="\${TOK}" \
  --dry-run=client -o yaml | kubectl apply -f -

helm upgrade --install relay-pubsub "\${STAGING}/relay-pubsub/deploy/helm/relay-pubsub" \
  -n "\${NS_P}" \
  --set image.repository=docker.io/library/relay-pubsub \
  --set image.tag="\${TAG}" \
  --set image.pullPolicy=IfNotPresent \
  --set relay.backend=relay-events \
  --set relay.baseUrl="\${RELAY_URL}" \
  --set relay.tlsInsecure=1 \
  --set replicaCount=1

helm upgrade --install relay-edge "\${STAGING}/relay-edge/deploy/helm/relay-edge" \
  -n "\${NS_E}" \
  --set image.repository=docker.io/library/relay-edge \
  --set image.tag="\${TAG}" \
  --set image.pullPolicy=IfNotPresent \
  --set edge.relayBaseUrl="\${RELAY_URL}" \
  --set edge.gatewayBaseUrl="https://relay-pubsub.\${NS_P}.svc:8080" \
  --set persistence.enabled=false

kubectl -n "\${NS_P}" rollout status deployment/relay-pubsub --timeout=180s
kubectl -n "\${NS_E}" rollout status deployment/relay-edge --timeout=180s
kubectl -n "\${NS_P}" get pods -o wide
kubectl -n "\${NS_E}" get pods -o wide

echo "Running in-cluster e2e..."
bash "\${STAGING}/relay-edge/deploy/scripts/k8s-e2e.sh"
REMOTE

echo "OK: stack deployed on ${HOST}"
