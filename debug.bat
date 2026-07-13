@echo off
REM FORK-CUSTOM: Provide a local Windows development launcher for the fork.
cd /d "%~dp0"
echo Starting debug script...

set BACKEND_PORT=3000
set FRONTEND_PORT=5173

where go >nul 2>nul
if errorlevel 1 (
    echo Go not found
    pause
    exit /b 1
)

echo Go found

set FRONTEND_CMD=bun
where bun >nul 2>nul
if errorlevel 1 (
    echo Bun not found, using npm
    set FRONTEND_CMD=npm
)

echo Frontend command: %FRONTEND_CMD%

if not exist web\node_modules (
    echo Installing frontend dependencies...
    pushd web\default
    %FRONTEND_CMD% install
    popd
)

echo Starting backend...
start "Backend" cmd /k "set PORT=%BACKEND_PORT%&& go run main.go"

timeout /t 3 /nobreak >nul

echo Starting frontend...
pushd web\default
start "Frontend" cmd /k "set VITE_REACT_APP_SERVER_URL=http://localhost:%BACKEND_PORT%&& %FRONTEND_CMD% run dev -- --port %FRONTEND_PORT% --strict-port"
popd

echo.
echo Services started!
echo Backend: http://localhost:%BACKEND_PORT%
echo Frontend: http://localhost:%FRONTEND_PORT%
echo.
pause
