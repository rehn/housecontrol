// Light / dark / automatic look, chosen per device (kept in this browser only).
// "auto" follows the device's own setting (prefers-color-scheme).

export type Theme = "light" | "dark" | "auto";
export const THEMES: Theme[] = ["light", "dark", "auto"];
export const THEME_KEY = "housecontrol.theme";

export function loadTheme(): Theme {
  try {
    const v = localStorage.getItem(THEME_KEY);
    return v === "light" || v === "dark" ? v : "auto";
  } catch {
    return "auto"; // storage blocked (private mode etc.)
  }
}

export function saveTheme(theme: Theme): void {
  try {
    if (theme === "auto") localStorage.removeItem(THEME_KEY);
    else localStorage.setItem(THEME_KEY, theme);
  } catch {
    // not remembered, but still applied for this visit
  }
}

/** light/dark: data-theme on <html> wins over the device; auto: no attribute. */
export function applyTheme(theme: Theme): void {
  const root = document.documentElement;
  if (theme === "auto") root.removeAttribute("data-theme");
  else root.setAttribute("data-theme", theme);
}

/** Inlined in <head> so the saved theme is in place before the first paint. */
export const themeScript = `try{var t=localStorage.getItem(${JSON.stringify(THEME_KEY)});if(t==="light"||t==="dark")document.documentElement.setAttribute("data-theme",t)}catch(e){}`;
