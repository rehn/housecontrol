import { afterEach, describe, expect, it, vi } from "vitest";
import { applyTheme, loadTheme, saveTheme, THEME_KEY, themeScript } from "./theme";

afterEach(() => {
  localStorage.clear();
  document.documentElement.removeAttribute("data-theme");
  vi.restoreAllMocks();
});

describe("theme", () => {
  it("is automatic (follows the device) until something is chosen", () => {
    expect(loadTheme()).toBe("auto");
  });

  it("remembers the choice on this device", () => {
    saveTheme("dark");
    expect(localStorage.getItem(THEME_KEY)).toBe("dark");
    expect(loadTheme()).toBe("dark");
    saveTheme("auto");
    expect(loadTheme()).toBe("auto");
  });

  it("ignores junk in storage", () => {
    localStorage.setItem(THEME_KEY, "purple");
    expect(loadTheme()).toBe("auto");
  });

  it("still works when the browser blocks storage", () => {
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new Error("blocked");
    });
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new Error("blocked");
    });
    expect(loadTheme()).toBe("auto");
    expect(() => saveTheme("dark")).not.toThrow();
  });

  it("sets data-theme on <html> for light and dark, and removes it for auto", () => {
    applyTheme("dark");
    expect(document.documentElement.dataset.theme).toBe("dark");
    applyTheme("light");
    expect(document.documentElement.dataset.theme).toBe("light");
    applyTheme("auto");
    expect(document.documentElement.hasAttribute("data-theme")).toBe(false);
  });

  it("has a tiny script that applies the saved theme before the page is drawn (no flash)", () => {
    localStorage.setItem(THEME_KEY, "dark");
    new Function(themeScript)();
    expect(document.documentElement.dataset.theme).toBe("dark");
    localStorage.setItem(THEME_KEY, "junk");
    document.documentElement.removeAttribute("data-theme");
    new Function(themeScript)();
    expect(document.documentElement.hasAttribute("data-theme")).toBe(false);
  });
});
