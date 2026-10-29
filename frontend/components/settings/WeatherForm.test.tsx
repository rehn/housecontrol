import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Place, WeatherSettings } from "@/lib/api";
import type { FoundPosition } from "@/lib/position";
import { I18nProvider } from "@/lib/i18n";
import { WeatherForm } from "./WeatherForm";

afterEach(cleanup);

const unconfigured: WeatherSettings = {
  provider: "smhi",
  latitude: 0,
  longitude: 0,
  location: "",
  configured: false,
  providers: ["smhi", "metno", "openmeteo"],
};

const places: Place[] = [
  { name: "Enköping", region: "Uppsala", country: "Sverige", latitude: 59.63607, longitude: 17.07768 },
  { name: "Enköpings-Näs", country: "Sverige", latitude: 59.6, longitude: 17.2 },
];

function setup(initial = unconfigured) {
  const onSave = vi.fn<(s: Omit<WeatherSettings, "configured" | "providers">) => Promise<void>>(() =>
    Promise.resolve(),
  );
  const search = vi.fn<(q: string) => Promise<Place[]>>(() => Promise.resolve(places));
  render(<WeatherForm initial={initial} onSave={onSave} search={search} />);
  return { onSave, search };
}

describe("WeatherForm", () => {
  it("has SMHI selected by default and lists the free providers", () => {
    setup();
    const smhi = screen.getByRole("radio", { name: /SMHI/ });
    expect(smhi.getAttribute("aria-checked")).toBe("true");
    expect(screen.getByRole("radio", { name: /MET Norway/ })).toBeTruthy();
    expect(screen.getByRole("radio", { name: /Open-Meteo/ })).toBeTruthy();
  });

  it("can't save before a place is chosen", () => {
    setup();
    expect((screen.getByRole("button", { name: "Save" }) as HTMLButtonElement).disabled).toBe(true);
  });

  it("searches for a place, picks it and saves with the chosen provider", async () => {
    const { onSave, search } = setup();
    fireEvent.click(screen.getByRole("radio", { name: /Open-Meteo/ }));
    fireEvent.change(screen.getByLabelText("Place"), { target: { value: "Enköping" } });
    fireEvent.click(screen.getByRole("button", { name: "Search" }));
    await vi.waitFor(() => expect(search).toHaveBeenCalledWith("Enköping"));

    fireEvent.click(await screen.findByRole("button", { name: /Enköping, Uppsala, Sverige/ }));
    expect(screen.getByText(/59\.6361, 17\.0777/)).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await vi.waitFor(() =>
      expect(onSave).toHaveBeenCalledWith({
        provider: "openmeteo",
        latitude: 59.63607,
        longitude: 17.07768,
        location: "Enköping",
      }),
    );
    expect(await screen.findByText("Saved.")).toBeTruthy();
  });

  it("shows when nothing was found", async () => {
    const onSave = vi.fn(() => Promise.resolve());
    render(<WeatherForm initial={unconfigured} onSave={onSave} search={() => Promise.resolve([])} />);
    fireEvent.change(screen.getByLabelText("Place"), { target: { value: "zzzz" } });
    fireEvent.click(screen.getByRole("button", { name: "Search" }));
    expect(await screen.findByText("No places found.")).toBeTruthy();
  });

  it("keeps the saved place and allows changing only the provider", async () => {
    const { onSave } = setup({ ...unconfigured, configured: true, location: "Enköping", latitude: 59.6, longitude: 17.1 });
    expect(screen.getByText(/Enköping/)).toBeTruthy();
    fireEvent.click(screen.getByRole("radio", { name: /MET Norway/ }));
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await vi.waitFor(() =>
      expect(onSave).toHaveBeenCalledWith({ provider: "metno", latitude: 59.6, longitude: 17.1, location: "Enköping" }),
    );
  });

  it("'Use my position' fills in the place and shows the area", async () => {
    const onSave = vi.fn<(s: Omit<WeatherSettings, "configured" | "providers">) => Promise<void>>(() =>
      Promise.resolve(),
    );
    const locate = vi.fn<() => Promise<FoundPosition>>(() =>
      Promise.resolve({ name: "Enköping", area: "Enköping, Uppsala län", latitude: 59.6361, longitude: 17.0777, approximate: false }),
    );
    render(<WeatherForm initial={unconfigured} onSave={onSave} search={() => Promise.resolve([])} locate={locate} />);

    fireEvent.click(screen.getByRole("button", { name: "Use my position" }));
    expect(await screen.findByText("Area: Enköping, Uppsala län")).toBeTruthy();
    expect(screen.queryByText(/Approximate position/)).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await vi.waitFor(() =>
      expect(onSave).toHaveBeenCalledWith({ provider: "smhi", latitude: 59.6361, longitude: 17.0777, location: "Enköping" }),
    );
  });

  it("warns when the position is only approximate (from the internet connection)", async () => {
    const locate = () =>
      Promise.resolve({ name: "Enköping", latitude: 59.6356, longitude: 17.0764, approximate: true });
    render(
      <WeatherForm initial={unconfigured} onSave={() => Promise.resolve()} search={() => Promise.resolve([])} locate={locate} />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Use my position" }));
    expect(await screen.findByText(/Approximate position/)).toBeTruthy();
  });

  it("shows an error if no position could be found", async () => {
    const locate = () => Promise.reject(new Error("all IP services down"));
    render(
      <WeatherForm initial={unconfigured} onSave={() => Promise.resolve()} search={() => Promise.resolve([])} locate={locate} />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Use my position" }));
    expect(await screen.findByText("Could not find your position: all IP services down")).toBeTruthy();
  });

  it("shows the saved area", () => {
    setup({ ...unconfigured, configured: true, location: "Enköping", latitude: 59.6, longitude: 17.1, area: "Enköping, Uppsala län" });
    expect(screen.getByText("Area: Enköping, Uppsala län")).toBeTruthy();
  });

  it("is in Swedish when Swedish is chosen", () => {
    render(
      <I18nProvider language="sv">
        <WeatherForm
          initial={unconfigured}
          onSave={() => Promise.resolve()}
          search={() => Promise.resolve([])}
          locate={() => Promise.reject(new Error("x"))}
        />
      </I18nProvider>,
    );
    expect(screen.getByLabelText("Plats")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Sök" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Använd min position" })).toBeTruthy();
    expect(screen.getByText(/Sverige och Norden/)).toBeTruthy();
  });
});
