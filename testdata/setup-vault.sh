#!/bin/bash

set -e

# Variables
NAMESPACE="vault"
RELEASE_NAME="vault"
VALUES_FILE="vault-values.yaml"
RAFT_JOIN_THRESHOLD=3
RAFT_KEY_THRESHOLD=2

cat <<EOF > $VALUES_FILE
server:
  ha:
    enabled: true
    raft:
      enabled: true
  replicas: ${RAFT_JOIN_THRESHOLD}
  dataStorage:
    enabled: true
    size: 1Gi
  standalone:
    enabled: false
  affinity: ""
injector:
  enabled: false
ui:
  enabled: false
EOF

# Create namespace
kubectl create namespace $NAMESPACE || true

# Add HashiCorp repo and install chart
helm repo add hashicorp https://helm.releases.hashicorp.com || true
helm repo update
helm upgrade --install $RELEASE_NAME hashicorp/vault -n $NAMESPACE -f $VALUES_FILE

echo "Waiting 20s for Vault pods to be created..."
sleep 20  # Give time for pods to start

# Get Vault pods
PODS=($(kubectl get pods -n $NAMESPACE -l app.kubernetes.io/name=vault -o jsonpath="{.items[*].metadata.name}"))
LEADER_POD="${PODS[0]}"

# Get pod names
PODS=($(kubectl get pods -n $NAMESPACE -l app.kubernetes.io/name=vault -o jsonpath="{.items[*].metadata.name}"))

# Initialize Vault on first pod
echo "Initializing Vault on ${PODS[0]}..."
INIT_OUTPUT=$(kubectl exec -n $NAMESPACE "${PODS[0]}" -- vault operator init -format=json -key-shares=$RAFT_JOIN_THRESHOLD -key-threshold=$RAFT_KEY_THRESHOLD)
echo "$INIT_OUTPUT" > vault-init.json

# Extract keys
UNSEAL_KEYS=($(echo "$INIT_OUTPUT" | jq -r '.unseal_keys_b64[]'))
ROOT_TOKEN=$(echo "$INIT_OUTPUT" | jq -r '.root_token')

# Unseal leader pod
echo "Unsealing leader Vault pod..."
LEADER_POD="${PODS[0]}"

kubectl exec -n $NAMESPACE "$LEADER_POD" -- vault operator unseal "${UNSEAL_KEYS[0]}"
kubectl exec -n $NAMESPACE "$LEADER_POD" -- vault operator unseal "${UNSEAL_KEYS[1]}"

echo "Initializing and unsealing other Vault pods..."
for ((i=1; i<${#PODS[@]}; i++)); do
  FOLLOWER="${PODS[$i]}"
  kubectl exec -n $NAMESPACE "$FOLLOWER" -- vault operator raft join "http://${LEADER_POD}.vault-internal:8200"
  echo "Unsealing $POD..."
  kubectl exec -n $NAMESPACE "$FOLLOWER" -- vault operator unseal "${UNSEAL_KEYS[0]}"
  kubectl exec -n $NAMESPACE "$FOLLOWER" -- vault operator unseal "${UNSEAL_KEYS[1]}"
done

# Login and verify
kubectl exec -n $NAMESPACE "${PODS[0]}" -- vault login "$ROOT_TOKEN"
kubectl exec -n $NAMESPACE "${PODS[0]}" -- vault operator raft list-peers
