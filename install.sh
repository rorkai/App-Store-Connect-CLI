#!/usr/bin/env bash
set -euo pipefail

REPO="rorkai/App-Store-Connect-CLI"
BIN_NAME="asc"
DEFAULT_INSTALL_DIR="/usr/local/bin"
if [ -n "${HOME:-}" ]; then
  DEFAULT_INSTALL_DIR="${HOME}/.local/bin"
fi
INSTALL_DIR="${INSTALL_DIR:-${DEFAULT_INSTALL_DIR}}"

OS="$(uname -s)"
ARCH="$(uname -m)"

case "${OS}" in
  Darwin) OS="macOS" ;;
  Linux) OS="linux" ;;
  *)
    echo "Unsupported OS: ${OS}"
    exit 1
    ;;
esac

case "${ARCH}" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *)
    echo "Unsupported architecture: ${ARCH}"
    exit 1
    ;;
esac

LATEST_URL="$(curl -fsSL -o /dev/null -w "%{url_effective}" "https://github.com/${REPO}/releases/latest")"
VERSION="${LATEST_URL##*/}"
if [ -z "${VERSION}" ] || [ "${VERSION}" = "latest" ]; then
  echo "Could not determine latest version."
  exit 1
fi

ASSET="${BIN_NAME}_${VERSION}_${OS}_${ARCH}"
BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"
BIN_URL="${BASE_URL}/${ASSET}"
CHECKSUMS_ASSET="${BIN_NAME}_${VERSION}_checksums.txt"
CHECKSUMS_URL="${BASE_URL}/${CHECKSUMS_ASSET}"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

echo "Downloading ${ASSET}..."
curl -fsSL "${BIN_URL}" -o "${TMP_DIR}/${ASSET}"

if curl -fsSL "${CHECKSUMS_URL}" -o "${TMP_DIR}/checksums.txt"; then
  if command -v shasum >/dev/null 2>&1 || command -v sha256sum >/dev/null 2>&1; then
    EXPECTED="$(grep -E "[ *]${ASSET}$" "${TMP_DIR}/checksums.txt" | awk '{print $1}')"
    if [ -n "${EXPECTED}" ]; then
      if command -v shasum >/dev/null 2>&1; then
        ACTUAL="$(shasum -a 256 "${TMP_DIR}/${ASSET}" | awk '{print $1}')"
      else
        ACTUAL="$(sha256sum "${TMP_DIR}/${ASSET}" | awk '{print $1}')"
      fi
      if [ "${EXPECTED}" != "${ACTUAL}" ]; then
        echo "Checksum verification failed."
        exit 1
      fi
      echo "Checksum verified."
    else
      echo "Warning: Asset not found in checksums.txt. Skipping verification."
    fi
  else
    echo "Warning: No checksum tool (shasum/sha256sum) available. Skipping verification."
  fi
else
  echo "Warning: Could not download checksums.txt. Skipping verification."
fi

if ! mkdir -p "${INSTALL_DIR}" 2>/dev/null; then
  if command -v sudo >/dev/null 2>&1; then
    sudo mkdir -p "${INSTALL_DIR}"
  else
    echo "Cannot create ${INSTALL_DIR}; try running with sudo or set INSTALL_DIR."
    exit 1
  fi
fi

if [ -w "${INSTALL_DIR}" ]; then
  install -m 755 "${TMP_DIR}/${ASSET}" "${INSTALL_DIR}/${BIN_NAME}"
else
  if command -v sudo >/dev/null 2>&1; then
    sudo install -m 755 "${TMP_DIR}/${ASSET}" "${INSTALL_DIR}/${BIN_NAME}"
  else
    echo "Cannot write to ${INSTALL_DIR}; try running with sudo or set INSTALL_DIR."
    exit 1
  fi
fi

echo "Installed ${BIN_NAME} to ${INSTALL_DIR}/${BIN_NAME}"
echo "Run: ${BIN_NAME} --help"
if [[ ":${PATH}:" != *":${INSTALL_DIR}:"* ]]; then
  echo "Note: ${INSTALL_DIR} is not in your PATH."
  echo "Add it to your shell profile, e.g.:"
  echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
fi
