#!/usr/bin/env bash
#
# protect_and_repack.sh — Replace .so in APK with VMPacker-protected versions, re-sign
#
# Strategy: copy APK, replace .so files in-place (preserves original zip structure
# including uncompressed resources.arsc), then re-sign.
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
BUILD_DIR="$PROJECT_DIR/build"
APK_IN="$PROJECT_DIR/app/build/outputs/apk/debug/app-debug.apk"
APK_OUT="$BUILD_DIR/app-protected.apk"
VMPACKER="go run $PROJECT_DIR/../../cmd/vmpacker/"
SO_FUNCS="vmp_compute,vmp_verify_key,vmp_md5_hex,vmp_get_process_name"
SO_NAME="libnative_test.so"

WORK="$BUILD_DIR/apk_work"
rm -rf "$WORK"
mkdir -p "$WORK" "$BUILD_DIR"

echo "[1/5] Extracting .so files from APK..."
for abi in arm64-v8a armeabi-v7a; do
    so_path="lib/$abi/$SO_NAME"
    if unzip -l "$APK_IN" "$so_path" &>/dev/null; then
        mkdir -p "$WORK/lib/$abi"
        unzip -o -q "$APK_IN" "$so_path" -d "$WORK"
    fi
done

echo "[2/5] Protecting native libraries..."
for abi_dir in "$WORK"/lib/*/; do
    abi=$(basename "$abi_dir")
    so_file="$abi_dir/$SO_NAME"
    if [ -f "$so_file" ]; then
        echo "  -> Protecting $abi/$SO_NAME"
        $VMPACKER -func "$SO_FUNCS" -v \
            -o "${so_file}.protected" "$so_file"
        mv "${so_file}.protected" "$so_file"
    fi
done

echo "[3/5] Copying APK and replacing .so files..."
cp "$APK_IN" "$APK_OUT"

# Remove old signature
zip -d "$APK_OUT" "META-INF/*" 2>/dev/null || true

# Replace .so files in the APK (stored uncompressed with -0 to match original)
for abi_dir in "$WORK"/lib/*/; do
    abi=$(basename "$abi_dir")
    so_file="$abi_dir/$SO_NAME"
    if [ -f "$so_file" ]; then
        echo "  -> Replacing $abi/$SO_NAME in APK"
        (cd "$WORK" && zip -0 "$APK_OUT" "lib/$abi/$SO_NAME")
    fi
done

echo "[4/5] Aligning APK..."
if command -v zipalign &>/dev/null; then
    zipalign -f -p 4 "$APK_OUT" "${APK_OUT}.aligned"
    mv "${APK_OUT}.aligned" "$APK_OUT"
else
    ZIPALIGN_PATH=$(find "$ANDROID_HOME/build-tools" -name "zipalign" 2>/dev/null | sort -V | tail -1)
    if [ -n "$ZIPALIGN_PATH" ]; then
        "$ZIPALIGN_PATH" -f -p 4 "$APK_OUT" "${APK_OUT}.aligned"
        mv "${APK_OUT}.aligned" "$APK_OUT"
    else
        echo "  [WARN] zipalign not found, skipping alignment"
    fi
fi

echo "[5/5] Signing APK..."
DEBUG_KEYSTORE="$HOME/.android/debug.keystore"
if [ -f "$DEBUG_KEYSTORE" ]; then
    APKSIGNER_PATH=$(find "$ANDROID_HOME/build-tools" -name "apksigner" 2>/dev/null | sort -V | tail -1)
    if [ -n "$APKSIGNER_PATH" ]; then
        "$APKSIGNER_PATH" sign --ks "$DEBUG_KEYSTORE" --ks-pass pass:android "$APK_OUT"
    elif command -v apksigner &>/dev/null; then
        apksigner sign --ks "$DEBUG_KEYSTORE" --ks-pass pass:android "$APK_OUT"
    elif command -v jarsigner &>/dev/null; then
        jarsigner -keystore "$DEBUG_KEYSTORE" -storepass android \
            -signedjar "$APK_OUT" "$APK_OUT" androiddebugkey
    else
        echo "  [WARN] No signing tool found, APK is unsigned"
    fi
else
    echo "  [WARN] Debug keystore not found at $DEBUG_KEYSTORE, APK unsigned"
fi

rm -rf "$WORK"
echo ""
echo "[DONE] Protected APK: $APK_OUT"
