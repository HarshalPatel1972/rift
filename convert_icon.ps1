Add-Type -AssemblyName System.Drawing

$srcPath = "$PSScriptRoot\web\icon.png"
$destPath = "$PSScriptRoot\cmd\rift\rift.ico"
$size = 256

try {
    # Load original image
    $srcImg = [System.Drawing.Bitmap]::FromFile($srcPath)
    
    # Create new resized bitmap (HighQualityBicubic for smoothness)
    $resized = New-Object System.Drawing.Bitmap($size, $size)
    $graph = [System.Drawing.Graphics]::FromImage($resized)
    $graph.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
    $graph.DrawImage($srcImg, 0, 0, $size, $size)
    
    # Convert to Icon
    $icon = [System.Drawing.Icon]::FromHandle($resized.GetHicon())
    
    # Save
    $fileStream = New-Object System.IO.FileStream($destPath, [System.IO.FileMode]::Create)
    $icon.Save($fileStream)
    
    # Cleanup
    $fileStream.Close()
    $icon.Dispose()
    $graph.Dispose()
    $resized.Dispose()
    $srcImg.Dispose()
    
    Write-Host "Success: Resized to 256x256 and converted to ICO."
} catch {
    Write-Error "Conversion Failed: $_"
    exit 1
}
