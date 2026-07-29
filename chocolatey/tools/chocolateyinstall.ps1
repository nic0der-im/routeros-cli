$ErrorActionPreference = 'Stop'
$toolsDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
$url64 = 'https://github.com/nic0der-im/routeros-cli/releases/download/v0.3.0/ros_0.3.0_windows_amd64.zip'
$checksum64 = '8b5c842b3e6f18c46313f1c8cf2cc267d0d96c7920ba0239ee93a477ccdbc018'

Install-ChocolateyZipPackage -PackageName 'ros' -Url64bit $url64 -UnzipLocation $toolsDir -Checksum64 $checksum64 -ChecksumType64 'sha256'
