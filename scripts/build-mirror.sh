#!/usr/bin/env bash

set -euo pipefail

provider_host="providers.civico.de"
provider_namespace="civico"
provider_type="checkmk"

if [[ $# -lt 2 ]]; then
  echo "usage: $0 VERSION TARGET_DIR [OS_ARCH ...]" >&2
  exit 64
fi

version="${1#v}"
target_dir="$2"
shift 2

if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+([+-][0-9A-Za-z.-]+)?$ ]]; then
  echo "VERSION must be a semantic version such as 0.1.0" >&2
  exit 64
fi

for command_name in go zip; do
  if ! command -v "$command_name" >/dev/null 2>&1; then
    echo "required command not found: $command_name" >&2
    exit 69
  fi
done

if command -v shasum >/dev/null 2>&1; then
  sha256_file() {
    shasum -a 256 "$1" | awk '{print $1}'
  }
elif command -v sha256sum >/dev/null 2>&1; then
  sha256_file() {
    sha256sum "$1" | awk '{print $1}'
  }
else
  echo "no SHA-256 checksum command found" >&2
  exit 69
fi

if [[ $# -eq 0 ]]; then
  host_os="$(go env GOOS)"
  host_arch="$(go env GOARCH)"
  platforms=("${host_os}_${host_arch}")
else
  platforms=("$@")
fi

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_dir="$(cd -- "$script_dir/.." && pwd)"
mkdir -p -- "$target_dir"
mirror_dir="$(cd -- "$target_dir" && pwd)"
package_dir="$mirror_dir/$provider_host/$provider_namespace/$provider_type"
work_dir="$(mktemp -d "${TMPDIR:-/tmp}/checkmk-provider-mirror.XXXXXX")"
trap 'rm -rf -- "$work_dir"' EXIT

mkdir -p -- "$package_dir"

for platform in "${platforms[@]}"; do
  if [[ ! "$platform" =~ ^[a-z0-9]+_[a-z0-9]+$ ]]; then
    echo "invalid platform $platform; expected OS_ARCH such as linux_amd64" >&2
    exit 64
  fi

  target_os="${platform%%_*}"
  target_arch="${platform#*_}"
  binary_name="terraform-provider-${provider_type}_v${version}"
  if [[ "$target_os" == "windows" ]]; then
    binary_name="${binary_name}.exe"
  fi

  binary_path="$work_dir/$platform/$binary_name"
  archive_path="$package_dir/terraform-provider-${provider_type}_${version}_${platform}.zip"
  mkdir -p -- "$(dirname -- "$binary_path")"

  (
    cd -- "$repo_dir"
    CGO_ENABLED=0 GOOS="$target_os" GOARCH="$target_arch" \
      go build -tags checkmk_all -trimpath -ldflags "-s -w -X main.version=$version" -o "$binary_path" .
  )
  chmod 0755 "$binary_path"
  touch -t 198001010000 "$binary_path"
  rm -f -- "$archive_path"
  zip -X -j -q "$archive_path" "$binary_path"
  archive_sha256="$(sha256_file "$archive_path")"
  echo "$archive_sha256  $archive_path"
done

# The registry ingests a signed SHA256SUMS pair, so emit the sums file here and
# sign it separately with the release key.
sums_path="$package_dir/terraform-provider-${provider_type}_${version}_SHA256SUMS"
: >"$sums_path"
for archive_path in "$package_dir"/terraform-provider-${provider_type}_${version}_*.zip; do
  [[ -e "$archive_path" ]] || continue
  printf '%s  %s\n' "$(sha256_file "$archive_path")" "$(basename -- "$archive_path")" >>"$sums_path"
done

versions_file="$work_dir/versions"
for archive_path in "$package_dir"/terraform-provider-${provider_type}_*.zip; do
  archive_name="$(basename -- "$archive_path")"
  archive_meta="${archive_name#terraform-provider-${provider_type}_}"
  archive_version="${archive_meta%%_*}"
  echo "$archive_version"
done | LC_ALL=C sort -u >"$versions_file"

{
  printf '{"versions":{'
  first=true
  while IFS= read -r available_version; do
    if [[ "$first" == false ]]; then
      printf ','
    fi
    printf '"%s":{}' "$available_version"
    first=false
  done <"$versions_file"
  printf '}}\n'
} >"$package_dir/index.json"

while IFS= read -r available_version; do
  {
    printf '{"archives":{'
    first=true
    for archive_path in "$package_dir"/terraform-provider-${provider_type}_${available_version}_*.zip; do
      archive_name="$(basename -- "$archive_path")"
      archive_meta="${archive_name#terraform-provider-${provider_type}_${available_version}_}"
      archive_platform="${archive_meta%.zip}"
      archive_sha256="$(sha256_file "$archive_path")"
      if [[ "$first" == false ]]; then
        printf ','
      fi
      printf '"%s":{"url":"%s","hashes":["zh:%s"]}' \
        "$archive_platform" "$archive_name" "$archive_sha256"
      first=false
    done
    printf '}}\n'
  } >"$package_dir/$available_version.json"
done <"$versions_file"

echo "private provider mirror written to $mirror_dir"
