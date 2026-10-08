#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd -P)"
downloads="$repo_root/.tools/downloads"
expected_pgdg_fingerprint='B97B0AFCAA1A47F044F244A07FCC7D46ACCC4CF8'
key_url='https://www.postgresql.org/media/keys/ACCC4CF8.asc'

if [[ "$(id -u)" -eq 0 ]]; then
  printf 'postgresql18_download=NOT_RUN\nreason=run_as_unprivileged_user\n'
  exit 2
fi
if [[ ! -r /etc/os-release || ! -r /usr/share/keyrings/ubuntu-archive-keyring.gpg ]]; then
  printf 'postgresql18_download=NOT_RUN\nreason=ubuntu_release_or_archive_keyring_unavailable\n'
  exit 3
fi
source /etc/os-release
if [[ "${ID:-}" != ubuntu || "${VERSION_CODENAME:-}" != jammy || "$(dpkg --print-architecture)" != amd64 ]]; then
  printf 'postgresql18_download=NOT_RUN\nreason=unsupported_platform\nplatform=%s/%s/%s\n' "${ID:-unknown}" "${VERSION_CODENAME:-unknown}" "$(dpkg --print-architecture 2>/dev/null || echo unknown)"
  exit 3
fi
for command in wget gpg apt-get apt-cache dpkg-deb sha256sum; do
  command -v "$command" >/dev/null 2>&1 || { printf 'postgresql18_download=NOT_RUN\nreason=required_tool_missing\ntool=%s\n' "$command"; exit 3; }
done

download_id="$(date -u +%Y%m%dT%H%M%SZ)-$$"
evidence_parent="$repo_root/evidence/development/polis-self-dev-v1-workspace-go-job-patch-slice/postgres-download"
mkdir -p "$evidence_parent"
evidence_dir="$evidence_parent/$download_id"
mkdir -m 0700 "$evidence_dir"

if [[ -d "$downloads" ]] && find "$downloads" -maxdepth 1 -type f -name '*.deb' -print -quit | grep -q .; then
  printf 'postgresql18_download=NOT_RUN\nreason=downloads_directory_already_contains_packages\n' >"$evidence_dir/summary.txt"
  printf 'evidence_dir=%s\n' "$evidence_dir"
  exit 2
fi

temp_root="$(mktemp -d /tmp/polis-pg18-download.XXXXXX)"
temp_root="$(cd "$temp_root" && pwd -P)"
case "$temp_root" in
  /tmp/polis-pg18-download.*) ;;
  *) printf 'unsafe temporary apt root\n' >&2; exit 1 ;;
esac
trap 'status=$?; if [[ "$temp_root" == /tmp/polis-pg18-download.* && ! -L "$temp_root" ]]; then rm -rf -- "$temp_root"; fi; exit "$status"' EXIT

lists="$temp_root/lists"
archives="$temp_root/archives"
cache="$temp_root/cache"
mkdir -m 0755 "$lists" "$archives" "$cache"
mkdir -m 0777 "$lists/partial" "$archives/partial"
mkdir -m 0700 "$temp_root/gnupg"
export GNUPGHOME="$temp_root/gnupg"
cp /var/lib/dpkg/status "$temp_root/status"
if ! wget --https-only --timeout=15 --tries=2 --output-document="$temp_root/apt.postgresql.org.asc" "$key_url" >"$evidence_dir/key-download.log" 2>&1; then
  printf 'postgresql18_download=FAIL\nreason=official_pgdg_key_download_failed\n' >"$evidence_dir/summary.txt"
  exit 4
fi
fingerprint="$(gpg --batch --show-keys --with-colons "$temp_root/apt.postgresql.org.asc" | awk -F: '$1=="fpr" {print $10; exit}')"
if [[ "$fingerprint" != "$expected_pgdg_fingerprint" ]]; then
  printf 'postgresql18_download=NOT_RUN\nreason=pgdg_signing_key_fingerprint_mismatch\n' | tee "$evidence_dir/summary.txt"
  exit 4
fi
gpg --batch --yes --dearmor --output "$temp_root/pgdg.gpg" "$temp_root/apt.postgresql.org.asc"
sources="$temp_root/pg18.sources"
cat >"$sources" <<SOURCES
Types: deb
URIs: https://archive.ubuntu.com/ubuntu
Suites: jammy jammy-updates
Components: main universe
Signed-By: /usr/share/keyrings/ubuntu-archive-keyring.gpg

Types: deb
URIs: https://security.ubuntu.com/ubuntu
Suites: jammy-security
Components: main universe
Signed-By: /usr/share/keyrings/ubuntu-archive-keyring.gpg

Types: deb
URIs: https://apt.postgresql.org/pub/repos/apt
Suites: jammy-pgdg
Components: main
Signed-By: $temp_root/pgdg.gpg
SOURCES
chmod 0600 "$sources" "$temp_root/status"

apt_options=(
  -o "Dir::Etc::sourcelist=$sources"
  -o 'Dir::Etc::sourceparts=-'
  -o "Dir::State::lists=$lists"
  -o "Dir::State::status=$temp_root/status"
  -o "Dir::Cache::archives=$archives"
  -o "Dir::Cache::pkgcache=$cache/pkgcache.bin"
  -o "Dir::Cache::srcpkgcache=$cache/srcpkgcache.bin"
  -o "APT::Sandbox::User=$(id -un)"
)

if ! apt-get "${apt_options[@]}" update >"$evidence_dir/apt-update.log" 2>&1; then
  printf 'postgresql18_download=FAIL\nreason=official_signed_apt_metadata_update_failed\n' >"$evidence_dir/summary.txt"
  exit 5
fi
apt_policy="$(apt-cache "${apt_options[@]}" policy postgresql-18 2>"$evidence_dir/apt-cache.log" || true)"
printf '%s\n' "$apt_policy" >"$evidence_dir/apt-policy.txt"
candidate="$(awk '/Candidate:/ {print $2; exit}' <<<"$apt_policy")"
case "$candidate" in
  ''|\(none\))
    printf 'postgresql18_download=NOT_RUN\nreason=postgresql18_not_in_signed_pgdg_index\n' >"$evidence_dir/summary.txt"
    exit 6
    ;;
esac
if ! apt-get "${apt_options[@]}" --download-only --no-install-recommends --no-upgrade --yes install postgresql-18 postgresql-client-18 >"$evidence_dir/apt-download.log" 2>&1; then
  printf 'postgresql18_download=FAIL\nreason=official_signed_package_download_failed\n' >"$evidence_dir/summary.txt"
  exit 7
fi

deb_files=("$archives"/*.deb)
if ((${#deb_files[@]} == 0)) || [[ ! -f "${deb_files[0]}" ]]; then
  printf 'postgresql18_download=FAIL\nreason=apt_download_returned_no_debian_packages\n' >"$evidence_dir/summary.txt"
  exit 8
fi
mkdir -p "$downloads"
for package in "${deb_files[@]}"; do
  filename="$(basename "$package")"
  destination="$downloads/$filename"
  if [[ -e "$destination" || -L "$destination" ]]; then
    printf 'postgresql18_download=FAIL\nreason=refusing_to_overwrite_existing_package\npackage=%s\n' "$filename" >"$evidence_dir/summary.txt"
    exit 9
  fi
done
for package in "${deb_files[@]}"; do
  install -m 0644 "$package" "$downloads/$(basename "$package")"
done
sha256sum "${downloads}"/*.deb >"$evidence_dir/package-sha256.txt"
printf 'platform=ubuntu-jammy-amd64\nrepository=https://apt.postgresql.org/pub/repos/apt\npgdg_key_fingerprint=%s\npostgresql18_candidate=%s\npackage_count=%s\ndownloaded_to=%s\n' \
  "$fingerprint" "$candidate" "${#deb_files[@]}" "$downloads" >"$evidence_dir/summary.txt"

if ! bash "$repo_root/scripts/prepare-pg.sh" >"$evidence_dir/prepare-pg.log" 2>&1; then
  printf 'postgresql18_extract=FAIL\n' >>"$evidence_dir/summary.txt"
  printf 'evidence_dir=%s\n' "$evidence_dir"
  exit 10
fi
printf 'postgresql18_extract=PASS\n' >>"$evidence_dir/summary.txt"
printf 'evidence_dir=%s\n' "$evidence_dir"
