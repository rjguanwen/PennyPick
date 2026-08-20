@echo off
cd /d "%~dp0"
start "pennypick-vite" /min cmd /c "npm run dev > dev-server.log 2>&1"
