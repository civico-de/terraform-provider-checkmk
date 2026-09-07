#!/usr/bin/env bash

# Registers one provider release with the private Terralist registry. Terralist
# ingests by URL, so the archives must already be reachable when this runs.
# Pass a base URL, or "-" plus ASSET_URL_MAP for a host that addresses files by
# id instead of by name, such as the GitHub release asset API.

set -euo pipefail

provider_namespace="civico"
provider_type="checkmk"
protocols='["6.0"]'

if [[ $# -ne 3 ]]; then
  echo "usage: $0 VERSION MIRROR_DIR ASSET_BASE_URL|-" >&2
  echo "env: TERRALIST_URL, TERRALIST_API_KEY" >&2
  echo "     ASSET_URL_MAP   JSON object mapping file name to URL, required for '-'" >&2
  echo "     ASSET_HEADERS   newline-separated 'Name: value' headers for the fetches" >&2
  exit 64
fi

version="${1#v}"
mirror_dir="$2"
asset_base="${3%/}"

: "${TERRALIST_URL:?TERRALIST_URL is required}"
: "${TERRALIST_API_KEY:?TERRALIST_API_KEY is required}"

if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+([+-][0-9A-Za-z.-]+)?$ ]]; then
  echo "VERSION must be a semantic version such as 0.1.0" >&2
  exit 64
fi

package_dir="$mirror_dir/providers.civico.de/$provider_namespace/$provider_type"
sums_name="terraform-provider-${provider_type}_${version}_SHA256SUMS"
sums_path="$package_dir/$sums_name"

for required in "$sums_path" "$sums_path.sig"; do
  if [[ ! -f "$required" ]]; then
    echo "missing release artifact: $required" >&2
    exit 66
  fi
done

if [[ "$asset_base" == "-" && ! -f "${ASSET_URL_MAP:-}" ]]; then
  echo "ASSET_URL_MAP must point at a JSON file when the base URL is '-'" >&2
  exit 64
fi

body="$(
  SUMS_PATH="$sums_path" ASSET_BASE="$asset_base" VERSION="$version" \
  PROVIDER_TYPE="$provider_type" SUMS_NAME="$sums_name" PROTOCOLS="$protocols" \
  ASSET_AUTH_HEADER="${ASSET_AUTH_HEADER:-}" python3 - <<'PY'
import json
import os
import sys

sums_path = os.environ["SUMS_PATH"]
base = os.environ["ASSET_BASE"]
version = os.environ["VERSION"]
provider_type = os.environ["PROVIDER_TYPE"]
prefix = f"terraform-provider-{provider_type}_{version}_"

url_map = {}
map_path = os.environ.get("ASSET_URL_MAP", "")
if map_path:
    with open(map_path, encoding="utf-8") as handle:
        url_map = json.load(handle)


def asset_url(name):
    if base != "-":
        return f"{base}/{name}"
    if name not in url_map:
        sys.exit(f"ASSET_URL_MAP has no entry for {name}")
    return url_map[name]


platforms = []
for line in open(sums_path, encoding="utf-8"):
    digest, name = line.split()
    if not name.startswith(prefix) or not name.endswith(".zip"):
        continue
    target = name[len(prefix):-len(".zip")]
    target_os, _, arch = target.partition("_")
    if not target_os or not arch:
        sys.exit(f"cannot derive os/arch from {name}")
    platforms.append({
        "os": target_os,
        "arch": arch,
        "filename": name,
        "download_url": asset_url(name),
        "shasum": digest,
    })

if not platforms:
    sys.exit(f"no provider archives listed in {sums_path}")

payload = {
    "protocols": json.loads(os.environ["PROTOCOLS"]),
    "shasums": {
        "url": asset_url(os.environ["SUMS_NAME"]),
        "signature_url": asset_url(os.environ["SUMS_NAME"] + ".sig"),
    },
    "platforms": platforms,
}

headers = {}
for line in os.environ.get("ASSET_HEADERS", "").splitlines():
    line = line.strip()
    if not line:
        continue
    name, separator, value = line.partition(":")
    if not separator or not value.strip():
        sys.exit("ASSET_HEADERS entries must look like 'Name: value'")
    headers[name.strip()] = value.strip()
if headers:
    payload["headers"] = headers

print(json.dumps(payload))
PY
)"

upload_url="${TERRALIST_URL%/}/v1/api/providers/$provider_namespace/$provider_type/$version/upload"
response_file="$(mktemp)"
trap 'rm -f -- "$response_file"' EXIT

# The API key and any asset header stay out of argv and out of the log.
status="$(
  printf '%s' "$body" | curl -sS -o "$response_file" -w '%{http_code}' \
    -X POST "$upload_url" \
    -H "X-API-Key: $TERRALIST_API_KEY" \
    -H 'Content-Type: application/json' \
    --data-binary @-
)"

if [[ "$status" != "200" ]]; then
  echo "upload failed with HTTP $status" >&2
  # Terralist echoes the fetch URL on failure, never the key or the header.
  cat "$response_file" >&2
  exit 70
fi

echo "published $provider_namespace/$provider_type $version to ${TERRALIST_URL%/}"
