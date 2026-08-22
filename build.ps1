<#
.SYNOPSIS
    Builds PDF Scanner with the Wails CLI, including the checks and the mingw-w64
    workaround a clean checkout needs.

.DESCRIPTION
    `wails build` is the only working entry point (see CLAUDE.md "Building"), but it needs
    a few things arranged around it:

      1. go / node / npm / wails / gcc on PATH, and Wails CLI >= 2.15 (2.12 cannot
         generate bindings under Go 1.27).
      2. The Go-only quality gates: gofmt, go vet, and the tests that need no C toolchain.
      3. CGO_LDFLAGS aliasing __intrinsic_setjmpex to _setjmpex, which go-fitz' prebuilt
         MuPDF needs on mingw-w64 v12+.
      4. A clean build/bin, so stale binaries from an earlier name do not linger.
      5. The packaging assets copied from packaging/ into build/, where Wails insists on
         reading them - build/ is generated and untracked, packaging/ is the source.
      6. Tesseract staged into build/bin/tesseract, where internal/ocr looks for a bundled
         copy first, so OCR works without the user installing anything.

.PARAMETER Dev
    Run `wails dev` (live reload) instead of a production build.

.PARAMETER SkipChecks
    Skip gofmt / go vet / go test.

.PARAMETER SkipTesseract
    Do not stage Tesseract. The build still succeeds - the app reports a missing engine as
    a state rather than an error - but scanned PDFs stay unsearchable unless Tesseract is
    installed on the machine that runs it.

.PARAMETER NoClean
    Keep the existing contents of build/bin instead of passing -clean.

.PARAMETER StopRunning
    Terminate a running PDF Scanner instead of refusing to build over it.

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
    [switch]$SkipTesseract,
    [switch]$NoClean,
    [switch]$StopRunning,
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

function Write-Warn($text) {
    Write-Host "    $text" -ForegroundColor Yellow
}

function Fail($text) {
    Write-Host ''
    Write-Host "!!! $text" -ForegroundColor Red
    exit 1
}

function Assert-ExitCode($what) {
    if ($LASTEXITCODE -ne 0) { Fail "$what failed (exit $LASTEXITCODE)." }
}

# Copy-Packaging stages the icon, manifest and version template Wails reads while it builds.
#
# Wails hardcodes build/ for these - there is no CLI flag and no wails.json key - and it
# quietly writes its own defaults over anything missing. That makes build/ a directory Wails
# both reads sources from and writes output to, which is why it could not simply be
# committed: ignoring it loses the icon, tracking it commits the binaries. So the sources
# live in packaging/, under version control, and get copied in on every build. A stale or
# hand-edited build/ copy cannot drift: it is overwritten from packaging/ each time.
function Copy-Packaging {
    $source = Join-Path $PSScriptRoot 'packaging'
    if (-not (Test-Path $source)) {
        Fail "packaging/ is missing - it holds the app icon, manifest and version info that Wails needs."
    }

    Write-Step 'Staging the packaging assets'
    $target = Join-Path $PSScriptRoot 'build'
    if (-not (Test-Path $target)) { New-Item -ItemType Directory -Path $target | Out-Null }
    Copy-Item (Join-Path $source '*') $target -Recurse -Force

    Get-ChildItem $source -Recurse -File | ForEach-Object {
        Write-Ok "build\$($_.FullName.Substring($source.Length + 1))"
    }
}

# Assert-NotRunning keeps `wails build -clean` away from a build\bin the app has open.
#
# -clean empties build\bin and then writes the binary. A running instance holds a lock on
# its own .exe, so the wipe succeeds - taking build\bin\tesseract with it - and the write
# then fails with "Access is denied". Assert-ExitCode aborts the build there, which is
# *before* Copy-Tesseract, so what survives is the previous .exe with no engine beside it.
# Launch that and OCR reports Tesseract as not installed, with nothing in the chain naming
# the rebuild as the cause. Cheaper to refuse up front than to explain afterwards.
function Assert-NotRunning($exePath) {
    $name = [IO.Path]::GetFileNameWithoutExtension($exePath)
    $running = @(
        Get-Process -Name $name -ErrorAction SilentlyContinue |
            Where-Object { $_.Path -eq $exePath }
    )
    if ($running.Count -eq 0) { return }

    $pids = ($running | ForEach-Object { $_.Id }) -join ', '
    if (-not $StopRunning) {
        Fail ("$name is running (PID $pids) and holds its own .exe open.`n" +
            "    Building over it would delete the bundled Tesseract, then fail to replace`n" +
            "    the .exe - leaving an app that reports OCR as unavailable.`n" +
            "    Close PDF Scanner and run this again, or pass -StopRunning.")
    }

    Write-Step "Stopping $name (-StopRunning)"
    foreach ($proc in $running) {
        Write-Ok "PID $($proc.Id)"
        Stop-Process -Id $proc.Id -Force
    }

    # The lock outlives the process by a moment, and losing that race puts us straight back
    # into the failure this function exists to prevent. Wait for the file itself, not the
    # process handle: an exclusive open is the same question the linker will ask.
    $deadline = (Get-Date).AddSeconds(10)
    while ((Get-Date) -lt $deadline) {
        try {
            ([IO.File]::Open($exePath, 'Open', 'ReadWrite', 'None')).Dispose()
            Write-Ok 'lock released'
            return
        } catch {
            Start-Sleep -Milliseconds 200
        }
    }
    Fail "$exePath is still locked 10s after stopping $name."
}

# Copy-Tesseract puts the staged tree beside the binary, which is where Locate() in
# internal/ocr looks first. It runs after `wails build`, because -clean empties build/bin.
#
# A missing or failed bundle is a warning rather than a failure: the app treats an absent
# engine as a state it renders, so a build without it is a working build that cannot read
# scans - not a broken one. Failing here would block a build over an offline machine.
function Copy-Tesseract($staged, $binDir) {
    if ($SkipTesseract) {
        Write-Step 'Skipping the Tesseract bundle (-SkipTesseract)'
        return
    }

    Write-Step 'Staging Tesseract'
    $global:LASTEXITCODE = 0
    try {
        & (Join-Path $PSScriptRoot 'Get-Tesseract.ps1') -Destination $staged
    } catch {
        Write-Warn $_.Exception.Message
        $global:LASTEXITCODE = 1
    }
    if ($LASTEXITCODE -ne 0) {
        Write-Warn 'Tesseract could not be staged - building without it.'
        Write-Warn 'Scanned PDFs will only be searchable where Tesseract is installed.'
        Write-Warn 'Run .\Get-Tesseract.ps1 on its own to see why, or pass -SkipTesseract.'
        return
    }

    $target = Join-Path $binDir 'tesseract'
    if (Test-Path $target) { Remove-Item $target -Recurse -Force }
    if (-not (Test-Path $binDir)) { New-Item -ItemType Directory -Path $binDir | Out-Null }
    Copy-Item $staged $target -Recurse -Force
    Write-Ok "bundled into $target"
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

    # Both packages are free of the PDF dependency, so they test without a C
    # toolchain or a MuPDF DLL. That is the reason they are listed one by one
    # rather than as ./internal/... - the packages that import go-fitz would
    # drag the whole toolchain into a check meant to be cheap.
    & go test ./internal/match/ ./internal/ocr/textcache/
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
$binDir = Join-Path $PSScriptRoot 'build\bin'
$stagedTesseract = Join-Path $PSScriptRoot 'build\tesseract'
$exe = Join-Path $binDir ('{0}.exe' -f $config.outputfilename)

# Before anything is copied or cleaned: a locked .exe makes the whole run destructive.
Assert-NotRunning $exe

Copy-Packaging

if ($Dev) {
    # `wails dev` builds into build/bin as well, so the dev binary finds the same bundle a
    # released one does - otherwise OCR would only work in production.
    Copy-Tesseract $stagedTesseract $binDir

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

# --- 5. Bundle Tesseract ----------------------------------------------------------------

Copy-Tesseract $stagedTesseract $binDir

# --- 6. Report --------------------------------------------------------------------------

if (-not (Test-Path $exe)) {
    Fail "wails build reported success but $exe is missing."
}

$item = Get-Item $exe
$sizeMb = [math]::Round($item.Length / 1MB, 1)

Write-Host ''
Write-Host "Built $($config.info.productName) v$($config.info.productVersion)" -ForegroundColor Green
Write-Host "  $exe  ($sizeMb MB)" -ForegroundColor Green

$bundle = Join-Path $binDir 'tesseract'
if (Test-Path $bundle) {
    $bundleMb = [math]::Round((Get-ChildItem $bundle -Recurse -File | Measure-Object -Property Length -Sum).Sum / 1MB, 1)
    Write-Host "  $bundle  ($bundleMb MB)" -ForegroundColor Green
} else {
    Write-Host "  no Tesseract bundle at $bundle" -ForegroundColor Yellow
    Write-Host "  the app will report OCR as unavailable - run .\Get-Tesseract.ps1 to see why" -ForegroundColor Yellow
}
