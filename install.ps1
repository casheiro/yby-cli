# Instalador do Yby CLI para Windows (Windows PowerShell 5.1 e PowerShell 7).
#
#   irm https://raw.githubusercontent.com/casheiro/yby-cli/main/install.ps1 | iex
#
# Com parâmetros:
#   & ([scriptblock]::Create((irm https://raw.githubusercontent.com/casheiro/yby-cli/main/install.ps1))) -Versao v5.8.4
#
# Parâmetros (todos opcionais):
#   -Versao   versão a instalar (ex.: v5.8.4); sem ela, a última release. Também pela env YBY_VERSION.
#   -Destino  pasta de instalação; padrão $env:LOCALAPPDATA\Programs\yby. Também pela env YBY_INSTALL_DIR.
#   -SemPath  não mexe no PATH do usuário.
#
# Não precisa de administrador: instala na pasta do usuário e põe a pasta no PATH do usuário.
#
# O arquivo é UTF-8 sem BOM. Pelo irm, os acentos saem certos. Salvo e rodado como arquivo no
# Windows PowerShell 5.1 de um Windows em cp1252, as mensagens saem com acentos trocados
# ("nÃ£o"); a instalação não muda. Nesse caso, prefira o irm ou o pwsh (PowerShell 7).
[CmdletBinding()]
param(
    [string]$Versao = $env:YBY_VERSION,
    [string]$Destino = $env:YBY_INSTALL_DIR,
    [switch]$SemPath
)

function Escrever-Passo([string]$Texto) {
    Write-Host $Texto
}

function Escrever-Aviso([string]$Texto) {
    if ($env:NO_COLOR) { Write-Host "aviso: $Texto" } else { Write-Host "aviso: $Texto" -ForegroundColor Yellow }
}

function Escrever-Ok([string]$Texto) {
    if ($env:NO_COLOR) { Write-Host $Texto } else { Write-Host $Texto -ForegroundColor Green }
}

# Arquitetura do Windows, não a do processo: um PowerShell de 32 bits ou emulado num Windows
# de 64 bits ainda deve levar o binário nativo.
function Obter-Arquitetura {
    $arq = $null
    try {
        $arq = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()
    } catch {
        $arq = $null
    }
    if (-not $arq) {
        $arq = $env:PROCESSOR_ARCHITEW6432
        if (-not $arq) { $arq = $env:PROCESSOR_ARCHITECTURE }
    }
    switch -Regex ($arq) {
        '^(X64|AMD64)$' { return 'amd64' }
        '^ARM64$' { return 'arm64' }
        default { throw "arquitetura não suportada: $arq (há binários para amd64 e arm64)" }
    }
}

function Normalizar-Pasta([string]$Pasta) {
    if (-not $Pasta) { return '' }
    return [Environment]::ExpandEnvironmentVariables($Pasta.Trim().Trim('"')).TrimEnd('\', '/')
}

# Devolve o PATH com a pasta acrescentada no fim, ou $null se ela já está nele.
function Acrescentar-AoPath([string]$PathAtual, [string]$Pasta) {
    $alvo = Normalizar-Pasta $Pasta
    $partes = @()
    if ($PathAtual) { $partes = @($PathAtual -split ';' | Where-Object { $_ -ne '' }) }
    foreach ($parte in $partes) {
        if ((Normalizar-Pasta $parte) -ieq $alvo) { return $null }
    }
    return (@($partes) + $Pasta) -join ';'
}

# Primeiro yby.exe no PATH dado que não é o recém-instalado e vem antes dele (ou $null).
function Encontrar-YbyAntes([string]$PathCompleto, [string]$Pasta) {
    $alvo = Normalizar-Pasta $Pasta
    foreach ($parte in ($PathCompleto -split ';')) {
        $dir = Normalizar-Pasta $parte
        if (-not $dir) { continue }
        if ($dir -ieq $alvo) { return $null }
        $candidato = Join-Path $dir 'yby.exe'
        if (Test-Path -LiteralPath $candidato -PathType Leaf) { return $candidato }
    }
    return $null
}

function Baixar([string]$Url, [string]$Arquivo) {
    Invoke-WebRequest -Uri $Url -OutFile $Arquivo -UseBasicParsing -Headers @{ 'User-Agent' = 'yby-install' }
}

function Instalar-Yby {
    $ErrorActionPreference = 'Stop'
    # A barra de progresso do Invoke-WebRequest deixa o download muito lento no PowerShell 5.1.
    $ProgressPreference = 'SilentlyContinue'
    # O PowerShell 5.1 pode não oferecer TLS 1.2, que o GitHub exige.
    try {
        [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
    } catch { }

    $repo = 'casheiro/yby-cli'
    $pasta = $Destino
    if (-not $pasta) { $pasta = Join-Path $env:LOCALAPPDATA 'Programs\yby' }
    # Caminho absoluto: um -Destino relativo (ou com %VARIAVEL% ou ~) no PATH do usuário não
    # valeria em nenhum terminal.
    $pasta = [Environment]::ExpandEnvironmentVariables($pasta.Trim().Trim('"'))
    $pasta = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($pasta)
    if ($pasta.Length -gt 3) { $pasta = $pasta.TrimEnd('\', '/') }

    Escrever-Ok 'Yby CLI: instalação'

    $arq = Obter-Arquitetura
    Escrever-Passo "sistema: windows/$arq"

    $versao = $Versao
    if (-not $versao) {
        Escrever-Passo 'buscando a última versão…'
        try {
            $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$repo/releases/latest" -UseBasicParsing -Headers @{ 'User-Agent' = 'yby-install' }
        } catch {
            throw "não deu para consultar a última versão no GitHub ($($_.Exception.Message)); confira a conexão (ou o limite da API) e tente de novo, ou informe a versão com -Versao"
        }
        $versao = [string]$release.tag_name
        if (-not $versao) { throw 'a resposta do GitHub não trouxe a versão; informe-a com -Versao (ex.: -Versao v5.8.4)' }
    }
    if (-not $versao.StartsWith('v')) { $versao = "v$versao" }
    Escrever-Passo "versão: $versao"

    # O nome do arquivo leva a versão sem o v: yby_5.8.4_windows_amd64.zip
    $arquivo = "yby_$($versao.Substring(1))_windows_$arq.zip"
    $baseUrl = "https://github.com/$repo/releases/download/$versao"

    $tmp = Join-Path ([IO.Path]::GetTempPath()) ("yby-install-" + [Guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $tmp | Out-Null
    try {
        Escrever-Passo "baixando $arquivo…"
        $zip = Join-Path $tmp $arquivo
        try {
            Baixar "$baseUrl/$arquivo" $zip
        } catch {
            throw "não deu para baixar $baseUrl/$arquivo ($($_.Exception.Message)); confira se a versão existe em https://github.com/$repo/releases"
        }
        $checksums = Join-Path $tmp 'checksums.txt'
        try {
            Baixar "$baseUrl/checksums.txt" $checksums
        } catch {
            throw "não deu para baixar o checksums.txt da $versao ($($_.Exception.Message))"
        }

        # SHA-256: cada linha do checksums.txt é "<hash>  <arquivo>".
        $esperado = $null
        foreach ($linha in (Get-Content -LiteralPath $checksums)) {
            $campos = $linha.Trim() -split '\s+'
            if ($campos.Count -ge 2 -and $campos[1].TrimStart('*') -eq $arquivo) { $esperado = $campos[0].ToLower(); break }
        }
        if (-not $esperado) { throw "o checksums.txt da $versao não lista $arquivo" }
        $obtido = (Get-FileHash -LiteralPath $zip -Algorithm SHA256).Hash.ToLower()
        if ($obtido -ne $esperado) {
            throw "o SHA-256 do download não confere (esperado $esperado, obtido $obtido); o arquivo veio corrompido ou alterado, rode de novo"
        }
        Escrever-Passo 'sha256 confere'

        $extraido = Join-Path $tmp 'extraido'
        Expand-Archive -LiteralPath $zip -DestinationPath $extraido -Force
        $novo = Join-Path $extraido 'yby.exe'
        if (-not (Test-Path -LiteralPath $novo -PathType Leaf)) { throw "o pacote $arquivo não trouxe o yby.exe" }

        New-Item -ItemType Directory -Path $pasta -Force | Out-Null
        $exe = Join-Path $pasta 'yby.exe'
        # Um yby.exe aberto não pode ser sobrescrito, mas pode ser renomeado: o antigo sai do caminho
        # e é apagado quando der (na próxima instalação, se ainda estiver aberto agora).
        $antigo = "$exe.old"
        if (Test-Path -LiteralPath $antigo) { Remove-Item -LiteralPath $antigo -Force -ErrorAction SilentlyContinue }
        if (Test-Path -LiteralPath $exe) { Move-Item -LiteralPath $exe -Destination $antigo -Force }
        try {
            Copy-Item -LiteralPath $novo -Destination $exe -Force
        } catch {
            if (Test-Path -LiteralPath $antigo) { Move-Item -LiteralPath $antigo -Destination $exe -Force -ErrorAction SilentlyContinue }
            throw "não deu para gravar $exe ($($_.Exception.Message))"
        }
        Remove-Item -LiteralPath $antigo -Force -ErrorAction SilentlyContinue
    } finally {
        Remove-Item -LiteralPath $tmp -Recurse -Force -ErrorAction SilentlyContinue
    }

    Escrever-Ok "yby $versao instalado em $exe"

    # PATH do usuário. O valor é lido e gravado cru no registro (REG_EXPAND_SZ): o
    # [Environment]::SetEnvironmentVariable gravaria o PATH expandido, perdendo os %VARIAVEL%.
    $pathNovo = $false
    if ($SemPath) {
        Escrever-Passo 'PATH do usuário não alterado (-SemPath)'
    } else {
        $chave = Get-Item -LiteralPath 'HKCU:\Environment'
        $pathUsuario = [string]$chave.GetValue('Path', '', 'DoNotExpandEnvironmentNames')
        $acrescido = Acrescentar-AoPath $pathUsuario $pasta
        if ($null -ne $acrescido) {
            Set-ItemProperty -LiteralPath 'HKCU:\Environment' -Name 'Path' -Value $acrescido -Type ExpandString
            # Avisa o Windows da mudança (WM_SETTINGCHANGE) para os terminais novos já verem o PATH:
            # o SetEnvironmentVariable de usuário faz esse aviso, então uma variável é criada e apagada.
            [Environment]::SetEnvironmentVariable('YBY_INSTALL_AVISO', '1', 'User')
            [Environment]::SetEnvironmentVariable('YBY_INSTALL_AVISO', $null, 'User')
            $pathNovo = $true
            Escrever-Passo "$pasta acrescentada ao PATH do usuário"
        }
        # Este terminal também passa a achar o yby.
        if ($null -ne (Acrescentar-AoPath $env:Path $pasta)) { $env:Path = "$env:Path;$pasta" }
    }

    # Um yby antigo (por exemplo de go install, em %USERPROFILE%\go\bin) que venha antes no PATH
    # continuaria sendo o que roda.
    $pathEfetivo = [Environment]::GetEnvironmentVariable('Path', 'Machine') + ';' + [Environment]::GetEnvironmentVariable('Path', 'User')
    $outro = Encontrar-YbyAntes $pathEfetivo $pasta
    if ($outro) {
        Escrever-Aviso "o yby que o terminal acha primeiro é $outro, não o recém-instalado; apague-o (Remove-Item '$outro') ou ponha $pasta antes no PATH"
    }

    if ($pathNovo) {
        Escrever-Passo 'abra um terminal novo para o PATH valer nele; depois rode yby version para conferir e yby doctor para ver as dependências'
    } else {
        Escrever-Passo 'rode yby version para conferir e yby doctor para ver as dependências'
    }
}

# Rodado como arquivo (powershell -File install.ps1, .\install.ps1), a falha sai com 1. Pelo
# irm | iex, o exit fecharia o terminal (ou o script) de quem instalou: o iex roda no escopo de
# quem chamou, então o $MyInvocation só é deste arquivo se o script em execução define o Instalar-Yby.
$comoArquivo = $false
if ($MyInvocation.MyCommand.CommandType -eq 'ExternalScript' -and $MyInvocation.MyCommand.ScriptBlock) {
    $comoArquivo = $null -ne $MyInvocation.MyCommand.ScriptBlock.Ast.Find({
            param($no)
            $no -is [System.Management.Automation.Language.FunctionDefinitionAst] -and $no.Name -eq 'Instalar-Yby'
        }, $true)
}

try {
    Instalar-Yby
} catch {
    if ($env:NO_COLOR) { Write-Host "erro: $($_.Exception.Message)" } else { Write-Host "erro: $($_.Exception.Message)" -ForegroundColor Red }
    if ($comoArquivo) { exit 1 }
}
