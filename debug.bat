@echo off
REM FORK-CUSTOM: Provide a local Windows development launcher for the fork.
echo Starting debug script...

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
    pushd web
    %FRONTEND_CMD% install
    popd
)

echo Starting backend...
start "Backend" cmd /k go run main.go

timeout /t 3 /nobreak >nul

echo Starting frontend...
pushd web
start "Frontend" cmd /k %FRONTEND_CMD% run dev
popd

echo.
echo Services started!
echo Backend: http://localhost:3000
echo Frontend: http://localhost:5173
echo.
pause
