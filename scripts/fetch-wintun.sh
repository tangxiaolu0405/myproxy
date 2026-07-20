#!/usr/bin/env bash
# 下载官方 Wintun，供 Windows TUN 模式使用。
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT_DIR="${ROOT}/third_party/wintun"
ZIP_URL="${WINTUN_URL:-https://www.wintun.net/builds/wintun-0.14.1.zip}"
TMP="$(mktemp -d)"
cleanup() { rm -rf "$TMP"; }
trap cleanup EXIT

mkdir -p "$OUT_DIR"
if [[ -f "$OUT_DIR/wintun.dll" ]]; then
  echo "wintun.dll already present: $OUT_DIR/wintun.dll"
  exit 0
fi

echo "Downloading $ZIP_URL ..."
curl -fsSL "$ZIP_URL" -o "$TMP/wintun.zip"

python3 - <<PY
import zipfile, pathlib, sys
z = zipfile.ZipFile("$TMP/wintun.zip")
out = pathlib.Path("$TMP/extract")
out.mkdir(parents=True, exist_ok=True)
z.extractall(out)
dlls = list(out.rglob("wintun.dll"))
if not dlls:
    sys.exit("ERROR: wintun.dll not found in archive")
# Prefer amd64 if multiple
prefer = [p for p in dlls if "amd64" in str(p).lower() or "x86_64" in str(p).lower()]
src = prefer[0] if prefer else dlls[0]
dest = pathlib.Path("$OUT_DIR/wintun.dll")
dest.write_bytes(src.read_bytes())
print(f"Installed {dest} (from {src})")
PY
