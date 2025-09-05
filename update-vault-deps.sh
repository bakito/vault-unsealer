#!/usr/bin/env bash
set -euo pipefail

VAULT_VERSION=$(grep "	github.com/hashicorp/vault v" "go.mod" | awk '{print $2}')

if [ -z "${GITHUB_TOKEN}" ]; then
    echo "⛔ Please set GITHUB_TOKEN environment variable"
    exit 1
fi

echo "🔍 Got vault version $VAULT_VERSION"

COMMIT_HASH=$(curl -s -H "Authorization: token ${GITHUB_TOKEN}" \
    "https://api.github.com/repos/hashicorp/vault/tags" | \
    jq -r ".[] | select(.name == \"${VAULT_VERSION}\") | .commit.sha")

if [ -z "${COMMIT_HASH}" ]; then
    echo "❌ Could not find commit hash for version ${VAULT_VERSION}"
    exit 1
fi

echo "📝 updating github.com/hashicorp/vault/api and github.com/hashicorp/vault/sdk to SHA ${COMMIT_HASH}"
sed -i "s|github.com/hashicorp/vault/api => github.com/hashicorp/vault/api .*|github.com/hashicorp/vault/api => github.com/hashicorp/vault/api ${COMMIT_HASH}|" go.mod
sed -i "s|github.com/hashicorp/vault/sdk => github.com/hashicorp/vault/sdk .*|github.com/hashicorp/vault/sdk => github.com/hashicorp/vault/sdk ${COMMIT_HASH}|" go.mod

echo "🔄 Updating go moduls"
go mod tidy