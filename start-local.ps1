$ErrorActionPreference = 'Stop'
$siteRoot = $PSScriptRoot
$siteExecutable = Join-Path $siteRoot 'community-blacklist.exe'
if (-not (Test-Path -LiteralPath $siteExecutable)) { throw '请先在此目录运行 go build -o community-blacklist.exe .' }
$siteData = Join-Path $siteRoot 'local-only'
New-Item -ItemType Directory -Path $siteData -Force | Out-Null
$env:STORAGE_MODE = 'sqlite'
$env:DATABASE_FILE = Join-Path $siteData 'alliance.db'
$env:LISTEN_ADDR = '127.0.0.1:8089'
$env:PUBLIC_ORIGIN = 'http://127.0.0.1:8089'
$env:COOKIE_SECURE = 'false'
$env:BASE_PATH = ''
$env:TRUST_PROXY_LOOPBACK = 'false'
$env:IMPORT_JSON = ''
try {
    $siteHealth = Invoke-RestMethod 'http://127.0.0.1:8089/api/v1/health' -TimeoutSec 2
    if ($siteHealth.ok) { Write-Output '本地网站已运行：http://127.0.0.1:8089/'; return }
} catch {}
$siteProcess = Start-Process -FilePath $siteExecutable -WorkingDirectory $siteData -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $siteData 'server.log') -RedirectStandardError (Join-Path $siteData 'server-error.log')
Set-Content -LiteralPath (Join-Path $siteData 'process.pid') -Value $siteProcess.Id
Write-Output "本地网站已启动：http://127.0.0.1:8089/ （PID $($siteProcess.Id)）"
