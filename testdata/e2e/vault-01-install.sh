#!/bin/bash

set -e

# Variables
NAMESPACE="vault"
RELEASE_NAME="vault"
VALUES_FILE="./testdata/e2e/vault-values.yaml"

# Create namespace
kubectl create namespace $NAMESPACE || true

# Add HashiCorp repo and install chart
helm repo add hashicorp https://helm.releases.hashicorp.com || true
helm repo update
helm upgrade --install $RELEASE_NAME hashicorp/vault -n $NAMESPACE -f $VALUES_FILE
