#!/bin/bash
set -e
docker build -f Dockerfile --build-arg VERSION=e2e-tests -t vault-unsealer:e2e .
docker save vault-unsealer:e2e -o /tmp/image.tar
