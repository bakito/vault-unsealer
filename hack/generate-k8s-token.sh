#!/bin/bash
# Before launch external tool for IntelliJ to get a value k8s token for sa auth
kubectl create token my-vault-unsealer > dist/k8s-token
