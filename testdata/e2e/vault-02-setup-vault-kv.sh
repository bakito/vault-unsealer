#!/bin/bash

set -e

# Variables
NAMESPACE="vault"
RELEASE_NAME="vault"
RAFT_JOIN_THRESHOLD=3
RAFT_KEY_THRESHOLD=2
SECRET_FILE_UNSEAL_KEYS="secret-unseal-keys.yaml"

# Get Vault pods
PODS=($(kubectl get pods -n $NAMESPACE -l app.kubernetes.io/name=vault -o jsonpath="{.items[*].metadata.name}"))
LEADER_POD="${PODS[0]}"

echo "Wait for pod ${LEADER_POD}..."
kubectl wait -n "$NAMESPACE" --for=jsonpath='{.status.phase}'=Running "pod/$LEADER_POD" --timeout=180s

wait_for_vault_api() {
  local attempts=0
  until kubectl exec -n "$NAMESPACE" "$LEADER_POD" -- vault status -format=json 2>/dev/null | jq -e 'type == "object"' >/dev/null; do
    attempts=$((attempts + 1))
    if [ "$attempts" -ge 30 ]; then
      echo "Vault API did not become ready in time"
      kubectl describe pod -n "$NAMESPACE" "$LEADER_POD" || true
      kubectl logs -n "$NAMESPACE" "$LEADER_POD" --tail=200 || true
      exit 1
    fi
    echo "Waiting for Vault API on ${LEADER_POD}... (${attempts}/30)"
    sleep 2
  done
}

wait_for_vault_api

# Initialize Vault on first pod
echo "Initializing Vault on ${PODS[0]}..."
INIT_OUTPUT=$(kubectl exec -n $NAMESPACE "${PODS[0]}" -- vault operator init -format=json -key-shares=$RAFT_JOIN_THRESHOLD -key-threshold=$RAFT_KEY_THRESHOLD)
echo "$INIT_OUTPUT" > vault-init.json

# Extract keys
UNSEAL_KEYS=($(echo "$INIT_OUTPUT" | jq -r '.unseal_keys_b64[]'))
ROOT_TOKEN=$(echo "$INIT_OUTPUT" | jq -r '.root_token')

# Unseal leader pod
echo "Unsealing leader Vault pod..."
for ((i=0; i<RAFT_KEY_THRESHOLD; i++)); do
    kubectl exec -n "$NAMESPACE" "$LEADER_POD" -- vault operator unseal "${UNSEAL_KEYS[$i]}"
done


echo "Initializing and unsealing other Vault pods..."
for ((i=1; i<${#PODS[@]}; i++)); do
  FOLLOWER="${PODS[$i]}"
  echo "Waiting for pod ${FOLLOWER} to be running..."
  kubectl wait -n "$NAMESPACE" --for=jsonpath='{.status.phase}'=Running "pod/$FOLLOWER" --timeout=180s
  kubectl exec -n $NAMESPACE "$FOLLOWER" -- vault operator raft join "http://${LEADER_POD}.vault-internal:8200"
  # Wait for follower Vault API to be ready before unsealing
  echo "Waiting for Vault API on ${FOLLOWER}..."
  wait_attempts=0
  until kubectl exec -n "$NAMESPACE" "$FOLLOWER" -- vault status -format=json 2>/dev/null | jq -e 'type == "object"' >/dev/null; do
    wait_attempts=$((wait_attempts + 1))
    if [ "$wait_attempts" -ge 30 ]; then
      echo "Vault API on ${FOLLOWER} did not become ready in time"
      exit 1
    fi
    echo "  Waiting... (${wait_attempts}/30)"
    sleep 2
  done
  echo "Unsealing $FOLLOWER..."
  for ((k=0; k<RAFT_KEY_THRESHOLD; k++)); do
    kubectl exec -n "$NAMESPACE" "$FOLLOWER" -- vault operator unseal "${UNSEAL_KEYS[$k]}"
  done
  # A raft follower completes unsealing asynchronously (join challenge + sync),
  # so poll until it reports unsealed (vault status exits 2 when sealed, so ignore exit code)
  seal_attempts=0
  until [ "$(kubectl exec -n "$NAMESPACE" "$FOLLOWER" -- vault status -format=json 2>/dev/null | jq -r '.sealed' || true)" = "false" ]; do
    seal_attempts=$((seal_attempts + 1))
    if [ "$seal_attempts" -ge 30 ]; then
      echo "Vault pod ${FOLLOWER} is still sealed"
      kubectl logs -n "$NAMESPACE" "$FOLLOWER" --tail=100 || true
      exit 1
    fi
    echo "  Waiting for ${FOLLOWER} to be unsealed... (${seal_attempts}/30)"
    sleep 2
  done
  echo "${FOLLOWER} is unsealed"
done

# Wait for all Vault pods to be ready before making API calls
echo "Waiting for all Vault pods to be ready..."
for ((i=1; i<${#PODS[@]}; i++)); do
  FOLLOWER="${PODS[$i]}"
  kubectl wait --for=condition=ready pod "$FOLLOWER" -n $NAMESPACE --timeout=60s
done

# Login with the root token (required for all following API calls)
echo "Logging in with root token..."
kubectl exec -n "$NAMESPACE" "$LEADER_POD" -- vault login -no-print "$ROOT_TOKEN"

# Create the admin policy used by the unsealer user to read the unseal keys
echo "Creating admin policy..."
# sys/mounts read is required by the unsealer to detect the kv version of the secret engine
cat <<'EOF' | kubectl exec -i -n "$NAMESPACE" "$LEADER_POD" -- vault policy write admin -
path "unsealer/*" {
  capabilities = ["read", "list"]
}
path "sys/mounts" {
  capabilities = ["read"]
}
EOF

# Enable userpass auth method for the unsealer
echo "Enabling userpass auth method..."
kubectl exec -n "$NAMESPACE" "$LEADER_POD" -- vault auth enable userpass

echo "Creating admin user..."
kubectl exec -n "$NAMESPACE" "$LEADER_POD" -- vault write auth/userpass/users/admin password=admin policies=admin

# Enable KV v2 secrets engine and write unseal keys to Vault
echo "Enabling KV v2 secrets engine..."
kubectl exec -n "$NAMESPACE" "$LEADER_POD" -- vault secrets enable -path=unsealer -version=2 kv

echo "Writing unseal keys to Vault KV v2..."
kubectl exec -n "$NAMESPACE" "$LEADER_POD" -- sh -c "vault kv put unsealer/unseal unsealKey1='${UNSEAL_KEYS[0]}' unsealKey2='${UNSEAL_KEYS[1]}' unsealKey3='${UNSEAL_KEYS[2]}'"

echo "Write vault unsealer secret"
cat <<EOF > $SECRET_FILE_UNSEAL_KEYS
apiVersion: v1
stringData:
  secretPath: unsealer/unseal
  username: admin
  password: admin

kind: Secret
metadata:
  labels:
    vault-unsealer.bakito.net/stateful-set: ${RELEASE_NAME}
  name: unseal
  namespace: ${NAMESPACE}
type: Opaque
EOF

kubectl apply -f $SECRET_FILE_UNSEAL_KEYS
