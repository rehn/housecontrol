import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { I18nProvider } from "@/lib/i18n";
import { SettingsLayout, type SettingsSection } from "./SettingsLayout";

const sections: SettingsSection[] = [
  { id: "general", label: "General", content: <p>title form</p> },
  { id: "weather", label: "Weather", content: <p>weather form</p> },
  { id: "mqtt", label: "MQTT", content: <p>mqtt form</p> },
];

beforeEach(() => history.replaceState(null, "", "/settings/"));
afterEach(cleanup);

describe("SettingsLayout", () => {
  it("lists every section in a menu and shows the first one", () => {
    render(<SettingsLayout sections={sections} />);
    const menu = screen.getByRole("navigation", { name: "Settings menu" });
    expect([...menu.querySelectorAll("a")].map((a) => a.textContent)).toEqual(["General", "Weather", "MQTT"]);
    expect(screen.getByRole("link", { name: "General" }).getAttribute("aria-current")).toBe("page");
    expect(screen.getByText("title form")).toBeTruthy();
    expect(screen.queryByText("weather form")).toBeNull();
  });

  it("shows only the chosen section, with its name as heading", () => {
    render(<SettingsLayout sections={sections} />);
    fireEvent.click(screen.getByRole("link", { name: "Weather" }));
    expect(screen.getByText("weather form")).toBeTruthy();
    expect(screen.queryByText("title form")).toBeNull();
    expect(screen.getByRole("heading", { name: "Weather" })).toBeTruthy();
    expect(screen.getByRole("link", { name: "Weather" }).getAttribute("aria-current")).toBe("page");
    expect(screen.getByRole("link", { name: "General" }).getAttribute("aria-current")).toBeNull();
    expect(location.hash).toBe("#weather"); // reload or share = same section
  });

  it("opens the section named in the address", () => {
    history.replaceState(null, "", "/settings/#mqtt");
    render(<SettingsLayout sections={sections} />);
    expect(screen.getByText("mqtt form")).toBeTruthy();
  });

  it("follows the browser's back button", () => {
    render(<SettingsLayout sections={sections} />);
    act(() => {
      history.replaceState(null, "", "/settings/#weather");
      window.dispatchEvent(new HashChangeEvent("hashchange"));
    });
    expect(screen.getByText("weather form")).toBeTruthy();
  });

  it("falls back to the first section for an unknown address", () => {
    history.replaceState(null, "", "/settings/#nope");
    render(<SettingsLayout sections={sections} />);
    expect(screen.getByText("title form")).toBeTruthy();
  });

  it("names the menu in Swedish", () => {
    render(
      <I18nProvider language="sv">
        <SettingsLayout sections={sections} />
      </I18nProvider>,
    );
    expect(screen.getByRole("navigation", { name: "Inställningsmeny" })).toBeTruthy();
  });
});
