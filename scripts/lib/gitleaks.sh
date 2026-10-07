#!/usr/bin/env bash
# Installs Gitleaks v8.30.0 into a user cache after verifying a pinned SHA-256.
# v8.30.1 is deliberately avoided: its published build has a known
# false-negative regression.
#
# Usage (sourced): gitleaks_bin=$(ensure_gitleaks)

GITLEAKS_VERSION=8.30.0

gitleaks_expected_sha256() {
  case "$1" in
    darwin_arm64) echo b251ab2bcd4cd8ba9e56ff37698c033ebf38582b477d21ebd86586d927cf87e7 ;;
    darwin_x64) echo ca221d012d247080c2f6f61f4b7a83bffa2453806b0c195c795bbe9a8c775ed5 ;;
    linux_arm64) echo b4cbbb6ddf7d1b2a603088cd03a4e3f7ce48ee7fd449b51f7de6ee2906f5fa2f ;;
    linux_x64) echo 79a3ab579b53f71efd634f3aaf7e04a0fa0cf206b7ed434638d1547a2470a66e ;;
    *) return 1 ;;
  esac
}

sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

ensure_gitleaks() {
  local os arch platform expected cache_dir archive binary actual
  case "$(uname -s)" in
    Darwin) os=darwin ;;
    Linux) os=linux ;;
    *) echo "unsupported OS for gitleaks: $(uname -s)" >&2; return 1 ;;
  esac
  case "$(uname -m)" in
    arm64|aarch64) arch=arm64 ;;
    x86_64|amd64) arch=x64 ;;
    *) echo "unsupported architecture for gitleaks: $(uname -m)" >&2; return 1 ;;
  esac
  platform=${os}_${arch}
  expected=$(gitleaks_expected_sha256 "$platform") || {
    echo "no pinned gitleaks checksum for $platform" >&2
    return 1
  }

  cache_dir=${XDG_CACHE_HOME:-$HOME/.cache}/spotify-release-tools/gitleaks-$GITLEAKS_VERSION-$platform
  binary=$cache_dir/gitleaks
  archive=$cache_dir/gitleaks.tar.gz

  if [[ ! -x "$binary" ]]; then
    mkdir -p "$cache_dir"
    curl -fsSL -o "$archive" \
      "https://github.com/gitleaks/gitleaks/releases/download/v$GITLEAKS_VERSION/gitleaks_${GITLEAKS_VERSION}_${platform}.tar.gz"
    actual=$(sha256_of "$archive")
    if [[ "$actual" != "$expected" ]]; then
      rm -f "$archive"
      echo "gitleaks checksum mismatch for $platform" >&2
      return 1
    fi
    tar -xzf "$archive" -C "$cache_dir" gitleaks
    rm -f "$archive"
  fi

  "$binary" version 2>/dev/null | grep -q "$GITLEAKS_VERSION" || {
    echo "cached gitleaks is not v$GITLEAKS_VERSION" >&2
    return 1
  }
  printf '%s\n' "$binary"
}

# scan_secrets <gitleaks-binary> <directory>
# Prints file, line, and rule for each finding; secret values are fully redacted.
scan_secrets() {
  "$1" dir "$2" --no-banner --redact=100 --exit-code 1 --verbose --log-level warn
}
