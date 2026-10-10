@echo off
setlocal
cd /d "%~dp0"

echo ========================================================
echo   BGH PhoneBook - Deploy Frontend to GitHub Pages
echo ========================================================

where git >nul 2>nul || (echo [ERROR] git not found & pause & exit /b 1)
where npm >nul 2>nul || (echo [ERROR] npm not found & pause & exit /b 1)

echo [1/3] Building Frontend with Production Assets...
cd frontend
call npm run build || (echo [ERROR] Build failed & pause & exit /b 1)
cd ..

echo [2/3] Preparing gh-pages branch...
if exist ..\gh-pages-deploy rd /s /q ..\gh-pages-deploy >nul 2>nul
git worktree add ..\gh-pages-deploy origin/gh-pages || git worktree add ..\gh-pages-deploy gh-pages || (echo [ERROR] Failed to add worktree & pause & exit /b 1)

echo [3/3] Copying build files to gh-pages...
powershell -Command "Copy-Item -Path 'frontend\dist\*' -Destination '..\gh-pages-deploy' -Recurse -Force; Copy-Item -Path 'frontend\dist\index.html' -Destination '..\gh-pages-deploy\404.html' -Force; New-Item -ItemType File -Name '..\gh-pages-deploy\.nojekyll' -Force | Out-Null"

cd ..\gh-pages-deploy
git checkout -B gh-pages
git add -A
git commit -m "deploy: update GitHub Pages build"
git push origin gh-pages

cd ..\BGH-PhoneBook
git worktree remove ..\gh-pages-deploy --force

echo.
echo ========================================================
echo  [SUCCESS] GitHub Pages updated successfully!
echo  URL: https://zabobgh.github.io/BGH-PhoneBook/
echo ========================================================
pause
