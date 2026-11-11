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
UNSEALER_POD=$(kubectl get pod -n $NAMESPACE -l app.kubernetes.io/instance=vault-unsealer -o name)
echo "Waiting for unsealer to read keys from OpenBao KV..."
MAX_RETRIES=30
RETRY_COUNT=0
while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
  if kubectl logs -n $NAMESPACE $UNSEALER_POD 2>&1 | grep -q "successfully read unseal keys from vault"; then
    echo "SUCCESS: Unsealer read unseal keys from OpenBao KV"
    kubectl logs -n $NAMESPACE $UNSEALER_POD 2>&1 | grep "successfully read unseal keys from vault"
    break
  fi
  RETRY_COUNT=$((RETRY_COUNT + 1))
  echo "Waiting for unsealer to process... (${RETRY_COUNT}/${MAX_RETRIES})"
  sleep 2
done

if [ $RETRY_COUNT -eq $MAX_RETRIES ]; then
  echo "FAILURE: Unsealer did not read unseal keys from OpenBao KV within timeout"
  echo "Unsealer logs:"
  kubectl logs -n $NAMESPACE $UNSEALER_POD
  exit 1
fi

# Test unsealer by restarting OpenBao pods sequentially
echo ""
echo "=========================================="
echo "Testing unsealer: Restarting OpenBao pods sequentially"
echo "=========================================="

for ((i=0; i<${#PODS[@]}; i++)); do
  POD="${PODS[$i]}"
  echo ""
  echo "Restarting pod ${POD}..."
  OLD_UID=$(kubectl get pod -n $NAMESPACE "$POD" -o jsonpath='{.metadata.uid}')
  RESTART_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  kubectl delete pod -n $NAMESPACE "$POD"

  # Wait for the pod to be recreated (new UID) and running
  # Note: "Running" is a pod phase, not a condition, so poll the phase instead of using kubectl wait
  echo "Waiting for ${POD} to be recreated..."
  RETRY_COUNT=0
  while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
    NEW_UID=$(kubectl get pod -n $NAMESPACE "$POD" -o jsonpath='{.metadata.uid}' 2>/dev/null || true)
    PHASE=$(kubectl get pod -n $NAMESPACE "$POD" -o jsonpath='{.status.phase}' 2>/dev/null || true)
    if [ -n "$NEW_UID" ] && [ "$NEW_UID" != "$OLD_UID" ] && [ "$PHASE" = "Running" ]; then
      echo "Pod ${POD} is running"
      break
    fi
    RETRY_COUNT=$((RETRY_COUNT + 1))
    echo "  Waiting for ${POD} to be running... (${RETRY_COUNT}/${MAX_RETRIES}, phase: ${PHASE:-none})"
    sleep 2
  done

  if [ $RETRY_COUNT -eq $MAX_RETRIES ]; then
    echo "FAILURE: Pod ${POD} was not recreated within timeout"
    kubectl get pods -n $NAMESPACE -o wide || true
    kubectl describe pod -n $NAMESPACE "$POD" || true
    exit 1
  fi

  # Wait for the unsealer to unseal the pod (only consider log lines for this pod since the restart)
  echo "Waiting for unsealer to unseal ${POD}..."
  RETRY_COUNT=0
  while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
    if kubectl logs -n $NAMESPACE $UNSEALER_POD --since-time="$RESTART_TIME" 2>&1 | \
      grep "successfully unsealed vault" | grep -q "\"pod\":\"${POD}\""; then
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

  # Wait for the pod to be ready (unsealed pods pass readiness probe)
  echo "Waiting for ${POD} to be ready..."
  kubectl wait --for=condition=ready pod "$POD" -n $NAMESPACE --timeout=60s
  echo "Pod ${POD} is ready and unsealed."
done

echo ""
echo "=========================================="
echo "SUCCESS: All OpenBao pods were restarted and unsealed by the unsealer"
echo "=========================================="
