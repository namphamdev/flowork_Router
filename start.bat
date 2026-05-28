@echo off
REM ╔══════════════════════════════════════════════════════════════════════╗
REM ║ Flow Router — one-click start (Windows)                              ║
REM ║ Builds the binary on first run (needs Go 1.25+), then serves on      ║
REM ║ http://127.0.0.1:2402. Set FLOW_ROUTER_PORT to override.             ║
REM ╚══════════════════════════════════════════════════════════════════════╝

setlocal
cd /d "%~dp0"

if not defined FLOW_ROUTER_PORT set FLOW_ROUTER_PORT=2402
echo Flow Router — starting on http://127.0.0.1:%FLOW_ROUTER_PORT%

if not exist "flow-router-bin.exe" (
    echo - binary not found, building one-time ^(needs Go 1.25+^)...
    go build -o flow-router-bin.exe .
    if errorlevel 1 (
        echo build failed.
        pause
        exit /b 1
    )
)

REM Heavy brain assets live in project root under brain/ and models/.
REM Override only the brain path so main data.sqlite stays at canonical
REM %USERPROFILE%\.flow_router\db\ (preserves provider/OAuth state).
if exist "%~dp0brain\flowork-brain.sqlite" (
    if not defined FLOW_ROUTER_BRAIN_DB set FLOW_ROUTER_BRAIN_DB=%~dp0brain\flowork-brain.sqlite
    echo Brain: %~dp0brain\flowork-brain.sqlite
)

flow-router-bin.exe --addr "127.0.0.1:%FLOW_ROUTER_PORT%"
pause
