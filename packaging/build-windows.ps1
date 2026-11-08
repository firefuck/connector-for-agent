param([string]$ISCC='ISCC.exe',[string]$CertificateThumbprint='', [string]$TimestampServer='http://timestamp.digicert.com')
$ErrorActionPreference='Stop'
Push-Location (Join-Path $PSScriptRoot '..')
try {
 New-Item -ItemType Directory -Force dist | Out-Null
 & go build -buildvcs=false -trimpath '-ldflags=-s -w' -o dist/relaydesk.exe ./cmd/relaydesk
 if($LASTEXITCODE -ne 0){throw 'Go build failed'}
 $version=(& .\dist\relaydesk.exe version) -replace '^RelayDesk ',''
 if($version -notmatch '^\d+\.\d+\.\d+$'){throw 'Invalid product version'}
 Copy-Item LICENSE,NOTICE,THIRD_PARTY_NOTICES.md dist -Force
 function Sign-And-Verify([string]$Path){
  $cert=Get-Item "Cert:\CurrentUser\My\$CertificateThumbprint"
  if(-not $cert.HasPrivateKey){throw 'Certificate has no private signing key'}
  $signature=Set-AuthenticodeSignature -FilePath $Path -Certificate $cert -HashAlgorithm SHA256 -TimestampServer $TimestampServer
  if($signature.Status -ne 'Valid'){throw "Signature verification failed for $Path"}
 }
 if($CertificateThumbprint){Sign-And-Verify (Resolve-Path dist/relaydesk.exe)}
 & $ISCC "/DProductVersion=$version" packaging/relaydesk.iss
 if($LASTEXITCODE -ne 0){throw 'Inno Setup build failed'}
 $installer="dist/RelayDesk-Setup-$version-x64.exe"
 if($CertificateThumbprint){Sign-And-Verify (Resolve-Path $installer)}
 $files=@('dist/relaydesk.exe',$installer,'dist/LICENSE','dist/NOTICE','dist/THIRD_PARTY_NOTICES.md')
 $lines=foreach($file in $files){$hash=(Get-FileHash -Algorithm SHA256 $file).Hash.ToLower(); "$hash  $([IO.Path]::GetFileName($file))"}
 [IO.File]::WriteAllLines((Join-Path (Get-Location) 'dist/SHA256SUMS-WINDOWS'),$lines,[Text.Encoding]::ASCII)
 if(-not $CertificateThumbprint){Write-Warning 'Build is UNSIGNED. A checksum does not verify publisher identity.'}
 Write-Host "Created $installer. Pairing is preserved during uninstall."
}finally{Pop-Location}
