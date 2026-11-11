#!/bin/bash

set -e

# Variables
NAMESPACE="vault"

PODS=($(kubectl get pods -n $NAMESPACE -l app.kubernetes.io/name=vault -o jsonpath="{.items[*].metadata.name}"))

echo "Waiting for Vault pods to be ready..."
for ((i=1; i<${#PODS[@]}; i++)); do
  FOLLOWER="${PODS[$i]}"
  kubectl wait --for=condition=ready pod $FOLLOWER -n $NAMESPACE --timeout=30s
done

ROOT_TOKEN=$(cat vault-init.json | jq -r '.root_token')

# Login and verify
kubectl exec -n $NAMESPACE "${PODS[0]}" -- vault login "$ROOT_TOKEN"
kubectl exec -n $NAMESPACE "${PODS[0]}" -- vault operator raft list-peers

# Verify unseal keys were read from Vault KV
echo "Verifying unseal keys were read from Vault KV..."
UNSEALER_POD=$(kubectl get pod -n $NAMESPACE -l app.kubernetes.io/instance=vault-unsealer -o name)
echo "Waiting for unsealer to read keys from Vault KV..."
MAX_RETRIES=30
RETRY_COUNT=0
while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
  if kubectl logs -n $NAMESPACE $UNSEALER_POD 2>&1 | grep -q "successfully read unseal keys from vault"; then
    echo "SUCCESS: Unsealer read unseal keys from Vault KV"
    kubectl logs -n $NAMESPACE $UNSEALER_POD 2>&1 | grep "successfully read unseal keys from vault"
    break
  fi
  RETRY_COUNT=$((RETRY_COUNT + 1))
  echo "Waiting for unsealer to process... (${RETRY_COUNT}/${MAX_RETRIES})"
  sleep 2
done

if [ $RETRY_COUNT -eq $MAX_RETRIES ]; then
  echo "FAILURE: Unsealer did not read unseal keys from Vault KV within timeout"
  echo "Unsealer logs:"
  kubectl logs -n $NAMESPACE $UNSEALER_POD
  exit 1
fi

# Test unsealer by restarting Vault pods sequentially
echo ""
echo "=========================================="
echo "Testing unsealer: Restarting Vault pods sequentially"
echo "=========================================="

for ((i=0; i<${#PODS[@]}; i++)); do
  POD="${PODS[$i]}"
  echo ""
  echo "Restarting pod ${POD}..."
  kubectl delete pod -n $NAMESPACE "$POD" --grace-period=0 --force

  # Wait for the pod to be recreated and running
  echo "Waiting for ${POD} to be recreated..."
  kubectl wait -n $NAMESPACE --for=condition=Running "pod/$POD" --timeout=120s

  # Wait for the unsealer to unseal the pod
  echo "Waiting for unsealer to unseal ${POD}..."
  RETRY_COUNT=0
  while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
    if kubectl logs -n $NAMESPACE $UNSEALER_POD 2>&1 | grep -q "successfully unsealed vault"; then
      echo "SUCCESS: Unsealer unsealed ${POD}"
      break
    fi
    RETRY_COUNT=$((RETRY_COUNT + 1))
    echo "  Waiting for unsealer... (${RETRY_COUNT}/${MAX_RETRIES})"
    sleep 2
  done

  if [ $RETRY_COUNT -eq $MAX_RETRIES ]; then
    echo "FAILURE: Unsealer did not unseal ${POD} within timeout"
    echo "Unsealer logs:"
    kubectl logs -n $NAMESPACE $UNSEALER_POD
    exit 1
  fi

  # Clear the unseal log for the next iteration
  kubectl logs -n $NAMESPACE $UNSEALER_POD --since=1s >/dev/null 2>&1

  # Wait for the pod to be ready (unsealed pods pass readiness probe)
  echo "Waiting for ${POD} to be ready..."
  kubectl wait --for=condition=ready pod "$POD" -n $NAMESPACE --timeout=60s
  echo "Pod ${POD} is ready and unsealed."
done

echo ""
echo "=========================================="
echo "SUCCESS: All Vault pods were restarted and unsealed by the unsealer"
echo "=========================================="
