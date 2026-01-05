@echo off
echo ========================================
echo Starting 4 in a Row Game Server
echo ========================================

echo Starting backend server...
cd backend
start "Backend Server" cmd /k "go run cmd/server/main.go"

echo Waiting for backend to start...
timeout /t 3 /nobreak >nul

echo Starting frontend development server...
cd ..\frontend
start "Frontend Server" cmd /k "npm start"

echo.
echo ========================================
echo Servers are starting...
echo ========================================
echo Backend: http://localhost:8081
echo Frontend: http://localhost:3001
echo.
echo Press any key to close this window...
pause >nul