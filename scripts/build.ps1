$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..')
$targetOS = if ($env:GOOS) { $env:GOOS } else { go env GOOS }
$targetArch = if ($env:GOARCH) { $env:GOARCH } else { go env GOARCH }
if ($targetOS -notin @('darwin', 'linux', 'windows')) { throw 'Only desktop targets are supported' }
$outputDir = Join-Path (Get-Location) "build/$targetOS-$targetArch"
New-Item -ItemType Directory -Force $outputDir | Out-Null
if (!(Test-Path core/Clash.Meta/go.mod)) { git submodule update --init --recursive }
$env:CGO_ENABLED = '0'
$env:GOOS = $targetOS
$env:GOARCH = $targetArch
$suffix = if ($targetOS -eq 'windows') { '.exe' } else { '' }
go build -trimpath -tags mygo_noinspector -ldflags '-s -w -X github.com/egoist/mygo.packageName=FlClash -X github.com/egoist/mygo.packageIdentifier=com.flclash.mygo -X github.com/egoist/mygo.packageURLSchemes=flclash-mygo,flclash,clash,clashmeta -X github.com/egoist/mygo.packageFileExtensions=yaml,yml' -o "$outputDir/flclash$suffix" ./cmd/flclash
if ($LASTEXITCODE -ne 0) { throw 'UI build failed' }
Push-Location core
try {
    go build -trimpath -tags with_gvisor -ldflags '-s -w' -o "$outputDir/flclash-core$suffix" ./cmd/core
    if ($LASTEXITCODE -ne 0) { throw 'Core build failed' }
} finally { Pop-Location }
Copy-Item LICENSE "$outputDir/LICENSE"
Copy-Item THIRD_PARTY_NOTICES "$outputDir/THIRD_PARTY_NOTICES"
Write-Output $outputDir
