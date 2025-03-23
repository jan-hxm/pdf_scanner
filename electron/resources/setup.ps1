### Define constants for installation
$pythonInstallerFilename = "Winpython64-3.11.3.1dot.exe"
$installationFolderName = "PDF-Searcher"
$requirementsFilename = "requirements.txt"
$pdfFolderName = "pdf"
$searchHistoryFileName = "search_history.json"
$workdirFileName = "production_workdir.json"

### Determine script and required paths
$scriptDirectory = Split-Path -Parent $MyInvocation.MyCommand.Definition
$installerFilePath = Join-Path $scriptDirectory $pythonInstallerFilename
$requirementsFilePath = Join-Path $scriptDirectory $requirementsFilename
$documentsDirectory = [Environment]::GetFolderPath("MyDocuments")
$pdfFolderSource = Join-Path $scriptDirectory $pdfFolderName

### Define target installation directory
$targetInstallationPath = Join-Path $documentsDirectory $installationFolderName
$destinationInstallerPath = Join-Path $targetInstallationPath $pythonInstallerFilename
$pdfFolderPath = Join-Path $targetInstallationPath $pdfFolderName
$historyFilePath = Join-Path $targetInstallationPath $searchHistoryFileName

### Check if an existing WinPython installation is present
$existingPythonBasePath = Get-ChildItem -Path $targetInstallationPath -Directory -ErrorAction SilentlyContinue |
    Where-Object { $_.Name -match "^WPy64-" } |
    Select-Object -First 1 -ExpandProperty FullName

### Create installation directory if it does not exist
if (-Not (Test-Path $targetInstallationPath)) {
    New-Item -ItemType Directory -Path $targetInstallationPath -Force | Out-Null
    Write-Host "[Setup]: Created target installation folder at: $targetInstallationPath"
}

if (-Not (Test-Path $pdfFolderPath)) {
    Write-Host "[Setup]: No pdf folder found. Copying default..."
    Copy-item -Force -Recurse -Verbose $pdfFolderSource -Destination $pdfFolderPath
    Write-Host "[Setup]: Copied pdf folder to $pdfFolderPath."
} else {
    Write-Host "[Setup]: Pdf folder found. Skipping copy..."
}

if (-Not (Test-Path $historyFilePath)) {
    @{"history" = @()} | ConvertTo-Json -Depth 2 | Set-Content -Path $historyFilePath -Encoding UTF8
    Write-Host "[Setup]: Created search history file"
} else {
    Write-Host "[Setup]: Search history file found. Skipping ..."
}

if (-not $existingPythonBasePath) {
    ### Ensure the installer is available
    if (!(Test-Path $installerFilePath)) {
        Write-Host "[Error]: Installer not found! Please download WinPython and place it in the same directory as this script."
        exit
    }

    Write-Host "[Installation]: Installing Portable Python to: $targetInstallationPath ..."

    ### Copy installer to the target location if not already present
    if (-Not (Test-Path $destinationInstallerPath)) {
        Copy-Item -Path $installerFilePath -Destination $destinationInstallerPath
        Write-Host "[Success]: PyInstaller Copied successfully to $destinationInstallerPath"
    } else {
        Write-Host "[Setup]: PyIntstaller File already exists. Using existing installer..."
    }

    ### Run the WinPython installer silently
    Start-Process -FilePath $destinationInstallerPath -ArgumentList "/VERYSILENT /DIR=$($targetInstallationPath)" -Wait

    ### Check again for installation directory
    $existingPythonBasePath = Get-ChildItem -Path $targetInstallationPath -Directory -ErrorAction SilentlyContinue |
        Where-Object { $_.Name -match "^WPy64-" } |
        Select-Object -First 1 -ExpandProperty FullName

    if (-not $existingPythonBasePath) {
        Write-Host "[Error]: WinPython installation failed!"
        exit
    }

    Write-Host "[Success]: Portable Python successfully installed at: $existingPythonBasePath"

    ### Remove installer file after installation
    Remove-Item -Path $destinationInstallerPath -Force
    Write-Host "[Cleanup]: Installer file deleted."
} else {
    Write-Host "[Success]: Found existing WinPython installation at: $existingPythonBasePath"
}

### Locate Python executable inside the WinPython installation
$pythonExecutablePath = Get-ChildItem -Path "$existingPythonBasePath" -Directory |
    Where-Object { $_.Name -match "^python-.*\.amd64$" } |
    ForEach-Object { Join-Path $_.FullName "python.exe" } |
    Select-Object -First 1

if (-not $pythonExecutablePath -or !(Test-Path $pythonExecutablePath)) {
    Write-Host "[Error]: Python executable not found inside WinPython installation!"
    exit
}

Write-Host "[Safety Check]: Python located at: $pythonExecutablePath"

### Install required Python dependencies if requirements file exists
if (Test-Path $requirementsFilePath) {
    Write-Host "[Dependency Installation]: Installing Python dependencies from requirements.txt..."
    & "$pythonExecutablePath" -m pip install --upgrade pip
    & "$pythonExecutablePath" -m pip install -r "$requirementsFilePath"
    Write-Host "[Success]: Python dependencies installed successfully."
} else {
    Write-Host "[Error]: No requirements.txt file found. Skipping dependency installation."
}

### Save the Python executable path to a configuration file
$pythonConfigData = @{ "workdir" = $targetInstallationPath } | ConvertTo-Json
$pythonConfigFilePath = Join-Path $scriptDirectory $workdirFileName
$pythonConfigData | Set-Content -Path $pythonConfigFilePath

Write-Host "[Configuration]: Python path saved in $pythonConfigFilePath"
