#!/bin/bash
set -e

helm upgrade --install vault-unsealer chart \
  --namespace vault \
  -f ./testdata/e2e/unsealer-values.yaml \
  --atomic
