#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
: "${WASI_SDK:?Set WASI_SDK to the unpacked wasi-sdk-34.0 directory}"
"$WASI_SDK/bin/clang" --sysroot="$WASI_SDK/share/wasi-sysroot" --target=wasm32-wasip1 -mexec-model=reactor -O2 -D_GNU_SOURCE \
  -Ithird_party/quickjs -x c internal/script/source/evaluate.c \
  third_party/quickjs/quickjs.c third_party/quickjs/libregexp.c \
  third_party/quickjs/libunicode.c third_party/quickjs/dtoa.c \
  -Wl,--export=malloc -Wl,--export=free -Wl,--export=flclash_evaluate \
  -Wl,--export=flclash_result_size -Wl,-z,stack-size=1048576 \
  -Wl,--max-memory=285212672 -Wl,--strip-all -lm -o internal/script/quickjs.wasm
