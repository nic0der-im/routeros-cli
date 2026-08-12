$ErrorActionPreference = 'Stop'
$toolsDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
$url64 = 'https://github.com/nic0der-im/routeros-cli/releases/download/v0.7.0/ros_0.7.0_windows_amd64.zip'
$checksum64 = 'be124ed3c0d1bfc7f28413bcd8f1ebdda9ef86f6a88f9aa7be4a04422950f287'

Install-ChocolateyZipPackage -PackageName 'ros' -Url64bit $url64 -UnzipLocation $toolsDir -Checksum64 $checksum64 -ChecksumType64 'sha256'
