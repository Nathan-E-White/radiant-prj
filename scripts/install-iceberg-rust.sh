#!/usr/bin/env bash
set -Eeuo pipefail
IFS=$'\n\t'

SCRIPT_NAME="${0##*/}"
APACHE_DIST_BASE="${APACHE_DIST_BASE:-https://downloads.apache.org/iceberg}"
APACHE_KEYS_URL="${APACHE_KEYS_URL:-https://downloads.apache.org/iceberg/KEYS}"
VERSION="${ICEBERG_RUST_VERSION:-latest}"
PREFIX="${PREFIX:-/usr/local}"
SOURCE_ROOT="${ICEBERG_RUST_SOURCE_ROOT:-${PREFIX}/opt/iceberg-rust}"
BIN_DIR="${ICEBERG_RUST_BIN_DIR:-${PREFIX}/bin}"
PACKAGE="${ICEBERG_RUST_PACKAGE:-}"
BIN_NAME="${ICEBERG_RUST_BIN:-}"
SKIP_GPG="${ICEBERG_RUST_SKIP_GPG:-0}"
SKIP_SHA512="${ICEBERG_RUST_SKIP_SHA512:-0}"
SOURCE_ONLY=0
FORCE=0
KEEP_WORKDIR=0
LOCKED=1

usage() {
  cat <<USAGE
Usage: ${SCRIPT_NAME} [options]

Downloads Apache Iceberg Rust from ASF distribution, verifies SHA-512 and
OpenPGP signature, installs the verified source tree, and installs any Cargo
binary packages found in that source tree.

Options:
  --version VERSION       Iceberg Rust version to install, e.g. 0.9.1. Default: latest ASF release.
  --dist-base URL         ASF distribution base URL. Default: ${APACHE_DIST_BASE}
  --keys-url URL          KEYS URL for GPG verification. Default: ${APACHE_KEYS_URL}
  --prefix PATH           Install prefix. Default: ${PREFIX}
  --source-root PATH      Source install root. Default: ${SOURCE_ROOT}
  --bin-dir PATH          Destination for installed binaries. Default: ${BIN_DIR}
  --package NAME          Cargo package inside the verified source tree to install.
  --bin NAME              Cargo binary name to install. Requires --package.
  --source-only           Only install the verified source tree; do not run cargo install.
  --skip-gpg              Skip OpenPGP signature verification.
  --skip-sha512           Skip SHA-512 verification.
  --no-locked             Do not pass --locked to cargo install.
  --force                 Replace an existing source install and overwrite binaries.
  --keep-workdir          Keep temporary download/build directory for debugging.
  -h, --help              Show this help.

Environment overrides use the same names as the defaults above, for example:
  ICEBERG_RUST_VERSION=0.9.1 ICEBERG_RUST_BIN_DIR="\$HOME/.local/bin" ./${SCRIPT_NAME}
USAGE
}

log() { printf '[%s] %s\n' "${SCRIPT_NAME}" "$*" >&2; }
die() {
  printf '[%s] ERROR: %s\n' "${SCRIPT_NAME}" "$*" >&2
  exit 1
}

is_truthy() {
  case "${1:-}" in
  1 | true | TRUE | yes | YES | y | Y) return 0 ;;
  *) return 1 ;;
  esac
}

have() { command -v "$1" >/dev/null 2>&1; }

while [[ $# -gt 0 ]]; do
  case "$1" in
  --version)
    VERSION="${2:?missing value for --version}"
    shift 2
    ;;
  --dist-base)
    APACHE_DIST_BASE="${2:?missing value for --dist-base}"
    shift 2
    ;;
  --keys-url)
    APACHE_KEYS_URL="${2:?missing value for --keys-url}"
    shift 2
    ;;
  --prefix)
    PREFIX="${2:?missing value for --prefix}"
    SOURCE_ROOT="${ICEBERG_RUST_SOURCE_ROOT:-${PREFIX}/opt/iceberg-rust}"
    BIN_DIR="${ICEBERG_RUST_BIN_DIR:-${PREFIX}/bin}"
    shift 2
    ;;
  --source-root)
    SOURCE_ROOT="${2:?missing value for --source-root}"
    shift 2
    ;;
  --bin-dir)
    BIN_DIR="${2:?missing value for --bin-dir}"
    shift 2
    ;;
  --package)
    PACKAGE="${2:?missing value for --package}"
    shift 2
    ;;
  --bin)
    BIN_NAME="${2:?missing value for --bin}"
    shift 2
    ;;
  --source-only)
    SOURCE_ONLY=1
    shift
    ;;
  --skip-gpg)
    SKIP_GPG=1
    shift
    ;;
  --skip-sha512 | --skip-sha)
    SKIP_SHA512=1
    shift
    ;;
  --no-locked)
    LOCKED=0
    shift
    ;;
  --force)
    FORCE=1
    shift
    ;;
  --keep-workdir)
    KEEP_WORKDIR=1
    shift
    ;;
  -h | --help)
    usage
    exit 0
    ;;
  *) die "unknown option: $1" ;;
  esac
done

APACHE_DIST_BASE="${APACHE_DIST_BASE%/}"
VERSION="${VERSION#v}"

if [[ -n "${BIN_NAME}" && -z "${PACKAGE}" ]]; then
  die "--bin requires --package so cargo knows which workspace member owns the binary"
fi

if is_truthy "${SKIP_GPG}" && is_truthy "${SKIP_SHA512}"; then
  die "refusing to install with both GPG and SHA-512 verification disabled"
fi

fetch_stdout() {
  local url="$1"

  if have curl; then
    curl --fail --silent --show-error --location --proto '=https' --tlsv1.2 "$url"
  elif have wget; then
    wget -qO- "$url"
  else
    die "curl or wget is required"
  fi
}

fetch_file() {
  local url="$1"
  local dest="$2"

  log "Downloading ${url}"
  if have curl; then
    curl --fail --show-error --location --proto '=https' --tlsv1.2 --retry 3 --output "$dest" "$url"
  elif have wget; then
    wget -O "$dest" "$url"
  else
    die "curl or wget is required"
  fi
}

version_gt() {
  local left="${1#v}"
  local right="${2#v}"
  local old_ifs="${IFS}"
  local i av bv
  local -a a b

  IFS='.' read -r -a a <<<"$left"
  IFS='.' read -r -a b <<<"$right"
  IFS="${old_ifs}"

  for i in 0 1 2 3; do
    av="${a[$i]:-0}"
    bv="${b[$i]:-0}"
    av="${av%%[^0-9]*}"
    bv="${bv%%[^0-9]*}"
    av="${av:-0}"
    bv="${bv:-0}"

    if ((10#${av} > 10#${bv})); then return 0; fi
    if ((10#${av} < 10#${bv})); then return 1; fi
  done

  return 1
}

latest_release_version() {
  local html match version latest=""

  html="$(fetch_stdout "${APACHE_DIST_BASE}/")"
  while IFS= read -r match; do
    version="${match#apache-iceberg-rust-}"
    version="${version%/}"
    if [[ -z "${latest}" ]] || version_gt "${version}" "${latest}"; then
      latest="${version}"
    fi
  done < <(printf '%s\n' "$html" | grep -Eo 'apache-iceberg-rust-[0-9]+([.][0-9]+){1,3}/' || true)

  [[ -n "${latest}" ]] || die "could not determine latest apache-iceberg-rust release from ${APACHE_DIST_BASE}/"
  printf '%s\n' "${latest}"
}

sha512_digest() {
  local file="$1"

  if have shasum; then
    shasum -a 512 "$file" | awk '{print tolower($1)}'
  elif have sha512sum; then
    sha512sum "$file" | awk '{print tolower($1)}'
  elif have openssl; then
    openssl dgst -sha512 "$file" | awk '{print tolower($NF)}'
  elif have python3; then
    python3 - "$file" <<'PY'
import hashlib
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
h = hashlib.sha512()
with path.open("rb") as f:
    for chunk in iter(lambda: f.read(1024 * 1024), b""):
        h.update(chunk)
print(h.hexdigest())
PY
  else
    die "shasum, sha512sum, openssl, or python3 is required for SHA-512 verification"
  fi
}

verify_sha512() {
  local tarball="$1"
  local sha_file="$2"
  local expected actual

  expected="$(awk 'match($0, /[[:xdigit:]]{128}/) { print tolower(substr($0, RSTART, RLENGTH)); exit }' "$sha_file")"
  [[ -n "${expected}" ]] || die "could not parse SHA-512 digest from ${sha_file}"

  actual="$(sha512_digest "$tarball")"
  [[ "${actual}" == "${expected}" ]] || die "SHA-512 mismatch for ${tarball}"

  log "SHA-512 verified: $(basename "$tarball")"
}

verify_gpg() {

  local tarball="$1"
  local asc_file="$2"
  local keys_file="$3"
  local gpg_home="$4"
  local keyring status_file

  have gpg || die "gpg is required to prepare the ASF KEYS keyring; install gnupg or pass --skip-gpg"
  have gpgv || die "gpgv is required for agent-free signature verification; install gnupg or pass --skip-gpg"

  mkdir -p "$gpg_home"
  chmod 700 "$gpg_home"

  keyring="${gpg_home}/apache-iceberg-rust-trustedkeys.gpg"
  status_file="${gpg_home}/gpgv.status"

  # Avoid `gpg --import`: importing the ASF KEYS file can try to contact
  # gpg-agent once per key on some macOS setups. We only need a temporary
  # verification keyring, so dearmor the public KEYS file and verify with gpgv.

  gpg \
    --no-options \
    --batch \
    --yes \
    --output "$keyring" \
    --dearmor "$keys_file"

  if ! gpgv \
    --keyring "$keyring" \
    --status-fd 3 \
    "$asc_file" \
    "$tarball" \
    3>"$status_file"; then
    die "OpenPGP signature verification failed for ${tarball}"
  fi

  grep -q '^\[GNUPG:\] VALIDSIG ' "$status_file" ||
    die "OpenPGP verification completed but did not report a valid signature"
  log "OpenPGP signature verified with gpgv: $(basename "$asc_file")"

}

run_privileged() {
  local err_file status

  if [[ "${EUID}" -eq 0 ]]; then
    "$@"
    return
  fi

  err_file="${WORKDIR}/privileged.err"
  if "$@" 2>"$err_file"; then
    rm -f "$err_file"
    return
  fi

  status=$?
  if have sudo; then
    cat "$err_file" >&2 || true
    rm -f "$err_file"
    log "Retrying with sudo: $*"
    sudo "$@"
  else
    cat "$err_file" >&2 || true
    rm -f "$err_file"
    return "$status"
  fi
}

package_name_from_manifest() {
  local manifest="$1"

  awk '
    /^[[:space:]]*\[package\][[:space:]]*$/ { in_pkg=1; next }
    /^[[:space:]]*\[/ { in_pkg=0 }
    in_pkg && /^[[:space:]]*name[[:space:]]*=/ {
      line=$0
      sub(/^[^=]*=/, "", line)
      gsub(/[[:space:]"]/, "", line)
      print line
      exit
    }
  ' "$manifest"
}

manifest_has_binary_target() {
  local manifest="$1"
  local dir="${manifest%/Cargo.toml}"

  [[ -f "${dir}/src/main.rs" ]] && return 0

  if [[ -d "${dir}/src/bin" ]] && find "${dir}/src/bin" -type f -name '*.rs' | grep -q .; then
    return 0
  fi

  if grep -Eq '^[[:space:]]*\[\[bin\]\]' "$manifest"; then
    return 0
  fi

  return 1
}

find_package_dir() {
  local src_dir="$1"
  local wanted="$2"
  local manifest name

  while IFS= read -r manifest; do
    name="$(package_name_from_manifest "$manifest")"
    if [[ "${name}" == "${wanted}" ]]; then
      dirname "$manifest"
      return 0
    fi
  done < <(find "$src_dir" -name Cargo.toml -type f)

  return 1
}

install_source_tree() {
  local src_dir="$1"
  local target="${SOURCE_ROOT}/releases/${VERSION}"
  local current="${SOURCE_ROOT}/current"
  local staging="${WORKDIR}/install-source/apache-iceberg-rust-${VERSION}"

  if [[ -e "${target}" && "${FORCE}" != "1" ]]; then
    log "Verified source already installed at ${target}; use --force to replace it"
  else
    rm -rf "${staging}"
    mkdir -p "${staging}"

    (cd "$src_dir" && tar -cf - .) | (cd "$staging" && tar -xf -)
    printf 'version=%s\nsource=%s\ninstalled_at=%s\n' \
      "${VERSION}" "${APACHE_DIST_BASE}" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
      >"${staging}/.iceberg-rust-install"

    run_privileged mkdir -p "${SOURCE_ROOT}/releases"
    run_privileged rm -rf "${target}"
    run_privileged mv "${staging}" "${target}"

    log "Installed verified source to ${target}"
  fi

  run_privileged mkdir -p "${SOURCE_ROOT}"
  run_privileged rm -rf "${current}"
  run_privileged ln -s "${target}" "${current}"

  log "Updated current source symlink: ${current} -> ${target}"
}

cargo_install_package() {
  local package_dir="$1"
  local cargo_root="$2"
  local args

  args=(install --path "$package_dir" --root "$cargo_root" --force)

  if [[ "${LOCKED}" == "1" ]]; then
    args+=(--locked)
  fi

  if [[ -n "${BIN_NAME}" ]]; then
    args+=(--bin "$BIN_NAME")
  fi

  log "Running cargo ${args[*]}"
  cargo "${args[@]}"
}

install_cargo_binaries() {
  local src_dir="$1"
  local cargo_root="${WORKDIR}/cargo-install-root"
  local manifest name package_dir installed_any=0 copied_any=0 f

  [[ "${SOURCE_ONLY}" == "1" ]] && return 0

  mkdir -p "$cargo_root"

  if [[ -n "${PACKAGE}" ]]; then
    package_dir="$(find_package_dir "$src_dir" "$PACKAGE")" || die "package '${PACKAGE}' not found in verified source"
    manifest="${package_dir}/Cargo.toml"

    manifest_has_binary_target "$manifest" || die "package '${PACKAGE}' has no Cargo binary targets, so cargo install cannot install it"
    have cargo || die "cargo is required to build/install Iceberg Rust binaries; install Rust or pass --source-only"

    cargo_install_package "$package_dir" "$cargo_root"
    installed_any=1
  else
    while IFS= read -r manifest; do
      name="$(package_name_from_manifest "$manifest")"
      [[ -n "${name}" ]] || continue

      if manifest_has_binary_target "$manifest"; then
        have cargo || die "cargo is required to build/install Iceberg Rust binaries; install Rust or pass --source-only"
        cargo_install_package "$(dirname "$manifest")" "$cargo_root"
        installed_any=1
      fi
    done < <(find "$src_dir" -name Cargo.toml -type f)
  fi

  if [[ "${installed_any}" != "1" ]]; then
    log "No Cargo binary packages found in the verified source tree; source install only."
    return 0
  fi

  run_privileged mkdir -p "$BIN_DIR"

  if [[ -d "${cargo_root}/bin" ]]; then
    for f in "${cargo_root}"/bin/*; do
      [[ -e "$f" ]] || continue
      copied_any=1
      run_privileged install -m 0755 "$f" "${BIN_DIR}/$(basename "$f")"
      log "Installed binary: ${BIN_DIR}/$(basename "$f")"
    done
  fi

  [[ "${copied_any}" == "1" ]] || die "cargo install completed but no binaries were produced under ${cargo_root}/bin"
}

WORKDIR="$(mktemp -d "${TMPDIR:-/tmp}/iceberg-rust-install.XXXXXX")"

cleanup() {
  if [[ "${KEEP_WORKDIR}" == "1" ]]; then
    log "Keeping workdir: ${WORKDIR}"
  else
    rm -rf "$WORKDIR"
  fi
}
trap cleanup EXIT

if [[ "${VERSION}" == "latest" ]]; then
  VERSION="$(latest_release_version)"
fi

ARTIFACT="apache-iceberg-rust-${VERSION}.tar.gz"
RELEASE_DIR_URL="${APACHE_DIST_BASE}/apache-iceberg-rust-${VERSION}"
TARBALL_URL="${RELEASE_DIR_URL}/${ARTIFACT}"
ASC_URL="${TARBALL_URL}.asc"
SHA512_URL="${TARBALL_URL}.sha512"
DOWNLOAD_DIR="${WORKDIR}/downloads"
EXTRACT_DIR="${WORKDIR}/extract"

mkdir -p "$DOWNLOAD_DIR" "$EXTRACT_DIR"

log "Installing Apache Iceberg Rust ${VERSION}"
fetch_file "$TARBALL_URL" "${DOWNLOAD_DIR}/${ARTIFACT}"

if ! is_truthy "${SKIP_SHA512}"; then
  fetch_file "$SHA512_URL" "${DOWNLOAD_DIR}/${ARTIFACT}.sha512"
  verify_sha512 "${DOWNLOAD_DIR}/${ARTIFACT}" "${DOWNLOAD_DIR}/${ARTIFACT}.sha512"
else
  log "Skipping SHA-512 verification by request"
fi

if ! is_truthy "${SKIP_GPG}"; then
  fetch_file "$ASC_URL" "${DOWNLOAD_DIR}/${ARTIFACT}.asc"
  fetch_file "$APACHE_KEYS_URL" "${DOWNLOAD_DIR}/KEYS"
  verify_gpg "${DOWNLOAD_DIR}/${ARTIFACT}" "${DOWNLOAD_DIR}/${ARTIFACT}.asc" "${DOWNLOAD_DIR}/KEYS" "${WORKDIR}/gnupg"
else
  log "Skipping OpenPGP verification by request"
fi

log "Extracting ${ARTIFACT}"
tar -xzf "${DOWNLOAD_DIR}/${ARTIFACT}" -C "$EXTRACT_DIR"

SRC_DIR=""
for candidate in "$EXTRACT_DIR"/*; do
  if [[ -d "$candidate" ]]; then
    SRC_DIR="$candidate"
    break
  fi
done
[[ -n "${SRC_DIR}" ]] || die "source archive did not contain a top-level directory"

install_source_tree "$SRC_DIR"
install_cargo_binaries "$SRC_DIR"

log "Done. Verified source: ${SOURCE_ROOT}/current"
if [[ "${SOURCE_ONLY}" != "1" ]]; then
  log "Binaries, if any, were installed to: ${BIN_DIR}"
fi
