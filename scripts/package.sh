#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
task_os="${GOOS:-$(go env GOOS)}"
task_arch="${GOARCH:-$(go env GOARCH)}"
task_source="$PWD/build/$task_os-$task_arch"
mkdir -p dist
if [ "$task_os" = darwin ]; then
  task_app="$task_source/FlClash MyGo.app"
  mkdir -p "$task_app/Contents/MacOS" "$task_app/Contents/Resources"
  cp "$task_source/flclash" "$task_source/flclash-core" "$task_app/Contents/MacOS/"
  cp LICENSE THIRD_PARTY_NOTICES "$task_app/Contents/Resources/"
  cat > "$task_app/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleIdentifier</key><string>com.flclash.mygo</string>
<key>CFBundleName</key><string>FlClash MyGo</string>
<key>CFBundleExecutable</key><string>flclash</string>
<key>CFBundlePackageType</key><string>APPL</string>
<key>CFBundleShortVersionString</key><string>0.1.0</string>
<key>LSMinimumSystemVersion</key><string>12.0</string>
<key>NSHighResolutionCapable</key><true/>
<key>CFBundleURLTypes</key><array><dict><key>CFBundleURLName</key><string>FlClash configuration</string><key>CFBundleURLSchemes</key><array><string>flclash-mygo</string><string>flclash</string><string>clash</string><string>clashmeta</string></array></dict></array>
</dict></plist>
PLIST
  (cd "$task_source" && zip -qr "$OLDPWD/dist/FlClash-MyGo-$task_os-$task_arch.zip" 'FlClash MyGo.app')
elif [ "$task_os" = windows ]; then
  (cd "$task_source" && zip -q "$OLDPWD/dist/FlClash-MyGo-$task_os-$task_arch.zip" flclash.exe flclash-core.exe LICENSE THIRD_PARTY_NOTICES)
else
  cp assets/icon.png "$task_source/flclash.png"
  cp scripts/flclash-mygo.desktop "$task_source/"
  tar -C "$task_source" -czf "dist/FlClash-MyGo-$task_os-$task_arch.tar.gz" flclash flclash-core flclash.png flclash-mygo.desktop LICENSE THIRD_PARTY_NOTICES
fi
