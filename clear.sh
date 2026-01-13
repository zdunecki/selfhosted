#!/bin/bash

# Cleanup script for npm workspaces monorepo
# Cleans node_modules, dist, and lock files from root and workspace packages

echo "🧹 Starting cleanup..."

# Define workspace packages
workspaces=("app" "web" "desktop/vite-src")

# Clean workspace packages
for workspace in "${workspaces[@]}"; do
  if [ -d "$workspace" ]; then
    echo "Cleaning $workspace..."
    
    # Remove node_modules
    if [ -d "$workspace/node_modules" ]; then
      echo "  Removing $workspace/node_modules"
      rm -rf "$workspace/node_modules"
    fi
    
    # Remove dist
    if [ -d "$workspace/dist" ]; then
      echo "  Removing $workspace/dist"
      rm -rf "$workspace/dist"
    fi
    
    # Remove lock files
    for lockfile in "package-lock.json" "pnpm-lock.yaml" "yarn.lock"; do
      if [ -f "$workspace/$lockfile" ]; then
        echo "  Removing $workspace/$lockfile"
        rm -f "$workspace/$lockfile"
      fi
    done
    
    # Remove TypeScript build info
    if [ -d "$workspace/node_modules/.tmp" ]; then
      echo "  Removing $workspace/node_modules/.tmp"
      rm -rf "$workspace/node_modules/.tmp"
    fi
    
    find "$workspace" -name "*.tsbuildinfo" -type f -delete 2>/dev/null
  fi
done

# Clean root
echo "Cleaning root..."
root_items=("node_modules" "package-lock.json" "pnpm-lock.yaml" "yarn.lock")

for item in "${root_items[@]}"; do
  if [ -d "$item" ]; then
    echo "  Removing $item"
    rm -rf "$item"
  elif [ -f "$item" ]; then
    echo "  Removing $item"
    rm -f "$item"
  fi
done

# Clean Go build artifacts
if [ -f "selfhosted" ]; then
  echo "  Removing selfhosted binary"
  rm -f "selfhosted"
fi

# Clean app build output
if [ -d "app/dist" ]; then
  echo "  Removing app/dist"
  rm -rf "app/dist"
fi

# Clean web build output
if [ -d "web/dist" ]; then
  echo "  Removing web/dist"
  rm -rf "web/dist"
fi

# Clean desktop build output
if [ -d "desktop/vite-src/dist" ]; then
  echo "  Removing desktop/vite-src/dist"
  rm -rf "desktop/vite-src/dist"
fi

echo "✅ Cleanup complete!"