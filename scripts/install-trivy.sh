#!/bin/sh
# Installs a pinned Trivy release into the given folder, checking the archive against the release's checksum file.
# The binary is used instead of aquasecurity/trivy-action: on 19/03/2026 most of that action's tags were moved to
# credential-stealing code (GHSA-69fq-xp46-6x23). Releases since v0.69.3 are immutable on GitHub. See docs/infra/CI.md.
set -eu

version=0.76.1
dest="${1:?usage: install-trivy.sh <folder>}"

base="https://github.com/aquasecurity/trivy/releases/download/v${version}"
case "$(uname -m)" in
  x86_64) arch=64bit ;;
  aarch64 | arm64) arch=ARM64 ;;
  *) echo "unsupported architecture $(uname -m)" >&2; exit 1 ;;
esac
archive="trivy_${version}_Linux-${arch}.tar.gz"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

curl -sSfL -o "$tmp/$archive" "$base/$archive"
curl -sSfL "$base/trivy_${version}_checksums.txt" | grep " $archive\$" > "$tmp/checksum.txt"
(cd "$tmp" && sha256sum -c checksum.txt)
tar -xzf "$tmp/$archive" -C "$tmp" trivy
mkdir -p "$dest"
mv "$tmp/trivy" "$dest/trivy"
"$dest/trivy" --version | head -1
