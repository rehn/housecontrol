import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { I18nProvider } from "@/lib/i18n";
import { THEME_KEY } from "@/lib/theme";
import { ThemePicker } from "./ThemePicker";

afterEach(() => {
  cleanup();
  localStorage.clear();
  document.documentElement.removeAttribute("data-theme");
});

describe("ThemePicker", () => {
  it("offers light, dark and automatic, with automatic chosen at first", () => {
    render(<ThemePicker />);
    const radios = screen.getAllByRole("radio");
    expect(radios.map((r) => r.textContent)).toEqual(["Light", "Dark", "Automatic"]);
    expect(screen.getByRole("radio", { name: "Automatic" }).getAttribute("aria-checked")).toBe("true");
  });

  it("switches at once and remembers it on this device", () => {
    render(<ThemePicker />);
    fireEvent.click(screen.getByRole("radio", { name: "Dark" }));
    expect(document.documentElement.dataset.theme).toBe("dark");
    expect(localStorage.getItem(THEME_KEY)).toBe("dark");
    expect(screen.getByRole("radio", { name: "Dark" }).getAttribute("aria-checked")).toBe("true");

    fireEvent.click(screen.getByRole("radio", { name: "Automatic" }));
    expect(document.documentElement.hasAttribute("data-theme")).toBe(false);
  });

  it("shows the saved choice", () => {
    localStorage.setItem(THEME_KEY, "light");
    render(<ThemePicker />);
    expect(screen.getByRole("radio", { name: "Light" }).getAttribute("aria-checked")).toBe("true");
  });

  it("is in Swedish", () => {
    render(
      <I18nProvider language="sv">
        <ThemePicker />
      </I18nProvider>,
    );
    expect(screen.getAllByRole("radio").map((r) => r.textContent)).toEqual(["Ljust", "Mörkt", "Automatiskt"]);
  });
});
