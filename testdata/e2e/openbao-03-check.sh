#!/bin/bash

set -e

# Variables
NAMESPACE="openbao"

echo "Waiting for openbao statefulset to be ready..."
kubectl wait --for=jsonpath='{.status.replicas}'=3 --timeout=180s -n $NAMESPACE statefulset/openbao

PODS=($(kubectl get pods -n $NAMESPACE -l app.kubernetes.io/name=openbao -o jsonpath="{.items[*].metadata.name}"))

echo "Waiting for openbao follower pods to be ready..."
for ((i=1; i<${#PODS[@]}; i++)); do
  FOLLOWER="${PODS[$i]}"
  kubectl wait --for=condition=ready pod $FOLLOWER -n $NAMESPACE --timeout=60s
done

ROOT_TOKEN=$(cat openbao-init.json | jq -r '.root_token')

# Login and verify
kubectl exec -n $NAMESPACE "${PODS[0]}" -- bao login "$ROOT_TOKEN"
kubectl exec -n $NAMESPACE "${PODS[0]}" -- bao operator raft list-peers
