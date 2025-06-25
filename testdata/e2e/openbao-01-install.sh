#!/bin/bash

set -e

# Variables
NAMESPACE="openbao"
RELEASE_NAME="openbao"
VALUES_FILE="./testdata/e2e/openbao-values.yaml"

# Create namespace
kubectl create namespace $NAMESPACE || true

# Add HashiCorp repo and install chart
helm repo add openbao https://openbao.github.io/openbao-helm || true
helm repo update
helm upgrade --install $RELEASE_NAME  openbao/openbao -n $NAMESPACE -f $VALUES_FILE
