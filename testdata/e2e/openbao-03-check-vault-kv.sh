#!/bin/bash

set -e

# Variables
NAMESPACE="openbao"

PODS=($(kubectl get pods -n $NAMESPACE -l app.kubernetes.io/name=openbao -o jsonpath="{.items[*].metadata.name}"))

echo "Waiting for openbao pods to be ready..."
for ((i=1; i<${#PODS[@]}; i++)); do
  FOLLOWER="${PODS[$i]}"
  kubectl wait --for=condition=ready pod $FOLLOWER -n $NAMESPACE --timeout=30s
done

ROOT_TOKEN=$(cat openbao-init.json | jq -r '.root_token')

# Login and verify
kubectl exec -n $NAMESPACE "${PODS[0]}" -- bao login "$ROOT_TOKEN"
kubectl exec -n $NAMESPACE "${PODS[0]}" -- bao operator raft list-peers

# Verify unseal keys were read from OpenBao KV
echo "Verifying unseal keys were read from OpenBao KV..."
POD_NAME=$(kubectl get pod -n $NAMESPACE -l app.kubernetes.io/instance=vault-unsealer -o name)
echo "Waiting for unsealer to read keys from OpenBao KV..."
MAX_RETRIES=30
RETRY_COUNT=0
while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
  if kubectl logs -n $NAMESPACE $POD_NAME 2>&1 | grep -q "successfully read unseal keys from vault"; then
    echo "SUCCESS: Unsealer read unseal keys from OpenBao KV"
    kubectl logs -n $NAMESPACE $POD_NAME 2>&1 | grep "successfully read unseal keys from vault"
    break
  fi
  RETRY_COUNT=$((RETRY_COUNT + 1))
  echo "Waiting for unsealer to process... (${RETRY_COUNT}/${MAX_RETRIES})"
  sleep 2
done

if [ $RETRY_COUNT -eq $MAX_RETRIES ]; then
  echo "FAILURE: Unsealer did not read unseal keys from OpenBao KV within timeout"
  echo "Unsealer logs:"
  kubectl logs -n $NAMESPACE $POD_NAME
  exit 1
fi
