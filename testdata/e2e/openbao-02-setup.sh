#!/bin/bash

set -e

# Variables
NAMESPACE="openbao"
RELEASE_NAME="openbao"
RAFT_JOIN_THRESHOLD=3
RAFT_KEY_THRESHOLD=2
SECRET_FILE_UNSEAL_KEYS="secret-unseal-keys.yaml"

# Get openbao pods
PODS=($(kubectl get pods -n $NAMESPACE -l app.kubernetes.io/name=openbao -o jsonpath="{.items[*].metadata.name}"))
LEADER_POD="${PODS[0]}"

echo "Wait for pod ${LEADER_POD}..."
kubectl wait -n "$NAMESPACE" --for=jsonpath='{.status.phase}'=Running "pod/$LEADER_POD" --timeout=180s

# Initialize openbao on first pod
echo "Initializing openbao on ${PODS[0]}..."
INIT_OUTPUT=$(kubectl exec -n $NAMESPACE "${PODS[0]}" -- bao operator init -format=json -key-shares=$RAFT_JOIN_THRESHOLD -key-threshold=$RAFT_KEY_THRESHOLD)
echo "$INIT_OUTPUT" > openbao-init.json

# Extract keys
UNSEAL_KEYS=($(echo "$INIT_OUTPUT" | jq -r '.unseal_keys_b64[]'))
ROOT_TOKEN=$(echo "$INIT_OUTPUT" | jq -r '.root_token')

# Unseal leader pod
echo "Unsealing leader openbao pod..."
for ((i=0; i<RAFT_KEY_THRESHOLD; i++)); do
    kubectl exec -n "$NAMESPACE" "$LEADER_POD" -- bao operator unseal "${UNSEAL_KEYS[$i]}"
done

echo "Initializing and unsealing other openbao pods..."
for ((i=1; i<${#PODS[@]}; i++)); do
  FOLLOWER="${PODS[$i]}"
  kubectl exec -n $NAMESPACE "$FOLLOWER" -- bao operator raft join "http://${LEADER_POD}.openbao-internal:8200"
  #echo "Unsealing $POD..."
  #kubectl exec -n $NAMESPACE "$FOLLOWER" -- bao operator unseal "${UNSEAL_KEYS[0]}"
  #kubectl exec -n $NAMESPACE "$FOLLOWER" -- bao operator unseal "${UNSEAL_KEYS[1]}"
done

echo "Write openbao unsealer secret"
cat <<EOF > $SECRET_FILE_UNSEAL_KEYS
apiVersion: v1
stringData:
  unsealKey1: ${UNSEAL_KEYS[0]}
  unsealKey2: ${UNSEAL_KEYS[1]}
  unsealKey3: ${UNSEAL_KEYS[2]}

kind: Secret
metadata:
  labels:
    vault-unsealer.bakito.net/stateful-set: ${RELEASE_NAME}
  name: unseal
  namespace: ${NAMESPACE}
type: Opaque
EOF

kubectl apply -f $SECRET_FILE_UNSEAL_KEYS
