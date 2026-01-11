# Development

## Build

### Frontend
```bash
cd web
npm install
npm run build
```

### Backend
```bash
# Copy frontend assets to server package
mkdir -p pkg/server/dist
cp -r web/dist/* pkg/server/dist/

# Build the binary
go build -o selfhosted
```