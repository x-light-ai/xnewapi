@echo off
REM ============================================
REM Local Development Script
REM ============================================
REM 本地开发脚本,连接本地后端服务
REM 后端默认地址: http://localhost:3000

echo [INFO] Starting local development server...
echo [INFO] Backend: http://localhost:3000
echo.

REM 确保使用本地环境变量
if exist .env.development.local (
    echo [WARN] Found .env.development.local, it may override settings
    echo [WARN] Make sure VITE_REACT_APP_SERVER_URL is not set or points to localhost:3000
    echo.
)

echo [INFO] Starting Rsbuild dev server...
npm run dev

pause
