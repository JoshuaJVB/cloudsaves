# Update the CloudSave Windows client to the latest release.
#
# Downloads the newest installer from GitHub and installs it silently. Run with:
#
#     powershell -ExecutionPolicy Bypass -File update-client.ps1
#
$ErrorActionPreference = 'Stop'
$repo = 'JoshuaJVB/cloudsaves'

Write-Host "Checking latest release..."
$rel = Invoke-RestMethod "https://api.github.com/repos/$repo/releases/latest"
$asset = $rel.assets | Where-Object { $_.name -eq 'CloudSave-Setup.exe' } | Select-Object -First 1
if (-not $asset) {
  throw "No CloudSave-Setup.exe found in the latest release ($($rel.tag_name))."
}

$out = Join-Path $env:TEMP 'CloudSave-Setup.exe'
Write-Host "Downloading CloudSave $($rel.tag_name)..."
Invoke-WebRequest -Uri $asset.browser_download_url -OutFile $out

# Strip the Mark-of-the-Web so SmartScreen doesn't flag the (unsigned)
# installer with a "Windows protected your PC" prompt.
Unblock-File -Path $out

# Close the app if it's running so the installer can replace the executable.
Get-Process -Name CloudSave -ErrorAction SilentlyContinue | Stop-Process -Force

Write-Host "Installing..."
Start-Process -FilePath $out -ArgumentList '/VERYSILENT', '/SUPPRESSMSGBOXES', '/NORESTART' -Wait

Write-Host "CloudSave updated to $($rel.tag_name)."
