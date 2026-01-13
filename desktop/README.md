# SelfHosted Desktop (Neutralino)

This folder contains a lightweight desktop wrapper around the existing SelfHosted web UI. It uses **Neutralino** to show the installer in a native window (webview).

## Prerequisites
- Install Neutralino CLI: `neu` (see Neutralino docs)
- Node.js >= 18
- A compiled backend binary available as `selfhost` (or `./selfhost`) on your PATH

## Dev setup
1. Create `desktop/vite-src/.env` with:

```bash
VITE_GLOBAL_URL=http://localhost:3000/
```

2. From `desktop/`:

```bash
neu update
neu run
```

The desktop app will try to start the backend automatically using:

```bash
selfhost serve --port 8080 --no-browser
```

If that fails, start the backend manually and re-open the desktop app.

## Build

```bash
neu build
```

