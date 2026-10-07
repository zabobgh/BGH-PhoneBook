@echo off
setlocal
cd /d "%~dp0"
echo ==============================================
echo   BGH PhoneBook - WAILS DEV MODE
echo ==============================================
where wails >nul 2>nul || (echo [ERROR] Wails CLI not found & pause & exit /b 1)
echo Starting Wails Dev Server (Desktop GUI + Vite Hot Reload)...
wails dev
