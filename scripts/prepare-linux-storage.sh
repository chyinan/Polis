#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
root="$PWD"
image="$root/.runtime/linux.ext4"
target="$root/.runtime/linux"
mkdir -p "$target"
if [ ! -f "$image" ]; then
  truncate -s 4G "$image"
  mkfs.ext4 -q -F "$image"
fi
if ! mountpoint -q "$target"; then mount -o loop "$image" "$target"; fi
chown 1000:1000 "$target"
chmod 700 "$target"
