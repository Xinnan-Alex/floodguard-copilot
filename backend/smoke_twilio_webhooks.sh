#!/usr/bin/env bash

set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:3400}"
REQUIRE_TRIAGE="${REQUIRE_TRIAGE:-yes}"
FEEDS_URL="$BASE_URL/api/feeds"
VOICE_URL="$BASE_URL/api/webhook/voice"
WHATSAPP_URL="$BASE_URL/api/webhook/whatsapp"
RUN_ID="$(date +%s)"
VOICE_SID="CA-smoke-$RUN_ID"
VOICE_TEXT="Smoke test flood emergency $RUN_ID in Shah Alam with elderly trapped"
WA_TEXT="Smoke test WhatsApp flood report $RUN_ID in Klang with rising water"

if ! command -v curl >/dev/null 2>&1; then
	echo "curl is required"
	exit 1
fi

if ! command -v python3 >/dev/null 2>&1; then
	echo "python3 is required"
	exit 1
fi

echo "Using BASE_URL=$BASE_URL"
echo "Require triage=$REQUIRE_TRIAGE"

post_voice() {
	echo "Posting voice webhook..."
	curl -fsS -X POST "$VOICE_URL" \
		--data-urlencode "From=+60123456789" \
		--data-urlencode "CallStatus=in-progress" \
		--data-urlencode "CallSid=$VOICE_SID" \
		--data-urlencode "SpeechResult=$VOICE_TEXT" >/dev/null
}

post_whatsapp() {
	echo "Posting WhatsApp webhook..."
	curl -fsS -X POST "$WHATSAPP_URL" \
		--data-urlencode "From=whatsapp:+60111111111" \
		--data-urlencode "Body=$WA_TEXT" \
		--data-urlencode "NumMedia=0" >/dev/null
}

find_feed() {
	local kind="$1"
	local marker="$2"
	local require_triage="$3"
	local feeds_json

	feeds_json="$(curl -fsS "$FEEDS_URL")"
	FEEDS_JSON="$feeds_json" python3 - "$kind" "$marker" "$require_triage" <<'PY'
import json
import os
import sys

kind = sys.argv[1]
marker = sys.argv[2]
require_triage = sys.argv[3] == "yes"

try:
    items = json.loads(os.environ["FEEDS_JSON"])
except Exception:
    print("invalid-json")
    sys.exit(2)

for item in items:
    if item.get("type") != kind:
        continue
    haystack = "\n".join([
        str(item.get("preview", "")),
        str(item.get("fullData", "")),
        str(item.get("from", "")),
    ])
    if marker not in haystack:
        continue
    has_triage = isinstance(item.get("triageResult"), dict) and bool(item.get("triageResult"))
    if require_triage and not has_triage:
        print("found-without-triage")
        sys.exit(3)
    print(json.dumps({
        "id": item.get("id"),
        "type": item.get("type"),
        "preview": item.get("preview"),
        "hasTriage": has_triage,
    }))
    sys.exit(0)

print("not-found")
sys.exit(1)
PY
}

wait_for_feed() {
	local kind="$1"
	local marker="$2"
	local attempts="$3"
	local label="$4"
	local result
	local require_triage="$REQUIRE_TRIAGE"

	for ((i = 1; i <= attempts; i++)); do
		set +e
		result="$(find_feed "$kind" "$marker" "$require_triage" 2>/dev/null)"
		status=$?
		set -e
		if [[ $status -eq 0 ]]; then
			echo "$label verified: $result"
			return 0
		fi
		if [[ $status -eq 3 ]]; then
			echo "$label found, waiting for background triage..."
		else
			echo "Waiting for $label... ($i/$attempts)"
		fi
		sleep 2
	done

	if [[ "$require_triage" == "yes" ]]; then
		echo "$label verification failed: feed was ingested but triageResult did not appear. Redeploy the backend with the auto-triage changes or run with REQUIRE_TRIAGE=no for ingestion-only validation."
	else
		echo "$label verification failed: feed item did not appear in /api/feeds."
	fi
	return 1
}

post_voice
wait_for_feed "call" "$VOICE_SID" 12 "Voice webhook"

post_whatsapp
wait_for_feed "whatsapp" "$WA_TEXT" 12 "WhatsApp webhook"

echo "Smoke test passed"