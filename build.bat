@echo off
REM FORK-CUSTOM: Provide a local Windows build helper for the fork.
cd /d "%~dp0"
if errorlevel 1 exit /b 1

go build -ldflags "-s -w" -o new-api.exe .
if errorlevel 1 exit /b %errorlevel%
