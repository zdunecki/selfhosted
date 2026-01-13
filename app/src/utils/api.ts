let cachedApiBaseUrl: string | null = null;

/**
 * Get the API base URL.
 * In desktop mode, uses SELFHOSTED_BACKEND_URL from Neutralino environment.
 * In web mode, uses relative URLs (same origin).
 */
export async function getApiBaseUrl(): Promise<string> {
  if (cachedApiBaseUrl !== null) {
    return cachedApiBaseUrl;
  }

  // Check if we're in a browser environment
  if (typeof window === "undefined") {
    cachedApiBaseUrl = "";
    return "";
  }

  // Check for backend URL in window (set by desktop main.tsx)
  const backendUrl = (window as any).SELFHOSTED_BACKEND_URL;
  if (backendUrl && typeof backendUrl === "string") {
    cachedApiBaseUrl = String(backendUrl).replace(/\/$/, "");
    return cachedApiBaseUrl;
  }

  // Try to get from Neutralino environment (for desktop mode)
  try {
    // @ts-ignore - Neutralino global
    if (typeof window.Neutralino !== "undefined") {
      // @ts-ignore
      const Neutralino = window.Neutralino;
      const envUrl = await Neutralino.os.getEnv("SELFHOSTED_BACKEND_URL");
      if (envUrl && typeof envUrl === "string" && envUrl.trim()) {
        cachedApiBaseUrl = envUrl.trim().replace(/\/$/, "");
        return cachedApiBaseUrl;
      }
    }
  } catch (e) {
    // Not in Neutralino or env var not set, continue to default
  }

  // Default: use relative URLs (same origin)
  cachedApiBaseUrl = "";
  return "";
}

/**
 * Make an API request with the correct base URL
 */
export async function apiFetch<T>(
  path: string,
  options?: RequestInit
): Promise<T> {
  const baseUrl = await getApiBaseUrl();
  const url = `${baseUrl}${path.startsWith("/") ? path : "/" + path}`;
  const res = await fetch(url, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...options?.headers,
    },
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(`API request failed: ${res.status} ${text}`);
  }

  // Handle non-JSON responses (like empty responses)
  const contentType = res.headers.get("content-type");
  if (contentType) {
    if (contentType.includes("application/json")) {
      const data = await res.json();
      // Ensure arrays are actually arrays (defensive check)
      if (Array.isArray(data)) {
        return data as T;
      }
      return data as T;
    } else if (contentType.includes("text/plain")) {
      const text = await res.text();

      try {
        const data = JSON.parse(text) as T;
        if (Array.isArray(data)) {
          return data as T;
        }
        return data as T;
      } catch (e) {}
    }
  }
  // For non-JSON responses, return empty object (or handle differently)
  return {} as T;
}

/**
 * Check if we're running in desktop mode (Neutralino)
 */
export function isDesktopMode(): boolean {
  if (typeof window === "undefined") {
    return false;
  }

  // Check for backend URL in window (set by desktop main.tsx)
  const backendUrl = (window as any).SELFHOSTED_BACKEND_URL;
  if (backendUrl && typeof backendUrl === "string") {
    return true;
  }

  // Check for Neutralino global
  try {
    // @ts-ignore - Neutralino global
    if (typeof window.Neutralino !== "undefined") {
      return true;
    }
  } catch (e) {
    // Not in Neutralino
  }

  return false;
}

/**
 * Get asset URL (handles desktop vs web mode)
 * In desktop mode, prefixes with backend URL. In web mode, uses relative path.
 */
export function getAssetUrl(path: string): string {
  // Check if we're in a browser environment
  if (typeof window === "undefined") {
    return path;
  }

  // Check for backend URL in window (set by desktop main.tsx)
  // @ts-ignore - SELFHOSTED_BACKEND_URL is set dynamically
  const backendUrl = (window as any).SELFHOSTED_BACKEND_URL;
  if (backendUrl && typeof backendUrl === "string") {
    return `${String(backendUrl).replace(/\/$/, "")}${path}`;
  }

  // Default: use relative path (same origin)
  return path;
}
