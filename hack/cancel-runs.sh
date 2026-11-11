#!/usr/bin/env bash
# Cancel in-progress GitHub Actions runs younger than 1 hour.
# Older runs are left alone; zombie/ghost runs that reject cancellation are skipped with a warning.
set -euo pipefail

REPO="bakito/vault-unsealer"
MAX_AGE_SECONDS=3600 # 1 hour

echo "🔍 Fetching in-progress runs for ${REPO}..."

# Fetch run ID and startedAt timestamp for filtering by age
RUN_DATA=$(gh run list --repo "$REPO" --status=in_progress --json databaseId,startedAt --jq '.[] | "\(.databaseId) \(.startedAt)"')

if [ -z "$RUN_DATA" ]; then
  echo "✅ No in-progress runs found."
  exit 0
fi

NOW=$(date +%s)
COUNT=0
SKIPPED=0
TOO_OLD=0

while IFS= read -r line; do
  id=$(echo "$line" | awk '{print $1}')
  started_at=$(echo "$line" | awk '{print $2}')

  # Convert ISO 8601 timestamp to epoch seconds
  started_epoch=$(date -d "$started_at" +%s 2>/dev/null || echo 0)
  age=$((NOW - started_epoch))

  if [ "$age" -gt "$MAX_AGE_SECONDS" ]; then
    echo "  ⏭ Skipping run ${id} (started $((age / 60)) min ago, older than $((MAX_AGE_SECONDS / 60)) min)"
    TOO_OLD=$((TOO_OLD + 1))
    continue
  fi

  WORKFLOW=$(gh run view "$id" --repo "$REPO" --json workflow --jq '.name' 2>/dev/null || echo "unknown")
  echo -n "  ⏹ Canceling run ${id} (${WORKFLOW}, $((age / 60)) min ago)... "
  if gh run cancel "$id" --repo "$REPO" >/dev/null 2>&1; then
    echo "done"
    COUNT=$((COUNT + 1))
  else
    echo "skipped (zombie run)"
    SKIPPED=$((SKIPPED + 1))
  fi
done <<< "$RUN_DATA"

echo ""
echo "✅ Canceled ${COUNT} run(s), skipped ${SKIPPED} zombie run(s), ignored ${TOO_OLD} old run(s)."
