<#
.SYNOPSIS
    Builds PDF Scanner with the Wails CLI, including the checks and the mingw-w64
    workaround a clean checkout needs.

.DESCRIPTION
    `wails build` is the only working entry point (see CLAUDE.md "Building"), but it needs
    a few things arranged around it:

      1. go / node / npm / wails / gcc on PATH, and Wails CLI >= 2.15 (2.12 cannot
         generate bindings under Go 1.27).
      2. The Go-only quality gates: gofmt, go vet, go test ./internal/match/.
      3. CGO_LDFLAGS aliasing __intrinsic_setjmpex to _setjmpex, which go-fitz' prebuilt
         MuPDF needs on mingw-w64 v12+.
      4. A clean build/bin, so stale binaries from an earlier name do not linger.

.PARAMETER Dev
    Run `wails dev` (live reload) instead of a production build.

.PARAMETER SkipChecks
    Skip gofmt / go vet / go test.

.PARAMETER NoClean
    Keep the existing contents of build/bin instead of passing -clean.

.PARAMETER Platform
    Target passed to `wails build`. Defaults to windows/amd64.

.EXAMPLE
    .\build.ps1

.EXAMPLE
    .\build.ps1 -Dev

.EXAMPLE
    powershell -ExecutionPolicy Bypass -File .\build.ps1 -SkipChecks
#>
[CmdletBinding()]
param(
    [switch]$Dev,
    [switch]$SkipChecks,
    [switch]$NoClean,
    [string]$Platform = 'windows/amd64'
)

$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot

function Write-Step($text) {
    Write-Host ''
    Write-Host "==> $text" -ForegroundColor Cyan
}

function Write-Ok($text) {
    Write-Host "    $text" -ForegroundColor DarkGray
}

function Fail($text) {
    Write-Host ''
    Write-Host "!!! $text" -ForegroundColor Red
    exit 1
}

function Assert-ExitCode($what) {
    if ($LASTEXITCODE -ne 0) { Fail "$what failed (exit $LASTEXITCODE)." }
}

# --- 1. Toolchain -----------------------------------------------------------------------

Write-Step 'Checking the toolchain'

$required = @(
    @{ Name = 'go';   Hint = 'Install Go 1.27+ from https://go.dev/dl/' },
    @{ Name = 'node'; Hint = 'Install Node.js 20+ from https://nodejs.org/' },
    @{ Name = 'npm';  Hint = 'Ships with Node.js' },
    @{ Name = 'wails'; Hint = 'go install github.com/wailsapp/wails/v2/cmd/wails@latest (and put %USERPROFILE%\go\bin on PATH)' }
)

foreach ($tool in $required) {
    $cmd = Get-Command $tool.Name -ErrorAction SilentlyContinue
    if ($null -eq $cmd) { Fail "$($tool.Name) not found on PATH. $($tool.Hint)" }
    Write-Ok "$($tool.Name.PadRight(5)) $($cmd.Source)"
}

# gcc is only needed for the cgo half (go-fitz/MuPDF); MSYS2 usually is not on PATH.
$gcc = Get-Command gcc -ErrorAction SilentlyContinue
if ($null -eq $gcc) {
    $probes = @(
        'C:\msys64\mingw64\bin',
        'C:\msys64\ucrt64\bin',
        'C:\mingw64\bin',
        'C:\ProgramData\chocolatey\bin'
    )
    foreach ($probe in $probes) {
        if (Test-Path (Join-Path $probe 'gcc.exe')) {
            $env:PATH = "$probe;$env:PATH"
            Write-Ok "added $probe to PATH for this build"
            break
        }
    }
    $gcc = Get-Command gcc -ErrorAction SilentlyContinue
}
if ($null -eq $gcc) {
    Fail 'gcc not found on PATH. go-fitz is cgo; install MSYS2 mingw-w64 (pacman -S mingw-w64-x86_64-gcc).'
}
Write-Ok "gcc   $($gcc.Source)"

# Wails CLI 2.12 cannot generate bindings under Go 1.27.
$wailsVersion = (& wails version) -join ' '
if ($wailsVersion -match 'v?(\d+)\.(\d+)\.(\d+)') {
    $major = [int]$Matches[1]
    $minor = [int]$Matches[2]
    Write-Ok "wails CLI v$major.$minor.$($Matches[3])"
    if ($major -lt 2 -or ($major -eq 2 -and $minor -lt 15)) {
        Fail "Wails CLI v$major.$minor is too old - v2.15+ is required under Go 1.27. Run: go install github.com/wailsapp/wails/v2/cmd/wails@latest"
    }
} else {
    Write-Ok 'could not parse the Wails CLI version - continuing anyway'
}

# --- 2. Go-only quality gates -----------------------------------------------------------

if ($SkipChecks) {
    Write-Step 'Skipping gofmt / go vet / go test (-SkipChecks)'
} else {
    Write-Step 'Running the Go checks'

    $unformatted = & gofmt -l . | Where-Object { $_ -notmatch '[\/]node_modules[\/]' }
    Assert-ExitCode 'gofmt'
    if ($unformatted) {
        Write-Host ($unformatted -join [Environment]::NewLine) -ForegroundColor Yellow
        Fail 'The files above are not gofmt-clean. Run: gofmt -w .'
    }
    Write-Ok 'gofmt clean'

    & go vet ./internal/...
    Assert-ExitCode 'go vet'
    Write-Ok 'go vet clean'

    & go test ./internal/match/
    Assert-ExitCode 'go test'
}

# --- 3. The mingw-w64 / MuPDF link workaround -------------------------------------------

# go-fitz ships a prebuilt MuPDF referencing __intrinsic_setjmpex, MSVC's name for the CRT's
# _setjmpex, which mingw-w64 v12+ no longer aliases but does ship in libmsvcrt.a.
$defsym = '-Wl,--defsym=__intrinsic_setjmpex=_setjmpex'
if ($env:CGO_LDFLAGS -notlike "*$defsym*") {
    if ([string]::IsNullOrWhiteSpace($env:CGO_LDFLAGS)) {
        $env:CGO_LDFLAGS = $defsym
    } else {
        $env:CGO_LDFLAGS = "$env:CGO_LDFLAGS $defsym"
    }
}
Write-Step "CGO_LDFLAGS = $env:CGO_LDFLAGS"

# --- 4. Build (or dev) ------------------------------------------------------------------

$config = Get-Content 'wails.json' -Raw | ConvertFrom-Json

if ($Dev) {
    Write-Step 'wails dev'
    & wails dev
    Assert-ExitCode 'wails dev'
    exit 0
}

$wailsArgs = @('build', '-platform', $Platform)
if (-not $NoClean) { $wailsArgs += '-clean' }

Write-Step "wails $($wailsArgs -join ' ')"
& wails @wailsArgs
Assert-ExitCode 'wails build'

# --- 5. Report --------------------------------------------------------------------------

$exe = Join-Path $PSScriptRoot ('build\bin\{0}.exe' -f $config.outputfilename)
if (-not (Test-Path $exe)) {
    Fail "wails build reported success but $exe is missing."
}

$item = Get-Item $exe
$sizeMb = [math]::Round($item.Length / 1MB, 1)

Write-Host ''
Write-Host "Built $($config.info.productName) v$($config.info.productVersion)" -ForegroundColor Green
Write-Host "  $exe  ($sizeMb MB)" -ForegroundColor Green
