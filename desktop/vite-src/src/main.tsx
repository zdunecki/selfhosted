import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { init } from "@neutralinojs/lib";
import * as Neutralino from "@neutralinojs/lib";
import App from "./App";

// Get backend URL from environment and set it on window for the app to use
async function setupBackendUrl() {
  try {
    const envUrl = await Neutralino.os.getEnv("SELFHOSTED_BACKEND_URL");
    if (envUrl && envUrl.trim()) {
      // @ts-ignore
      window.SELFHOSTED_BACKEND_URL = envUrl.trim();
    }
  } catch (e) {
    console.warn("Could not get SELFHOSTED_BACKEND_URL from environment:", e);
  }
}

// Initialize Neutralino first
init();

// Setup backend URL, then render app
setupBackendUrl().then(() => {
  createRoot(document.getElementById("root")!).render(
    <StrictMode>
      <App />
    </StrictMode>
  );
});

