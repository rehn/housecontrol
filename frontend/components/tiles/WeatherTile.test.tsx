import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { WeatherForecast } from "@/lib/api";
import { I18nProvider } from "@/lib/i18n";
import { WeatherTile } from "./WeatherTile";

vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

const forecast: WeatherForecast = {
  provider: "smhi",
  location: "Enköping",
  updated: "2026-10-28T21:16:00+01:00",
  current: { temperature: 7.4, condition: "partly_cloudy" },
  daily: [
    { date: "2026-10-28", min: 5.7, max: 7.4, condition: "partly_cloudy" },
    { date: "2026-10-29", min: 3.7, max: 10.3, condition: "cloudy" },
    { date: "2026-10-30", min: 3.5, max: 9.9, condition: "clear" },
    { date: "2026-10-31", min: 2.3, max: 8.5, condition: "partly_cloudy" },
    { date: "2026-11-01", min: 4.8, max: 7.7, condition: "cloudy" },
  ],
};

function stubFetch(status: number, body: unknown) {
  const fetch = vi.fn(() => Promise.resolve(new Response(JSON.stringify(body), { status })));
  vi.stubGlobal("fetch", fetch);
  return fetch;
}

describe("WeatherTile", () => {
  it("shows current weather and five days", async () => {
    stubFetch(200, forecast);
    render(<WeatherTile />);
    expect(await screen.findByText("Partly cloudy")).toBeTruthy();
    expect(screen.getByText("7.4 °C")).toBeTruthy();
    expect(screen.getByText(/Enköping/)).toBeTruthy();
    expect(screen.getByText(/SMHI/)).toBeTruthy();

    const days = screen.getAllByTestId("weather-day");
    expect(days).toHaveLength(5);
    expect(within(days[0]).getByText("Wed")).toBeTruthy();
    expect(within(days[1]).getByText("10.3°")).toBeTruthy();
    expect(within(days[1]).getByText("3.7°")).toBeTruthy();
  });

  it("shows the area from Settings instead of the place name", async () => {
    stubFetch(200, { ...forecast, area: "Enköping, Uppsala län" });
    render(<WeatherTile />);
    expect(await screen.findByText("Enköping, Uppsala län · SMHI")).toBeTruthy();
  });

  it("uses the chosen language for texts, weekdays and decimals", async () => {
    stubFetch(200, forecast);
    render(
      <I18nProvider language="sv">
        <WeatherTile />
      </I18nProvider>,
    );
    expect(await screen.findByText("Växlande molnighet")).toBeTruthy();
    expect(screen.getByText("7,4 °C")).toBeTruthy();
    expect(within(screen.getAllByTestId("weather-day")[0]).getByText("ons")).toBeTruthy();
  });

  it("asks to choose a place when weather isn't set up", async () => {
    stubFetch(404, { error: "weather is not configured", code: "weather_not_configured" });
    render(<WeatherTile />);
    expect(await screen.findByText(/Choose a place/)).toBeTruthy();
    expect(screen.getByRole("link").getAttribute("href")).toBe("/settings/");
  });

  it("says when the weather service is down", async () => {
    stubFetch(502, { error: "SMHI: weather service answered 503", code: "weather_unavailable" });
    render(<WeatherTile />);
    expect(await screen.findByText(/Weather is unavailable/)).toBeTruthy();
  });

  it("marks an old (cached) forecast", async () => {
    stubFetch(200, { ...forecast, stale: true });
    render(<WeatherTile />);
    expect(await screen.findByText(/latest forecast/)).toBeTruthy();
  });

  it("reloads when the settings change (refreshKey)", async () => {
    const fetch = stubFetch(200, forecast);
    const { rerender } = render(<WeatherTile refreshKey={0} />);
    await screen.findByText("Partly cloudy");
    rerender(<WeatherTile refreshKey={1} />);
    await vi.waitFor(() => expect(fetch).toHaveBeenCalledTimes(2));
  });
});
