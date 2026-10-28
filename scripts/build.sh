#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
task_os="${GOOS:-$(go env GOOS)}"
task_arch="${GOARCH:-$(go env GOARCH)}"
case "$task_os" in darwin|linux|windows) ;; *) echo "Desktop targets are darwin, linux, and windows" >&2; exit 1 ;; esac
task_out="${OUT_DIR:-$PWD/build/$task_os-$task_arch}"
mkdir -p "$task_out"
task_suffix=""
if [ "$task_os" = windows ]; then task_suffix=.exe; fi
if [ ! -f core/Clash.Meta/go.mod ]; then git submodule update --init --recursive; fi
export CGO_ENABLED=0 GOOS="$task_os" GOARCH="$task_arch"
go build -trimpath -tags mygo_noinspector -ldflags '-s -w -X github.com/egoist/mygo.packageName=FlClash -X github.com/egoist/mygo.packageIdentifier=com.flclash.mygo -X github.com/egoist/mygo.packageURLSchemes=flclash-mygo,flclash,clash,clashmeta -X github.com/egoist/mygo.packageFileExtensions=yaml,yml' -o "$task_out/flclash$task_suffix" ./cmd/flclash
(cd core && go build -trimpath -tags with_gvisor -ldflags '-s -w' -o "$task_out/flclash-core$task_suffix" ./cmd/core)
cp LICENSE "$task_out/LICENSE"
cp THIRD_PARTY_NOTICES "$task_out/THIRD_PARTY_NOTICES"
echo "$task_out"
