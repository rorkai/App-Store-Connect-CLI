#!/usr/bin/env bash

set -euo pipefail

readonly STEP_NAME="setup-asc"
readonly REPO_SLUG="rudrankriyam/App-Store-Connect-CLI"

TMP_DIR=""
RELEASE_TAG=""
ASSET_VERSION=""
ASSET_NAME=""
EXPECTED_CHECKSUM=""
CHECKSUMS_URL=""
ASSET_URL=""

log_info() {
  printf "[%s] %s\n" "${STEP_NAME}" "$*" >&2
}

log_warn() {
  printf "[%s] warning: %s\n" "${STEP_NAME}" "$*" >&2
}

fail() {
  printf "[%s] error: %s\n" "${STEP_NAME}" "$*" >&2
  exit 1
}

require_command() {
  local command_name="$1"
  if ! command -v "${command_name}" >/dev/null 2>&1; then
    fail "Missing required command: ${command_name}"
  fi
}

normalize_yes_no() {
  local value="${1:-}"
  local field_name="${2:-value}"

  case "${value}" in
    [Yy][Ee][Ss] | [Tt][Rr][Uu][Ee] | 1 | [Yy] | [Oo][Nn])
      printf "yes"
      ;;
    [Nn][Oo] | [Ff][Aa][Ll][Ss][Ee] | 0 | [Nn] | [Oo][Ff][Ff] | "")
      printf "no"
      ;;
    *)
      fail "Invalid value for ${field_name}: ${value}. Use yes or no."
      ;;
  esac
}

export_output() {
  local key="$1"
  local value="$2"

  if command -v envman >/dev/null 2>&1; then
    envman add --key "${key}" --value "${value}" >/dev/null
    return
  fi

  log_warn "envman not found, exporting output only for current process: ${key}"
  export "${key}=${value}"
}

resolve_requested_version() {
  local raw_version="${version:-latest}"
  local latest_url

  if [ -z "${raw_version}" ] || [ "${raw_version}" = "latest" ]; then
    latest_url="$(curl -fsSL -o /dev/null -w "%{url_effective}" "https://github.com/${REPO_SLUG}/releases/latest")" \
      || fail "Unable to resolve latest release version."
    raw_version="${latest_url##*/}"
  fi

  raw_version="${raw_version#v}"
  if [ -z "${raw_version}" ]; then
    fail "Version must not be empty."
  fi

  printf "%s" "${raw_version}"
}

detect_os_arch() {
  local os_name
  local arch_name

  os_name="$(uname -s)"
  arch_name="$(uname -m)"

  case "${os_name}" in
    Darwin)
      DETECTED_OS="macOS"
      ;;
    Linux)
      DETECTED_OS="linux"
      ;;
    *)
      fail "Unsupported operating system: ${os_name}"
      ;;
  esac

  case "${arch_name}" in
    x86_64 | amd64)
      DETECTED_ARCH="amd64"
      ;;
    arm64 | aarch64)
      DETECTED_ARCH="arm64"
      ;;
    *)
      fail "Unsupported architecture: ${arch_name}"
      ;;
  esac
}

extract_expected_checksum() {
  local checksums_file="$1"
  local asset_name="$2"

  awk -v target="${asset_name}" '
    {
      file_name = $2
      gsub(/^\*/, "", file_name)
      if (file_name == target) {
        print $1
        exit
      }
    }
  ' "${checksums_file}" | tr -d '\r\n '
}

resolve_release_artifacts() {
  local version_no_v="$1"
  local os_name="$2"
  local arch_name="$3"

  local checksums_file="${TMP_DIR}/checksums.txt"
  local candidate_tag
  local candidate_asset_version
  local checksums_name
  local candidate_asset_name
  local expected_checksum

  RELEASE_TAG=""
  ASSET_VERSION=""
  ASSET_NAME=""
  EXPECTED_CHECKSUM=""
  CHECKSUMS_URL=""
  ASSET_URL=""

  for candidate_tag in "${version_no_v}" "v${version_no_v}"; do
    for candidate_asset_version in "${version_no_v}" "v${version_no_v}"; do
      checksums_name="asc_${candidate_asset_version}_checksums.txt"
      CHECKSUMS_URL="https://github.com/${REPO_SLUG}/releases/download/${candidate_tag}/${checksums_name}"

      if ! curl -fsSL "${CHECKSUMS_URL}" -o "${checksums_file}"; then
        continue
      fi

      candidate_asset_name="asc_${candidate_asset_version}_${os_name}_${arch_name}"
      expected_checksum="$(extract_expected_checksum "${checksums_file}" "${candidate_asset_name}")"
      if [ -z "${expected_checksum}" ]; then
        continue
      fi

      RELEASE_TAG="${candidate_tag}"
      ASSET_VERSION="${candidate_asset_version}"
      ASSET_NAME="${candidate_asset_name}"
      EXPECTED_CHECKSUM="${expected_checksum}"
      ASSET_URL="https://github.com/${REPO_SLUG}/releases/download/${RELEASE_TAG}/${ASSET_NAME}"
      return
    done
  done

  fail "Could not resolve release artifacts for version ${version_no_v} (${os_name}/${arch_name})."
}

compute_sha256() {
  local file_path="$1"

  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "${file_path}" | awk '{print $1}' | tr -d '\r\n '
    return
  fi

  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "${file_path}" | awk '{print $1}' | tr -d '\r\n '
    return
  fi

  fail "Neither shasum nor sha256sum is available for checksum verification."
}

install_asc() {
  local install_dir_path="$1"
  local asset_path="${TMP_DIR}/${ASSET_NAME}"
  local actual_checksum
  local binary_path

  mkdir -p "${install_dir_path}"

  log_info "Downloading ${ASSET_NAME} from release ${RELEASE_TAG}."
  curl -fsSL "${ASSET_URL}" -o "${asset_path}" || fail "Failed to download ${ASSET_NAME}."

  actual_checksum="$(compute_sha256 "${asset_path}")"
  if [ "${actual_checksum}" != "${EXPECTED_CHECKSUM}" ]; then
    fail "Checksum mismatch for ${ASSET_NAME}."
  fi

  binary_path="${install_dir_path}/asc"
  cp "${asset_path}" "${binary_path}"
  chmod +x "${binary_path}"

  printf "%s" "${binary_path}"
}

configure_runtime_env() {
  local profile_value="$1"
  local debug_value="$2"
  local bypass_value="$3"

  if [ -n "${profile_value}" ]; then
    export ASC_PROFILE="${profile_value}"
  fi

  if [ "${debug_value}" = "yes" ]; then
    export ASC_DEBUG="1"
  fi

  if [ "${bypass_value}" = "yes" ]; then
    export ASC_BYPASS_KEYCHAIN="1"
  fi

  if [ -n "${key_id:-}" ]; then
    export ASC_KEY_ID="${key_id}"
  fi

  if [ -n "${issuer_id:-}" ]; then
    export ASC_ISSUER_ID="${issuer_id}"
  fi

  if [ -n "${private_key_path:-}" ]; then
    export ASC_PRIVATE_KEY_PATH="${private_key_path}"
  fi

  if [ -n "${private_key:-}" ]; then
    export ASC_PRIVATE_KEY="${private_key}"
  fi
}

run_user_command() {
  local run_command="$1"
  local working_directory="$2"
  local command_exit_code=0

  [ -d "${working_directory}" ] || fail "working_dir does not exist: ${working_directory}"

  log_info "Running user command in ${working_directory}."
  (
    cd "${working_directory}"
    bash -lc "${run_command}"
  ) || command_exit_code=$?

  export_output "ASC_COMMAND_EXIT_CODE" "${command_exit_code}"
  if [ "${command_exit_code}" -ne 0 ]; then
    fail "Command failed with exit code ${command_exit_code}."
  fi
}

main() {
  local mode_value
  local requested_version
  local install_dir_value
  local working_dir_value
  local profile_value
  local debug_value
  local bypass_value
  local run_command
  local asc_path
  local reported_version

  require_command curl

  mode_value="$(printf "%s" "${mode:-install}" | tr '[:upper:]' '[:lower:]')"
  case "${mode_value}" in
    install | run)
      ;;
    *)
      fail "Invalid mode: ${mode_value}. Use install or run."
      ;;
  esac

  requested_version="$(resolve_requested_version)"
  install_dir_value="${install_dir:-${HOME}/.local/bin}"
  working_dir_value="${working_dir:-${BITRISE_SOURCE_DIR:-$PWD}}"
  profile_value="${profile:-}"
  debug_value="$(normalize_yes_no "${debug:-no}" "debug")"
  bypass_value="$(normalize_yes_no "${bypass_keychain:-no}" "bypass_keychain")"
  run_command="${command:-}"

  if [ "${mode_value}" = "run" ] && [ -z "${run_command}" ]; then
    fail "command is required when mode=run."
  fi

  detect_os_arch

  TMP_DIR="$(mktemp -d)"
  trap 'rm -rf "${TMP_DIR}"' EXIT

  resolve_release_artifacts "${requested_version}" "${DETECTED_OS}" "${DETECTED_ARCH}"
  asc_path="$(install_asc "${install_dir_value}")"
  reported_version="${ASSET_VERSION#v}"
  export PATH="$(dirname "${asc_path}"):${PATH}"

  export_output "ASC_CLI_PATH" "${asc_path}"
  export_output "ASC_CLI_VERSION" "${reported_version}"

  if [ "${mode_value}" = "install" ]; then
    log_info "Installed asc ${reported_version} at ${asc_path}."
    exit 0
  fi

  configure_runtime_env "${profile_value}" "${debug_value}" "${bypass_value}"
  run_user_command "${run_command}" "${working_dir_value}"

  log_info "Command completed successfully."
}

main "$@"
