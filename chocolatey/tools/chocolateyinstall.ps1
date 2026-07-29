$ErrorActionPreference = 'Stop'
$toolsDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
$url64 = 'https://github.com/nic0der-im/routeros-cli/releases/download/v0.2.0/ros_0.2.0_windows_amd64.zip'
$checksum64 = '185413dfb6b9362250a047e2cfcba21792d29c07e5e9a1db58fd95b1242db464'

Install-ChocolateyZipPackage -PackageName 'ros' -Url64bit $url64 -UnzipLocation $toolsDir -Checksum64 $checksum64 -ChecksumType64 'sha256'
