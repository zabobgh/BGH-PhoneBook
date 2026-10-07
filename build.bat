@echo off
setlocal
cd /d "%~dp0"
echo ==============================================
echo   BGH PhoneBook - BUILD WAILS WINDOWS GUI
echo ==============================================
where wails >nul 2>nul || (echo [ERROR] Wails CLI not found & pause & exit /b 1)
where npm >nul 2>nul || (echo [ERROR] npm not found & pause & exit /b 1)
where go >nul 2>nul || (echo [ERROR] Go not found & pause & exit /b 1)

echo Building Wails Application...
call wails build -trimpath -ldflags="-s -w" || (pause & exit /b 1)

if exist build\bin\BGH-PhoneBook.exe (
    copy /y build\bin\BGH-PhoneBook.exe .\BGH-PhoneBook.exe >nul
    echo.
    echo [OK] Build complete: BGH-PhoneBook.exe (Windows Desktop GUI)
) else (
    echo [ERROR] BGH-PhoneBook.exe not found in build\bin
)
pause
