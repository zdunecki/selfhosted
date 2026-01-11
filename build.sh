#!/bin/bash
set -e

echo "📦 Building frontend..."
cd web && npm install
npm run build
cd ../

echo "📂 Copying assets..."
mkdir -p pkg/server/dist
cp -r web/dist/* pkg/server/dist/

echo "🔨 Building backend binary..."
go build -o selfhosted -v

echo "✅ Build complete! Run with: ./selfhosted"