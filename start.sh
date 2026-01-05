#!/bin/bash

echo "🎮 Starting 4 in a Row Game Server"
echo "=================================="

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Please install Go 1.21+ first."
    exit 1
fi

# Check if Node.js is installed
if ! command -v node &> /dev/null; then
    echo "❌ Node.js is not installed. Please install Node.js 18+ first."
    exit 1
fi

echo "✅ Prerequisites check passed"

# Setup backend dependencies
echo "📦 Installing backend dependencies..."
cd backend && go mod tidy
if [ $? -ne 0 ]; then
    echo "❌ Failed to install backend dependencies"
    exit 1
fi

# Setup frontend dependencies
echo "📦 Installing frontend dependencies..."
cd ../frontend && npm install
if [ $? -ne 0 ]; then
    echo "❌ Failed to install frontend dependencies"
    exit 1
fi

echo "✅ Dependencies installed successfully"
echo ""
echo "🚀 Ready to start!"
echo ""
echo "To start the application:"
echo "1. Backend:  cd backend && go run cmd/server/main.go"
echo "2. Frontend: cd frontend && npm start"
echo ""
echo "Or use the Makefile:"
echo "- make start-backend"
echo "- make start-frontend"
echo ""
echo "Optional infrastructure:"
echo "- make start-infra  # PostgreSQL + Kafka via Docker"