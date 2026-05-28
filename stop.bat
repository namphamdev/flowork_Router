@echo off
REM Flow Router — one-click stop (Windows)
echo Flow Router — stopping...
taskkill /F /IM flow-router-bin.exe >nul 2>&1
if errorlevel 1 (
    echo (not running)
) else (
    echo Stopped.
)
pause
