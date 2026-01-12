Add-Type -AssemblyName System.Drawing
$source = "web/icon.png"
$dest = "cmd/rift/app.ico"

if (-not (Test-Path $source)) {
    Write-Error "Source file $source not found!"
    exit 1
}

$bmp = [System.Drawing.Bitmap]::FromFile((Resolve-Path $source))
# Create a new handle to the icon
$hicon = $bmp.GetHicon()
$icon = [System.Drawing.Icon]::FromHandle($hicon)

$fs = [System.IO.File]::OpenWrite((Resolve-Path -Path "." -Relative).ToString() + "\" + $dest)
$icon.Save($fs)
$fs.Close()

# Cleanup
$icon.Dispose()
[System.Runtime.InteropServices.Marshal]::DestroyIcon($hicon) | Out-Null
$bmp.Dispose()

Write-Host "Converted $source to $dest"
