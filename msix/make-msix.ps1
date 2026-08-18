# ============================================================================
#  LdSSHManager MSIX 打包脚本 (Windows)
#
#  用途: 把 wails build 产出的 LdSSHManager.exe 打包成 MSIX, 用于提交微软商店。
#
#  前置条件:
#    1) 先运行 MAKE.bat 构建出 build/bin/LdSSHManager.exe
#    2) 本机已安装 Windows SDK (含 MakeAppx.exe)
#    3) 已从 Partner Center 获取 Publisher ID, 并已写入 AppxManifest.xml
#
#  用法:  powershell -ExecutionPolicy Bypass -File .\msix\make-msix.ps1
# ============================================================================
$ErrorActionPreference = "Stop"

$Root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)  # 项目根
$MsixDir = Join-Path $Root "msix"
$BinDir  = Join-Path $Root "build\bin"
$Payload = Join-Path $MsixDir "_payload"
$Version = "0.8.47.0"   # 与 AppxManifest.xml 保持一致; 发版时同步改

Write-Host "==> 1/4 准备 payload 目录 (exe + 图标)" -ForegroundColor Cyan
if (Test-Path $Payload) { Remove-Item -Recurse -Force $Payload }
New-Item -ItemType Directory -Path $Payload | Out-Null
New-Item -ItemType Directory -Path (Join-Path $Payload "assets") | Out-Null

$Exe = Join-Path $BinDir "LdSSHManager.exe"
if (-not (Test-Path $Exe)) { throw "未找到 $Exe, 请先运行 MAKE.bat 构建" }
Copy-Item $Exe (Join-Path $Payload "LdSSHManager.exe")
Copy-Item (Join-Path $MsixDir "assets\*.png") (Join-Path $Payload "assets\")
Copy-Item (Join-Path $MsixDir "AppxManifest.xml") (Join-Path $Payload "AppxManifest.xml")

Write-Host "==> 2/4 定位 MakeAppx.exe (Windows SDK)" -ForegroundColor Cyan
$makeAppx = Get-ChildItem "C:\Program Files (x86)\Windows Kits\10\bin" -Recurse -Filter "makeappx.exe" -ErrorAction SilentlyContinue |
            Sort-Object FullName -Descending | Select-Object -First 1
if (-not $makeAppx) { throw "未找到 MakeAppx.exe, 请安装 Windows SDK (开发人员工具)" }
Write-Host "    使用: $($makeAppx.FullName)"

Write-Host "==> 3/4 打包 MSIX" -ForegroundColor Cyan
$MsixPath = Join-Path $MsixDir "LdSSHManager-$Version.msix"
& $makeAppx.FullName pack /d $Payload /p $MsixPath /o
if ($LASTEXITCODE -ne 0) { throw "MakeAppx 打包失败 (exit $LASTEXITCODE)" }

Write-Host "==> 4/4 完成" -ForegroundColor Cyan
Remove-Item -Recurse -Force $Payload

Write-Host ""
Write-Host "完成! 产物:" -ForegroundColor Green
Write-Host "  - $MsixPath"
Write-Host ""
Write-Host "提交微软商店:" -ForegroundColor DarkYellow
Write-Host "  1. Partner Center -> 创建产品 -> MSIX 提交路径" -ForegroundColor DarkYellow
Write-Host "  2. 上传上面这个 .msix 文件 (商店会自动重新签名, 无需本地证书)" -ForegroundColor DarkYellow
Write-Host "  3. 补齐截图/描述/隐私政策后提交认证" -ForegroundColor DarkYellow
