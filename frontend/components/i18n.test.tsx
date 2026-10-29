import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Device } from "@/lib/api";
import { I18nProvider } from "@/lib/i18n";
import { MqttBanner } from "./MqttBanner";
import { ClockTile } from "./tiles/ClockTile";
import { DeviceTile } from "./tiles/DeviceTile";
import { GroupTile } from "./tiles/GroupTile";

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
  vi.useRealTimers();
});

const sv = (ui: React.ReactNode) => render(<I18nProvider language="sv">{ui}</I18nProvider>);

function light(id: string, name: string, on: boolean): Device {
  return {
    id,
    name,
    kind: "light",
    type: "Router",
    capabilities: { onOff: true, stateProperty: "state", brightness: true, brightnessMax: 254, colorTemp: false, color: false, power: false },
    availability: "online",
    state: on ? { state: "ON", brightness: 254 } : { state: "OFF" },
  };
}

describe("Swedish UI", () => {
  it("group tile", () => {
    const devices = { "0x01": light("0x01", "K1", true), "0x02": light("0x02", "K2", false) };
    sv(
      <GroupTile
        card={{ id: "g", type: "group", title: "Köket", deviceIds: ["0x01", "0x02"] }}
        devices={devices}
        onPatch={() => {}}
        onError={() => {}}
      />,
    );
    expect(screen.getByText("1 av 2 på")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Stäng av Köket" })).toBeTruthy();
    expect(screen.queryByRole("slider")).toBeNull(); // brightness lives in the hold-panel now
  });

  it("user-chosen names are not translated", () => {
    sv(
      <DeviceTile
        card={{ id: "d", type: "device", deviceId: "0x01", title: "Kitchen lamp" }}
        device={light("0x01", "x", false)}
        onPatch={() => {}}
        onError={() => {}}
      />,
    );
    expect(screen.getByRole("button", { name: "Slå på Kitchen lamp" })).toBeTruthy();
  });

  it("backend error codes are shown in Swedish", async () => {
    vi.stubGlobal("fetch", () =>
      Promise.resolve(
        new Response(JSON.stringify({ error: "not connected to the MQTT broker", code: "mqtt_not_connected" }), {
          status: 503,
        }),
      ),
    );
    const onError = vi.fn();
    sv(
      <DeviceTile
        card={{ id: "d", type: "device", deviceId: "0x01" }}
        device={light("0x01", "Taklampa", false)}
        onPatch={() => {}}
        onError={onError}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Slå på Taklampa" }));
    await vi.waitFor(() => expect(onError).toHaveBeenCalledWith("Taklampa: Inte ansluten till MQTT-brokern"));
  });

  it("MQTT banner", () => {
    sv(<MqttBanner conn="open" mqtt={{ state: "unconfigured", since: "" }} />);
    expect(screen.getByText(/MQTT är inte konfigurerat/)).toBeTruthy();
    expect(screen.getByRole("link", { name: "MQTT-inställningar" })).toBeTruthy();
  });

  it("clock shows the date in Swedish", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 9, 28, 22, 16, 7));
    sv(<ClockTile />);
    expect(screen.getByText("onsdag 28 oktober 2026")).toBeTruthy();
  });

  it("English is the default without a provider", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 9, 28, 22, 16, 7));
    render(<ClockTile />);
    expect(screen.getByText(/^Wednesday,? 28 October 2026$/)).toBeTruthy();
  });
});
