$ErrorActionPreference = 'Stop'
$keepAlive = Start-Process "$env:WINDIR\System32\wsl.exe" -ArgumentList '-d Ubuntu --exec /bin/sleep infinity' -WindowStyle Hidden -PassThru
$previousAddress = ''
try {
    while (-not $keepAlive.HasExited) {
        try {
            $address = ((& wsl.exe -d Ubuntu --exec hostname -I).Trim() -split '\s+')[0]
            if ($address -match '^\d+\.\d+\.\d+\.\d+$' -and $address -ne $previousAddress) {
                & netsh interface portproxy add v4tov4 listenaddress=0.0.0.0 listenport=9005 connectaddress=$address connectport=9005
                if ($LASTEXITCODE -ne 0) { throw 'Could not configure forwarding.' }
                $previousAddress = $address
                Set-Content "$PSScriptRoot\forwarding-status.txt" "Forwarding port 9005 to $address at $(Get-Date -Format o)"
            }
        } catch {
            Set-Content "$PSScriptRoot\forwarding-status.txt" $_.Exception.Message
        }
        Start-Sleep -Seconds 30
    }
    exit 1
} finally {
    if (-not $keepAlive.HasExited) { Stop-Process -Id $keepAlive.Id }
}
