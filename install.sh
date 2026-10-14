#!/usr/bin/env bash
# Downloads the nestor binary for the current OS/arch, verifies it against
# checksums.txt, and installs it to ~/.local/bin/nestor.
#
# Usage:
#   ./install.sh [base-url]
#
# base-url, if given, is the directory that holds the release assets
# (e.g. https://github.com/toniphan21/nestor/releases/download/v0.5.0).
# Without it, the latest GitHub release is resolved automatically.
set -euo pipefail

REPO="toniphan21/nestor"
INSTALL_DIR="${HOME}/.local/bin"
BASE_URL="${1:-}"

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$(uname -m)" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *) echo "unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

if [ -z "$BASE_URL" ]; then
  release_json="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest")"
  tag="$(printf '%s' "$release_json" | grep -m1 '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"
  if [ -z "$tag" ]; then
    echo "could not resolve the latest release tag" >&2
    exit 1
  fi
  BASE_URL="https://github.com/${REPO}/releases/download/${tag}"
fi

asset="nestor_${os}_${arch}.tar.gz"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

echo "downloading ${asset} from ${BASE_URL}"
curl -fsSL -o "${tmp_dir}/${asset}" "${BASE_URL}/${asset}"
curl -fsSL -o "${tmp_dir}/checksums.txt" "${BASE_URL}/checksums.txt"

cd "$tmp_dir"
want="$(grep " ${asset}\$" checksums.txt | awk '{print $1}')"
if [ -z "$want" ]; then
  echo "no checksum entry found for ${asset}" >&2
  exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
  got="$(sha256sum "$asset" | awk '{print $1}')"
else
  got="$(shasum -a 256 "$asset" | awk '{print $1}')"
fi

if [ "$want" != "$got" ]; then
  echo "checksum mismatch for ${asset}: want ${want}, got ${got}" >&2
  exit 1
fi

tar -xzf "$asset"
mkdir -p "$INSTALL_DIR"
install -m 0755 nestor "${INSTALL_DIR}/nestor"

echo "installed $("${INSTALL_DIR}/nestor" version) to ${INSTALL_DIR}/nestor"
case ":$PATH:" in
  *":${INSTALL_DIR}:"*) ;;
  *) echo "note: ${INSTALL_DIR} is not on your PATH" ;;
esac
