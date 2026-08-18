@echo off
rem ============================================================
rem  LdSSHManager build script
rem  - svn update + environment check + build
rem  - NO admin rights required (missing tools are installed
rem    by INIT.bat, run manually once as administrator)
rem ============================================================

cd /d %~dp0
set APP=LdSSHManager
set BIN=build\bin
set OUT=%~dp0

rem ----- svn update (optional, fixed path) -----
set "SVN=C:\Soft\SubversionCommandLineTools\svn.exe"
if exist "%SVN%" (
    cd /d %~dp0..
    "%SVN%" cleanup
    "%SVN%" up
    cd /d %~dp0
) else (
    echo [SKIP] svn not found: %SVN%
)

rem ============================================================
rem  Add known install paths to PATH (winget installs do not
rem  refresh the current session PATH)
rem ============================================================
if exist "%ProgramFiles%\Go\bin\go.exe" set "PATH=%ProgramFiles%\Go\bin;%PATH%"
if exist "%ProgramFiles%\nodejs\node.exe" set "PATH=%ProgramFiles%\nodejs;%PATH%"
if exist "C:\msys64\mingw64\bin\gcc.exe" set "PATH=C:\msys64\mingw64\bin;%PATH%"
if exist "C:\msys64\ucrt64\bin\gcc.exe" set "PATH=C:\msys64\ucrt64\bin;%PATH%"
if exist "C:\Program Files (x86)\NSIS\makensis.exe" set "PATH=C:\Program Files (x86)\NSIS;%PATH%"
if exist "C:\Program Files\NSIS\makensis.exe" set "PATH=C:\Program Files\NSIS;%PATH%"
for /f "delims=" %%i in ('go env GOPATH 2^>nul') do set "GPATH=%%i"
if exist "%GPATH%\bin\wails.exe" set "PATH=%GPATH%\bin;%PATH%"

rem ============================================================
rem  Check build environment (no install, no elevation)
rem ============================================================
echo.
echo ============================================
echo  Checking build environment...
echo ============================================

set MISSING_TOOLS=0
where go >nul 2>&1
if errorlevel 1 set MISSING_TOOLS=1
where node >nul 2>&1
if errorlevel 1 set MISSING_TOOLS=1
where gcc >nul 2>&1
if errorlevel 1 set MISSING_TOOLS=1
where makensis >nul 2>&1
if errorlevel 1 set MISSING_TOOLS=1
where wails >nul 2>&1
if errorlevel 1 set MISSING_TOOLS=1

if not "%MISSING_TOOLS%"=="1" goto :tools_ok

echo.
echo [ERROR] Missing build tools. Please run INIT.bat once (as administrator) to install them,
echo         then rerun this script.
echo         - double-click INIT.bat (UAC prompt will appear), or
echo         - run in an admin console:  INIT.bat
exit /b 1

:tools_ok
go version
go version | findstr /c:"go1.26.5" >nul
if errorlevel 1 echo [INFO] Current Go differs from target go1.26.5 (keep as-is if intended)
node --version
node --version | findstr /c:"v24.18" >nul
if errorlevel 1 echo [INFO] Current Node differs from target v24.18.x (keep as-is if intended)
gcc --version
makensis /VERSION
wails version

rem ============================================================
rem  Build
rem ============================================================
echo.
echo ============================================
echo  All build tools ready. Building...
echo ============================================

set CGO_ENABLED=1
if not exist build mkdir build
if exist icon.png copy /y icon.png build\appicon.png >nul

echo Building single-file portable + NSIS installer (x64)...
wails build -platform windows/amd64 -nsis -webview2 embed -o %APP%-x64.exe
if errorlevel 1 goto :fail

if not exist "%OUT%" mkdir "%OUT%"
echo Placing artifacts in project root...
if exist "%BIN%\%APP%-x64.exe" move /y "%BIN%\%APP%-x64.exe" "%OUT%\%APP%-x64.exe"
for %%f in (%BIN%\*-installer.exe) do move /y "%%f" "%OUT%\%APP%-Install-x64.exe"

echo.
echo Done.
goto :eof

:fail
echo Build failed.
exit /b 1
