export interface ThemeVars {
  "--de-bg": string;
  "--de-bg-secondary": string;
  "--de-fg": string;
  "--de-fg-muted": string;
  "--de-accent": string;
  "--de-accent-hover": string;
  "--de-accent-fg": string;
  "--de-border": string;
  "--de-success": string;
  "--de-error": string;
  "--de-overlay": string;
  "--de-radius": string;
  "--de-shadow": string;
}

const lightVars: ThemeVars = {
  "--de-bg": "#ffffff",
  "--de-bg-secondary": "#f7f8fa",
  "--de-fg": "#1a1a2e",
  "--de-fg-muted": "#6b7280",
  "--de-accent": "#2563eb",
  "--de-accent-hover": "#1d4ed8",
  "--de-accent-fg": "#ffffff",
  "--de-border": "#e5e7eb",
  "--de-success": "#16a34a",
  "--de-error": "#dc2626",
  "--de-overlay": "rgba(0, 0, 0, 0.5)",
  "--de-radius": "8px",
  "--de-shadow": "0 20px 60px rgba(0, 0, 0, 0.15), 0 4px 16px rgba(0, 0, 0, 0.08)",
};

const darkVars: ThemeVars = {
  "--de-bg": "#1a1a2e",
  "--de-bg-secondary": "#232340",
  "--de-fg": "#e5e7eb",
  "--de-fg-muted": "#9ca3af",
  "--de-accent": "#3b82f6",
  "--de-accent-hover": "#60a5fa",
  "--de-accent-fg": "#ffffff",
  "--de-border": "#374151",
  "--de-success": "#22c55e",
  "--de-error": "#ef4444",
  "--de-overlay": "rgba(0, 0, 0, 0.7)",
  "--de-radius": "8px",
  "--de-shadow": "0 20px 60px rgba(0, 0, 0, 0.4), 0 4px 16px rgba(0, 0, 0, 0.25)",
};

/** Detect system preference. */
function detectPreference(): "light" | "dark" {
  if (typeof window === "undefined") return "light";
  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

/** Resolve theme option to concrete light/dark. */
export function resolveTheme(theme?: "light" | "dark" | "auto"): "light" | "dark" {
  if (theme === "light" || theme === "dark") return theme;
  return detectPreference();
}

/** Get CSS variables for a resolved theme, with optional accent override. */
export function getThemeVars(resolved: "light" | "dark", accentColor?: string): ThemeVars {
  const vars = resolved === "dark" ? { ...darkVars } : { ...lightVars };
  if (accentColor) {
    vars["--de-accent"] = accentColor;
    // Keep accent-hover and accent-fg as defaults since we can't
    // reliably derive hover/contrast colors from an arbitrary input.
  }
  return vars;
}

/** Apply theme variables to a host element. */
export function applyThemeVars(el: HTMLElement, vars: ThemeVars): void {
  for (const [key, value] of Object.entries(vars)) {
    el.style.setProperty(key, value);
  }
}
