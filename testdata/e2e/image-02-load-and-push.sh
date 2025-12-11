#!/bin/bash
set -e
docker load -i /tmp/image.tar
docker push localhost:5001/vault-unsealer:e2e