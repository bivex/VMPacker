#!/usr/bin/env bash
# build_stub32_unix.sh — Unix/macOS compatible ARM32 blob builder
# Replaces PowerShell logic: extracts symbol offsets from ELF, prepends 12-byte header

set -e

ELF="$1"
RAW_BIN="$2"
OUT_BIN="$3"
NM_CMD="${4:-arm-linux-gnueabihf-nm}"

if [ $# -lt 3 ]; then
  echo "Usage: $0 <elf_file> <raw_binary> <output_blob> [nm_cmd]"
  exit 1
fi

get_symbol() {
  $NM_CMD "$ELF" 2>/dev/null | awk -v sym="$1" '$3 == sym {print $1; exit}'
}

off1=$(get_symbol "vm_entry")
off2=$(get_symbol "vm_entry_token")
off3=$(get_symbol "_token_table_va")

if [ -z "$off1" ]; then echo "Error: vm_entry not found in $ELF"; exit 1; fi
if [ -z "$off2" ]; then echo "Error: vm_entry_token not found in $ELF"; exit 1; fi
if [ -z "$off3" ]; then echo "Error: _token_table_va not found in $ELF"; exit 1; fi

# Use Python for reliable little-endian uint32 + binary concatenation
python3 - "$ELF" "$RAW_BIN" "$OUT_BIN" "$off1" "$off2" "$off3" << 'PY'
import sys, struct
_, _elf, raw_path, out_path, o1, o2, o3 = sys.argv
off1 = int(o1, 16)
off2 = int(o2, 16)
off3 = int(o3, 16)
hdr = struct.pack('<III', off1, off2, off3)
with open(raw_path, 'rb') as f:
    raw = f.read()
with open(out_path, 'wb') as f:
    f.write(hdr + raw)
print(f"[+] vm_interp_arm32.bin: {len(hdr)+len(raw)} bytes (vm_entry=0x{off1:X} vm_entry_token=0x{off2:X} _token_table_va=0x{off3:X})")
PY
