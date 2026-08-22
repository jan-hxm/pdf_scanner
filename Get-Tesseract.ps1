<#
.SYNOPSIS
    Stages a self-contained Tesseract OCR into build\tesseract so the build can ship it
    next to the .exe.

.DESCRIPTION
    Locate() in internal/ocr prefers a copy at "<exe dir>\tesseract\tesseract.exe" over
    anything installed on the machine, so dropping a Tesseract tree there is all it takes
    to make OCR work out of the box. This script produces that tree.

    Three sources, tried in order, all ending in the same layout:

      1. -From <dir>          an existing Tesseract directory, named explicitly.
      2. an installed copy    the same locations Locate() probes - PATH, Program Files,
                              LOCALAPPDATA. Costs no download.
      3. the UB Mannheim installer, downloaded once into build\.cache and unpacked with
         7-Zip. The installer is an NSIS archive; 7-Zip reads it without running it, so
         nothing is installed system-wide and no elevation is involved. Without 7-Zip on
         the machine, -UseInstaller runs it silently into a temp directory instead - that
         route may raise a UAC prompt.

    Language data is topped up from tessdata_fast for anything the source did not bring,
    and everything not asked for is pruned: a full installer carries language models that
    dwarf the binaries. osd.traineddata is always kept - see $models below.

    The result is verified by running the staged binary - it has to report every requested
    language back through --list-langs. A tree that cannot answer that is not a bundle.

    Idempotent: a destination that already satisfies -Languages is left alone, so build.ps1
    can call this on every build.

.PARAMETER Destination
    Where the tree is staged. Defaults to build\tesseract, which build.ps1 copies into
    build\bin\tesseract after wails build.

.PARAMETER Languages
    Language codes to guarantee. Defaults to deu and eng - the two the picker offers
    together for a German document quoting an English abstract.

.PARAMETER Version
    Version of the UB Mannheim installer to download.

.PARAMETER From
    Copy from this directory instead of looking for or downloading anything.

.PARAMETER Sha256
    Expected hash of the downloaded installer. The script prints the hash it got either
    way; passing it back makes the download verified rather than merely repeatable.

.PARAMETER TessdataRepo
    Which tesseract-ocr data repository language files come from: tessdata_fast (default,
    quantised, fastest), tessdata_best (float, slowest and most accurate) or tessdata.

.PARAMETER UseInstaller
    Run the downloaded installer silently instead of unpacking it with 7-Zip. Only needed
    when 7-Zip is not available; may prompt for elevation.

.PARAMETER Force
    Rebuild the tree even if it already looks complete.

.EXAMPLE
    .\Get-Tesseract.ps1

.EXAMPLE
    .\Get-Tesseract.ps1 -From 'C:\Program Files\Tesseract-OCR' -Languages deu,eng,fra
#>
[CmdletBinding()]
param(
    [string]$Destination = (Join-Path $PSScriptRoot 'build\tesseract'),
    [string[]]$Languages = @('deu', 'eng'),
    [string]$Version = '5.4.0.20240606',
    [string]$From,
    [string]$Sha256,
    [ValidateSet('tessdata_fast', 'tessdata_best', 'tessdata')]
    [string]$TessdataRepo = 'tessdata_fast',
    [switch]$UseInstaller,
    [switch]$Force
)

$ErrorActionPreference = 'Stop'

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

$exeName = 'tesseract.exe'
$cacheDir = Join-Path $PSScriptRoot 'build\.cache'

# --- Helpers ----------------------------------------------------------------------------

# Invoke-Tool runs a native executable and hands back its exit code and its combined output.
#
# Windows PowerShell wraps every stderr line from a native command in an ErrorRecord, and
# with $ErrorActionPreference = 'Stop' that turns any diagnostic the tool prints into a
# terminating error. Tesseract prints diagnostics on runs that succeed - "Too few characters.
# Skipping this page" among them - so a plain `& $exe ... 2>&1` aborts the script over output
# it was collecting on purpose. The exit code is the only thing that says whether the tool
# worked; stderr is evidence, not a verdict.
function Invoke-Tool($exe, $arguments) {
    $previous = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        $output = & $exe @arguments 2>&1 | ForEach-Object { $_.ToString() }
        return [pscustomobject]@{
            ExitCode = $LASTEXITCODE
            Text     = ($output -join "`n")
        }
    } finally {
        $ErrorActionPreference = $previous
    }
}

# Test-Bundle answers the only question worth asking about an existing tree: is there a
# binary, and is every model the app needs sitting beside it. Passing $models rather than
# $languages is also what retires a tree staged before osd was required - it now fails this
# check and gets rebuilt, instead of being left in place by the idempotency shortcut.
function Test-Bundle($dir, $models) {
    if (-not (Test-Path (Join-Path $dir $exeName))) { return $false }
    foreach ($model in $models) {
        if (-not (Test-Path (Join-Path $dir "tessdata\$model.traineddata"))) { return $false }
    }
    return $true
}

# Find-Installed mirrors Locate() in internal/ocr/tesseract.go, minus the bundled copy -
# looking for the bundle we are about to create would be circular.
function Find-Installed {
    $onPath = Get-Command tesseract -ErrorAction SilentlyContinue
    if ($null -ne $onPath) { return (Split-Path $onPath.Source -Parent) }

    $probes = @(
        'C:\Program Files\Tesseract-OCR',
        'C:\Program Files (x86)\Tesseract-OCR'
    )
    if ($env:LOCALAPPDATA) {
        $probes += (Join-Path $env:LOCALAPPDATA 'Programs\Tesseract-OCR')
        $probes += (Join-Path $env:LOCALAPPDATA 'Tesseract-OCR')
    }
    foreach ($probe in $probes) {
        if (Test-Path (Join-Path $probe $exeName)) { return $probe }
    }
    return $null
}

function Find-SevenZip {
    foreach ($name in @('7z', '7za')) {
        $cmd = Get-Command $name -ErrorAction SilentlyContinue
        if ($null -ne $cmd) { return $cmd.Source }
    }
    $probes = @(
        'C:\Program Files\7-Zip\7z.exe',
        'C:\Program Files (x86)\7-Zip\7z.exe',
        'C:\msys64\usr\bin\7z.exe',
        'C:\msys64\mingw64\bin\7z.exe'
    )
    foreach ($probe in $probes) {
        if (Test-Path $probe) { return $probe }
    }
    return $null
}

# userAgent names this script rather than imitating a browser. It is not decoration: the
# UB Mannheim mirror answers Windows PowerShell 5.1's default user agent with 403, because
# "WindowsPowerShell/5.1" on a download of an .exe is a signature droppers wear. Saying who
# we actually are gets a 200 and leaves an honest line in someone's access log.
$userAgent = 'pdf_scanner-build (+https://github.com/jan-hxm/pdf_scanner)'

# Invoke-Download is Invoke-WebRequest with the three things Windows PowerShell 5.1 gets
# wrong for this job: the user agent above, TLS 1.0 by default, which these hosts refuse,
# and a progress bar that costs more wall clock than the transfer does. The partial file is
# renamed into place only on success, so an interrupted download is never mistaken for a
# cache hit on the next run.
function Invoke-Download($url, $path) {
    $previous = $ProgressPreference
    $ProgressPreference = 'SilentlyContinue'
    try {
        [Net.ServicePointManager]::SecurityProtocol =
            [Net.SecurityProtocolType]::Tls12 -bor [Net.SecurityProtocolType]::Tls11
        $partial = "$path.partial"
        Invoke-WebRequest -Uri $url -OutFile $partial -UseBasicParsing -UserAgent $userAgent
        Move-Item $partial $path -Force
    } finally {
        $ProgressPreference = $previous
    }
}

function New-EmptyDir($path) {
    if (Test-Path $path) { Remove-Item $path -Recurse -Force }
    New-Item -ItemType Directory -Path $path | Out-Null
}

$wanted = @($Languages | ForEach-Object { $_.ToLowerInvariant() })

# osd is not a language and never appears in the picker - listLanguages() in
# internal/ocr/tesseract.go filters it out. It is the orientation and script detection
# model, and recognize() asks for it on every page with --psm 1.
#
# Without it, a scanned slide deck - landscape content on portrait paper, stored sideways -
# comes back as rotated gibberish that the scorer cannot match and the cache then keeps.
# Tesseract only warns about the missing model on stderr and carries on, so the failure is
# silent from the app's side; 10 MB against a 165 MB tree is not a trade worth thinking
# about twice. $models is therefore what gets kept, topped up and verified, while $wanted
# stays what the user asked for.
$models = @($wanted + 'osd' | Select-Object -Unique)

# --- 0. Already staged? -----------------------------------------------------------------

if (-not $Force -and (Test-Bundle $Destination $models)) {
    Write-Step 'Tesseract is already staged'
    Write-Ok "$Destination ($($wanted -join ', '))"
    exit 0
}

# --- 1. Acquire -------------------------------------------------------------------------

$staging = Join-Path ([IO.Path]::GetTempPath()) ('pdf_scanner-tesseract-' + [Guid]::NewGuid().ToString('N').Substring(0, 8))
New-EmptyDir $staging

try {
    $source = $From
    if ([string]::IsNullOrWhiteSpace($source)) { $source = Find-Installed }

    if (-not [string]::IsNullOrWhiteSpace($source)) {
        Write-Step "Copying Tesseract from $source"
        if (-not (Test-Path (Join-Path $source $exeName))) {
            Fail "$source does not contain $exeName."
        }
        Copy-Item (Join-Path $source '*') $staging -Recurse -Force
    } else {
        $url = "https://digi.bib.uni-mannheim.de/tesseract/tesseract-ocr-w64-setup-$Version.exe"
        $installer = Join-Path $cacheDir "tesseract-ocr-w64-setup-$Version.exe"

        Write-Step "Downloading Tesseract $Version"
        if (Test-Path $installer) {
            Write-Ok "cached  $installer"
        } else {
            if (-not (Test-Path $cacheDir)) { New-Item -ItemType Directory -Path $cacheDir | Out-Null }
            Write-Ok $url
            try {
                Invoke-Download $url $installer
            } catch {
                Fail ("Download failed: $($_.Exception.Message)`n" +
                    "    That version may have been superseded. Pick one from`n" +
                    "    https://digi.bib.uni-mannheim.de/tesseract/ and pass it as -Version,`n" +
                    "    or point -From at an existing Tesseract installation.")
            }
        }

        $hash = (Get-FileHash $installer -Algorithm SHA256).Hash
        Write-Ok "sha256  $hash"
        if (-not [string]::IsNullOrWhiteSpace($Sha256)) {
            if ($hash -ne $Sha256.Trim().ToUpperInvariant()) {
                Remove-Item $installer -Force
                Fail "SHA-256 mismatch - expected $Sha256. The cached copy has been deleted."
            }
            Write-Ok 'hash matches -Sha256'
        }

        $sevenZip = Find-SevenZip
        if ($null -ne $sevenZip -and -not $UseInstaller) {
            Write-Step "Unpacking with $sevenZip"
            & $sevenZip x $installer "-o$staging" -y | Out-Null
            # 7-Zip returns 1 for warnings it recovered from; 2 and up are fatal.
            if ($LASTEXITCODE -gt 1) { Fail "7-Zip could not unpack the installer (exit $LASTEXITCODE)." }
        } else {
            if (-not $UseInstaller) {
                Fail ("7-Zip not found, and it is what unpacks the installer without running it.`n" +
                    "    Install it with:  winget install -e --id 7zip.7zip`n" +
                    "    or re-run with -UseInstaller to run the installer silently instead`n" +
                    "    (into a temp directory - it may prompt for elevation),`n" +
                    "    or point -From at an existing Tesseract installation.")
            }
            # NSIS takes its target as an unquoted /D= that has to come last, so a path with
            # a space in it cannot be expressed. Install beside the staging directory, which
            # is under the temp path and ours to name.
            $installed = "$staging-install"
            if ($installed -match '\s') { Fail "The temp path contains a space, which NSIS /D= cannot express: $installed" }
            New-EmptyDir $installed

            Write-Step 'Running the installer silently'
            Write-Ok $installed
            $proc = Start-Process -FilePath $installer -ArgumentList '/S', "/D=$installed" -Wait -PassThru
            if ($proc.ExitCode -ne 0) { Fail "The installer exited with $($proc.ExitCode)." }
            Copy-Item (Join-Path $installed '*') $staging -Recurse -Force
            Remove-Item $installed -Recurse -Force
        }
    }

    # --- 2. Find the payload and prune it ------------------------------------------------

    # An NSIS archive unpacks to a layout that has changed between installer builds -
    # sometimes flat, sometimes under a directory, always with a $PLUGINSDIR of installer
    # scaffolding beside it. Rather than assume one, find the binary and let its directory
    # define the payload.
    $binary = Get-ChildItem $staging -Filter $exeName -Recurse -File | Select-Object -First 1
    if ($null -eq $binary) { Fail "No $exeName among the acquired files." }
    $payload = $binary.Directory.FullName

    Write-Step 'Pruning'
    $before = (Get-ChildItem $payload -Recurse -File | Measure-Object -Property Length -Sum).Sum

    # Documentation, the installer's own scaffolding and the uninstaller are weight the app
    # never reads.
    foreach ($junk in @('doc', '$PLUGINSDIR', '$R0', 'uninstall.exe', 'Uninstall.exe')) {
        $path = Join-Path $payload $junk
        if (Test-Path $path) { Remove-Item $path -Recurse -Force }
    }
    Get-ChildItem $payload -Filter '*.nsi' -File -ErrorAction SilentlyContinue | Remove-Item -Force

    # The installer ships the whole toolchain: lstmtraining, text2image, mftraining and a
    # dozen more, together some 63 MB of programs for building language models. The app runs
    # exactly one binary, so every other executable goes. The .jar files are ScrollView, a
    # Java debugging viewer for the same training work.
    Get-ChildItem $payload -Filter '*.exe' -File -Recurse |
        Where-Object { $_.Name -ne $exeName } | Remove-Item -Force
    Get-ChildItem $payload -Include '*.jar', '*.html' -File -Recurse | Remove-Item -Force

    # The DLLs are deliberately left alone. Most of the remaining bulk is libtesseract-5.dll
    # and ICU's data table, both of which are load-bearing, and the handful that only the
    # training tools needed (pango, cairo, fontconfig) is around 10 MB against a 165 MB tree.
    # Guessing wrong there produces an app that fails to start on a user's machine, which is
    # a bad trade for six per cent.

    $tessdata = Join-Path $payload 'tessdata'
    if (-not (Test-Path $tessdata)) { New-Item -ItemType Directory -Path $tessdata | Out-Null }

    # Language models are pruned to what was asked for - an installer's full tessdata runs
    # to several times the size of the binaries - plus osd, which the app does ask for on
    # every page. See $models.
    $dropped = 0
    Get-ChildItem $tessdata -Filter '*.traineddata' -File -Recurse | ForEach-Object {
        if ($models -notcontains $_.BaseName.ToLowerInvariant()) {
            Remove-Item $_.FullName -Force
            $dropped++
        }
    }
    if ($dropped -gt 0) { Write-Ok "dropped $dropped unrequested language file(s)" }

    $after = (Get-ChildItem $payload -Recurse -File | Measure-Object -Property Length -Sum).Sum
    Write-Ok ('{0:N0} MB removed' -f (($before - $after) / 1MB))

    # --- 3. Top up the languages ---------------------------------------------------------

    Write-Step 'Language data'
    foreach ($model in $models) {
        $file = Join-Path $tessdata "$model.traineddata"
        if (Test-Path $file) {
            Write-Ok "$model  from the source"
            continue
        }
        # osd.traineddata is byte-identical across tessdata, tessdata_fast and
        # tessdata_best - the orientation model is not quantised per repository - so it
        # comes from whichever one is configured without a special case.
        $url = "https://raw.githubusercontent.com/tesseract-ocr/$TessdataRepo/main/$model.traineddata"
        Write-Ok "$model  $url"
        try {
            Invoke-Download $url $file
        } catch {
            Fail "Could not fetch $model.traineddata from $TessdataRepo - $($_.Exception.Message)"
        }
    }

    # --- 4. Publish ----------------------------------------------------------------------

    $parent = Split-Path $Destination -Parent
    if (-not (Test-Path $parent)) { New-Item -ItemType Directory -Path $parent | Out-Null }
    if (Test-Path $Destination) { Remove-Item $Destination -Recurse -Force }
    Move-Item $payload $Destination
} finally {
    if (Test-Path $staging) {
        Remove-Item $staging -Recurse -Force -ErrorAction SilentlyContinue
    }
}

# --- 5. Verify --------------------------------------------------------------------------

# The tree is a bundle only if the binary in it runs and sees its own language data. The app
# sets TESSDATA_PREFIX the same way - see command() in internal/ocr/tesseract.go - so this
# asks exactly the question OCRStatus will ask at runtime.
Write-Step 'Verifying the staged copy'

$staged = Join-Path $Destination $exeName
$env:TESSDATA_PREFIX = Join-Path $Destination 'tessdata'

$listed = Invoke-Tool $staged @('--list-langs')
if ($listed.ExitCode -ne 0) {
    Fail "$staged --list-langs failed (exit $($listed.ExitCode)):`n$($listed.Text)"
}
$reported = @($listed.Text -split "`n" | ForEach-Object { $_.Trim() } | Where-Object { $_ -and $_ -notmatch ':$' })

foreach ($model in $models) {
    if ($reported -notcontains $model) {
        Fail "The staged Tesseract does not report '$model'. It lists: $($reported -join ', ')"
    }
}

# --list-langs proves the language data is visible. It does not prove the engine still
# recognises anything, which is exactly what the prune above puts at risk - so render a word
# and read it back.
#
# The word is rendered sideways and read with --psm 1, the same mode recognize() uses. That
# makes this the one check that can fail over a broken osd model: --list-langs is happy to
# name a file the engine cannot load, and a straight fixture reads back correctly whether
# orientation detection works or not. A sideways one does not.
#
# The app pipes the PNG in on stdin; this writes a temp file instead, because PowerShell
# cannot reproduce the app's invocation faithfully: $proc.StandardInput is a StreamWriter
# whose UTF-8 preamble reaches the pipe ahead of the raw bytes, Tesseract then fails to see
# a PNG signature and falls back to reading stdin as a list of filenames. Go's
# cmd.Stdin = bytes.NewReader(png) has no such preamble. The substitution costs nothing that
# matters here: leptonica's PNG decoding, the models and the engine are the same either way,
# and stdin plumbing is not something pruning files can break.
#
# Failing to *build* the fixture is this script's problem, not the bundle's, so that warns.
# Failing to read it back is the bundle's problem, and fails.
#
# Several lines rather than one word: orientation detection is a statistical judgement over
# the characters it can see, and Tesseract answers a single word with "Too few characters.
# Skipping this page" - which is not a verdict on the bundle either way.
$probeWord = 'Suchbegriff'
$probeLines = @(
    'Antidepressiva in der Hausarzt-Praxis',
    'Zentrum fuer Angst- und Depressionsbehandlung',
    "$probeWord in einem laengeren Absatz,",
    'damit die Ausrichtung erkannt werden kann.'
)
$probeImage = Join-Path ([IO.Path]::GetTempPath()) ('pdf_scanner-ocr-probe-' + [Guid]::NewGuid().ToString('N').Substring(0, 8) + '.png')
$rendered = $false
try {
    Add-Type -AssemblyName System.Drawing
    # Portrait page, landscape text: the shape a scanned slide deck arrives in. The transform
    # rotates the drawing surface, so the text is genuinely sideways in the saved pixels
    # rather than merely tagged as rotated - an EXIF orientation flag would prove nothing.
    #
    # Counter-clockwise specifically. Tesseract's line finder copes with the clockwise case
    # on its own, osd or no osd, so a fixture rotated that way passes either way and tests
    # nothing. Rotated the other way it reads the lines bottom-up and returns
    # "Bunjpueysqsuoissaldagq" - which is the failure this bundle exists to prevent, and the
    # direction the corpus actually had.
    $bitmap = New-Object Drawing.Bitmap 500, 1300
    $graphics = [Drawing.Graphics]::FromImage($bitmap)
    $graphics.Clear([Drawing.Color]::White)
    $graphics.TextRenderingHint = [Drawing.Text.TextRenderingHint]::AntiAliasGridFit
    $font = New-Object Drawing.Font 'Arial', 40
    $graphics.TranslateTransform(0, $bitmap.Height)
    $graphics.RotateTransform(-90)
    for ($i = 0; $i -lt $probeLines.Count; $i++) {
        $graphics.DrawString($probeLines[$i], $font, [Drawing.Brushes]::Black, 40, (40 + $i * 70))
    }
    $graphics.Dispose()
    $bitmap.Save($probeImage, [Drawing.Imaging.ImageFormat]::Png)
    $bitmap.Dispose()
    $font.Dispose()
    $rendered = $true
} catch {
    Write-Host "    could not render a test image, skipping the recognition check: $($_.Exception.Message)" -ForegroundColor Yellow
}

if ($rendered) {
    try {
        $probe = Invoke-Tool $staged @($probeImage, '-', '--psm', '1', '-l', $wanted[0])
        if ($probe.ExitCode -ne 0) {
            Fail "The staged Tesseract could not read a test image (exit $($probe.ExitCode)):`n$($probe.Text)"
        }
        if ($probe.Text -notmatch [regex]::Escape($probeWord)) {
            Fail ("The staged Tesseract ran but did not read the sideways test image back.`n" +
                "    Rendered '$probeWord' rotated 90 degrees, got:`n" +
                "    $($probe.Text.Trim() -replace "`n", "`n    ")`n" +
                "    Either osd.traineddata is missing or unloadable - orientation was not`n" +
                "    corrected - or something else the engine needs was pruned.")
        }
        Write-Ok "recognised '$probeWord' from a sideways image (--psm 1)"
    } finally {
        Remove-Item $probeImage -Force -ErrorAction SilentlyContinue
    }
}

$reportedVersion = ((Invoke-Tool $staged @('--version')).Text -split "`n" | Select-Object -First 1).Trim()
$size = (Get-ChildItem $Destination -Recurse -File | Measure-Object -Property Length -Sum).Sum
$sizeMb = [math]::Round($size / 1MB, 1)

Write-Host ''
Write-Host "Staged $reportedVersion" -ForegroundColor Green
Write-Host "  $Destination  ($sizeMb MB, languages: $($reported -join ', '))" -ForegroundColor Green

exit 0
