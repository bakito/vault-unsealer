#!/bin/bash
set -e

helm upgrade --install vault-unsealer chart \
  --namespace ${1} \
  -f ./testdata/e2e/unsealer-values.yaml \
  --atomic
