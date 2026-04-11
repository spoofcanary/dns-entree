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
/** Resolve theme option to concrete light/dark. */
export declare function resolveTheme(theme?: "light" | "dark" | "auto"): "light" | "dark";
/** Get CSS variables for a resolved theme, with optional accent override. */
export declare function getThemeVars(resolved: "light" | "dark", accentColor?: string): ThemeVars;
/** Apply theme variables to a host element. */
export declare function applyThemeVars(el: HTMLElement, vars: ThemeVars): void;
