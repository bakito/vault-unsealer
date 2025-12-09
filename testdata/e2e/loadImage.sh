#!/bin/bash
set -e
docker load -i /tmp/vault-unsealer-e2e.tar
docker push localhost:5001/vault-unsealer:e2e