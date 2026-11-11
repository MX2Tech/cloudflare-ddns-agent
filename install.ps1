# Instalador do cloudflare-ddns-agent para Windows (PowerShell 5.1+).
# Rode num PowerShell "Executar como administrador":
#
#   & ([scriptblock]::Create((irm https://raw.githubusercontent.com/MX2Tech/cloudflare-ddns-agent/main/install.ps1))) -Zona exemplo.com -Hostname host.exemplo.com
#
# O token da Cloudflare e pedido de forma oculta e nunca vai para parametro
# nem historico. O binario fica em Program Files (so administrador escreve:
# ele roda como SYSTEM) e a config em ProgramData com acesso so de SYSTEM e
# Administradores.
#
# Arquivo mantido em ASCII de proposito: o PowerShell 5.1 le .ps1 sem BOM
# como ANSI e corromperia acentos.
param(
    [string]$Zona,
    [string]$Hostname,
    [int]$IntervaloSegundos = 30,
    [string]$Versao = 'latest',
    # .exe ja baixado (ex.: maquina sem acesso ao GitHub). Pula a checagem de SHA-256.
    [string]$BinarioLocal,
    # Regrava a config mesmo se ja existir (pede o token de novo).
    [switch]$Reconfigurar
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'   # a barra de progresso deixa o download do PS 5.1 muito lento
[Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

$Repo       = 'MX2Tech/cloudflare-ddns-agent'
$TaskName   = 'cloudflare-ddns-agent'
$InstallDir = Join-Path $env:ProgramFiles 'cloudflare-ddns-agent'
$Exe        = Join-Path $InstallDir 'cloudflare-ddns-agent.exe'
$DataDir    = Join-Path $env:ProgramData 'cloudflare-ddns-agent'
$ConfigPath = Join-Path $DataDir 'config.yaml'
$LogPath    = Join-Path $DataDir 'agent.log'

function Falhar([string]$msg) { Write-Host "  [XX] $msg" -ForegroundColor Red; exit 1 }
function Ok([string]$msg) { Write-Host "  [ok] $msg" -ForegroundColor Green }

# Codigo de saida de um executavel sem deixar o stderr dele virar excecao no PS 5.1.
# -Silencioso descarta a saida (o icacls imprime resumo mesmo com /Q).
function Invoke-Nativo([string]$Arquivo, [string[]]$Argumentos, [switch]$Silencioso) {
    $anterior = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        if ($Silencioso) { & $Arquivo @Argumentos 2>&1 | Out-Null } else { & $Arquivo @Argumentos | Out-Host }
        return $LASTEXITCODE
    }
    finally { $ErrorActionPreference = $anterior }
}

$principal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Falhar 'Rode este instalador num PowerShell aberto com "Executar como administrador".'
}
if (-not [Environment]::Is64BitOperatingSystem) { Falhar 'Windows 32 bits nao e suportado.' }

Write-Host ''
Write-Host 'cloudflare-ddns-agent - instalacao no Windows'
Write-Host '---------------------------------------------'

# 1. Binario
$tmp = Join-Path $env:TEMP ('cloudflare-ddns-agent-' + [guid]::NewGuid().ToString('N') + '.exe')
if ($BinarioLocal) {
    if (-not (Test-Path $BinarioLocal)) { Falhar "Binario nao encontrado: $BinarioLocal" }
    Copy-Item $BinarioLocal $tmp
    Write-Host '  [!!] usando binario local, sem checagem de SHA-256' -ForegroundColor Yellow
} else {
    if ($Versao -eq 'latest') { $base = "https://github.com/$Repo/releases/latest/download" }
    else { $base = "https://github.com/$Repo/releases/download/$Versao" }

    $sumsFile = Join-Path $env:TEMP ('cloudflare-ddns-agent-sums-' + [guid]::NewGuid().ToString('N') + '.txt')
    Invoke-WebRequest -UseBasicParsing -Uri "$base/checksums.txt" -OutFile $sumsFile
    $linha = Get-Content $sumsFile | Where-Object { $_ -match 'windows_amd64' } | Select-Object -First 1
    Remove-Item $sumsFile -Force
    if (-not $linha) { Falhar "A versao '$Versao' nao tem binario para Windows (windows_amd64) no checksums.txt." }
    $partes = -split $linha
    $shaEsperado = $partes[0]
    $nomeAsset = $partes[1]

    Write-Host "  baixando $nomeAsset ($Versao)..."
    Invoke-WebRequest -UseBasicParsing -Uri "$base/$nomeAsset" -OutFile $tmp
    $shaObtido = (Get-FileHash -Algorithm SHA256 $tmp).Hash
    if ($shaObtido -ne $shaEsperado) {
        Remove-Item $tmp -Force
        Falhar "SHA-256 nao confere (esperado $shaEsperado, obtido $shaObtido). Nada foi instalado."
    }
    Ok 'download conferido por SHA-256'
}

# Para o loop em execucao antes de trocar o .exe (Windows nao sobrescreve arquivo em uso).
$tarefa = Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
if ($tarefa) {
    Stop-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
    for ($i = 0; $i -lt 20 -and (Get-Process -Name 'cloudflare-ddns-agent' -ErrorAction SilentlyContinue); $i++) {
        Start-Sleep -Milliseconds 500
    }
}
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
Move-Item -Force $tmp $Exe
Ok "binario em $Exe"

# 2. Pasta da config: restringe ANTES de gravar o token. SIDs = independente do idioma do Windows.
New-Item -ItemType Directory -Force -Path $DataDir | Out-Null
$rc = Invoke-Nativo 'icacls' @($DataDir, '/inheritance:r', '/grant:r', '*S-1-5-18:(OI)(CI)F', '*S-1-5-32-544:(OI)(CI)F', '/Q') -Silencioso
if ($rc -ne 0) { Falhar "Nao consegui restringir as permissoes de $DataDir (icacls saiu com $rc)." }
Ok "$DataDir acessivel so por SYSTEM e Administradores"

# 3. Config
if ((Test-Path $ConfigPath) -and -not $Reconfigurar) {
    Ok "config existente mantida: $ConfigPath (use -Reconfigurar para regravar)"
} else {
    if (-not $Zona) { $Zona = Read-Host 'Zona na Cloudflare (ex: mx2tech.cloud)' }
    if (-not $Hostname) { $Hostname = Read-Host "Hostname a manter atualizado (ex: cliente.$Zona)" }
    $Zona = $Zona.Trim().ToLowerInvariant()
    $Hostname = $Hostname.Trim().ToLowerInvariant()
    if ($Zona -notmatch '^[a-z0-9-]+(\.[a-z0-9-]+)+$') { Falhar "Zona invalida: '$Zona'" }
    if ($Hostname -ne $Zona -and -not $Hostname.EndsWith(".$Zona")) { Falhar "'$Hostname' nao pertence a zona '$Zona'." }
    if ($IntervaloSegundos -lt 10) { Falhar 'Intervalo minimo: 10 segundos.' }

    $seguro = Read-Host 'Cloudflare API Token (nao aparece na tela)' -AsSecureString
    $bstr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($seguro)
    try { $token = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($bstr) }
    finally { [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr) }
    $token = $token.Trim()
    if ($token -notmatch '^[A-Za-z0-9_-]{20,}$') { Falhar 'Token vazio ou com caracteres inesperados.' }

    $yaml = "cloudflare:`n  api_token: `"$token`"`ncheck_interval: ${IntervaloSegundos}s`nrecords:`n  - zone: $Zona`n    hostname: $Hostname`n"
    [IO.File]::WriteAllText($ConfigPath, $yaml, (New-Object Text.UTF8Encoding $false))
    Remove-Variable token, yaml
    Ok "config gravada em $ConfigPath"
}

# 4. Teste real antes de agendar
Write-Host ''
Write-Host 'Testando a configuracao...'
$rc = Invoke-Nativo $Exe @('update')
if ($rc -ne 0) {
    Falhar "A checagem inicial falhou. Corrija $ConfigPath (ou rode de novo com -Reconfigurar) e repita."
}

# 5. Tarefa agendada (SYSTEM, no boot, com watchdog)
Write-Host ''
Write-Host 'Registrando a tarefa agendada...'
$rc = Invoke-Nativo $Exe @('install')
if ($rc -ne 0) { Falhar 'Falha ao registrar a tarefa agendada.' }

Start-Sleep -Seconds 3
$info = Get-ScheduledTask -TaskName $TaskName
Ok ("tarefa '$TaskName' estado: " + $info.State)
if (Test-Path $LogPath) {
    Write-Host ''
    Write-Host "Ultimas linhas de $LogPath :"
    Get-Content $LogPath -Tail 5 | ForEach-Object { Write-Host "    $_" }
}

Write-Host ''
Write-Host 'Pronto! O agente roda como SYSTEM, sobe no boot e se religa sozinho se cair.'
Write-Host "Log:         Get-Content '$LogPath' -Tail 20 -Wait"
Write-Host "Desinstalar: & '$Exe' uninstall"
