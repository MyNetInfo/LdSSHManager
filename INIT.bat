@echo off
rem ============================================================
rem  LdSSHManager init script
rem  - detect missing build tools and install them via winget
rem    (Go / Node.js / MSYS2 GCC / NSIS / Wails CLI)
rem  - needs administrator rights (auto elevates once, UAC)
rem  - run manually when MAKE.bat reports missing tools
rem ============================================================

cd /d %~dp0
set GO_VERSION=1.26.5
set NODE_VERSION=24.18.1

rem ----- Require admin rights (auto elevate once) -----
net session >nul 2>&1
if not errorlevel 1 goto :admin_ok
if "%~1"=="elevated" goto :admin_ok
echo Requesting administrator privileges...
powershell -NoProfile -Command "Start-Process -FilePath '%~f0' -ArgumentList 'elevated' -Verb RunAs -Wait"
exit /b
:admin_ok

echo.
echo ============================================
echo  Checking and installing build tools (admin)...
echo ============================================

where winget >nul 2>&1
if errorlevel 1 (
    echo [ERROR] winget not found. Install "App Installer" from Microsoft Store, then rerun.
    exit /b 1
)

rem ----- Go -----
where go >nul 2>&1
if not errorlevel 1 goto :i_go_ok
if exist "%ProgramFiles%\Go\bin\go.exe" (
    set "PATH=%ProgramFiles%\Go\bin;%PATH%"
    goto :i_go_ok
)
echo [INSTALL] Go %GO_VERSION% not found, installing via winget (GoLang.Go)...
winget install --id GoLang.Go --version %GO_VERSION% -e --silent --accept-package-agreements --accept-source-agreements
if errorlevel 1 (
    echo [ERROR] Go install failed. Install Go %GO_VERSION% manually from https://go.dev/dl/
    exit /b 1
)
if exist "%ProgramFiles%\Go\bin\go.exe" set "PATH=%ProgramFiles%\Go\bin;%PATH%"
:i_go_ok
go version

rem ----- Node.js (frontend build) -----
where node >nul 2>&1
if not errorlevel 1 goto :i_node_ok
if exist "%ProgramFiles%\nodejs\node.exe" (
    set "PATH=%ProgramFiles%\nodejs;%PATH%"
    goto :i_node_ok
)
echo [INSTALL] Node.js %NODE_VERSION% not found, installing via winget (OpenJS.NodeJS.LTS)...
winget install --id OpenJS.NodeJS.LTS --version %NODE_VERSION% -e --silent --accept-package-agreements --accept-source-agreements
if errorlevel 1 (
    echo [ERROR] Node.js install failed. Install Node.js %NODE_VERSION% manually from https://nodejs.org/
    exit /b 1
)
if exist "%ProgramFiles%\nodejs\node.exe" set "PATH=%ProgramFiles%\nodejs;%PATH%"
:i_node_ok
node --version

rem ----- GCC (CGO required by go-sqlcipher) -----
where gcc >nul 2>&1
if not errorlevel 1 goto :i_gcc_ok
if exist "C:\msys64\mingw64\bin\gcc.exe" (
    set "PATH=C:\msys64\mingw64\bin;%PATH%"
    goto :i_gcc_ok
)
if exist "C:\msys64\ucrt64\bin\gcc.exe" (
    set "PATH=C:\msys64\ucrt64\bin;%PATH%"
    goto :i_gcc_ok
)
echo [INSTALL] GCC not found, installing MSYS2 via winget (MSYS2.MSYS2)...
winget install --id MSYS2.MSYS2 -e --silent --accept-package-agreements --accept-source-agreements
if not exist "C:\msys64\usr\bin\bash.exe" (
    echo [ERROR] MSYS2 install failed. Please install it manually from https://www.msys2.org/
    exit /b 1
)
echo [INSTALL] Installing MinGW-w64 gcc toolchain via pacman...
C:\msys64\usr\bin\bash.exe -lc "pacman -Sy --noconfirm --needed mingw-w64-ucrt-x86_64-gcc"
if errorlevel 1 (
    echo [ERROR] GCC install failed. Run manually: pacman -S mingw-w64-ucrt-x86_64-gcc
    exit /b 1
)
if exist "C:\msys64\ucrt64\bin\gcc.exe" set "PATH=C:\msys64\ucrt64\bin;%PATH%"
if exist "C:\msys64\mingw64\bin\gcc.exe" set "PATH=C:\msys64\mingw64\bin;%PATH%"
:i_gcc_ok
gcc --version

rem ----- NSIS (makensis, required for -nsis installer) -----
where makensis >nul 2>&1
if not errorlevel 1 goto :i_nsis_ok
if exist "C:\Program Files (x86)\NSIS\makensis.exe" (
    set "PATH=C:\Program Files (x86)\NSIS;%PATH%"
    goto :i_nsis_ok
)
if exist "C:\Program Files\NSIS\makensis.exe" (
    set "PATH=C:\Program Files\NSIS;%PATH%"
    goto :i_nsis_ok
)
echo [INSTALL] NSIS not found, installing via winget (NSIS.NSIS)...
winget install --id NSIS.NSIS -e --silent --accept-package-agreements --accept-source-agreements
if errorlevel 1 (
    echo [ERROR] NSIS install failed. Please install it manually from https://nsis.sourceforge.io/Download
    exit /b 1
)
if exist "C:\Program Files (x86)\NSIS\makensis.exe" set "PATH=C:\Program Files (x86)\NSIS;%PATH%"
if exist "C:\Program Files\NSIS\makensis.exe" set "PATH=C:\Program Files\NSIS;%PATH%"
:i_nsis_ok
makensis /VERSION

rem ----- Wails CLI -----
where wails >nul 2>&1
if not errorlevel 1 goto :i_wails_ok
for /f "delims=" %%i in ('go env GOPATH') do set "GPATH=%%i"
if exist "%GPATH%\bin\wails.exe" goto :i_wails_ok
echo [INSTALL] Wails CLI not found, installing via go install...
go install github.com/wailsapp/wails/v2/cmd/wails@latest
if errorlevel 1 (
    echo [ERROR] Wails CLI install failed. Run manually: go install github.com/wailsapp/wails/v2/cmd/wails@latest
    exit /b 1
)
if exist "%GPATH%\bin\wails.exe" set "PATH=%GPATH%\bin;%PATH%"
:i_wails_ok
wails version

echo.
echo ============================================
echo  All build tools ready. Now run MAKE.bat.
echo ============================================
exit /b 0
