import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Device } from "@/lib/api";
import { DeviceCard } from "./DeviceCard";
import { MqttBanner } from "./MqttBanner";

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

describe("MqttBanner", () => {
  it("renders nothing when connected", () => {
    const { container } = render(
      <MqttBanner conn="open" mqtt={{ state: "connected", since: "" }} />,
    );
    expect(container.textContent).toBe("");
  });

  it("asks to configure MQTT and links to settings", () => {
    render(<MqttBanner conn="open" mqtt={{ state: "unconfigured", since: "" }} />);
    expect(screen.getByText(/MQTT is not configured/)).toBeTruthy();
    expect(screen.getByRole("link").getAttribute("href")).toBe("/settings/");
  });

  it("shows the connection error", () => {
    render(
      <MqttBanner
        conn="open"
        mqtt={{ state: "error", broker: "tcp://192.168.1.201:1883", error: "connection refused", since: "" }}
      />,
    );
    expect(screen.getByText(/Cannot connect to tcp:\/\/192.168.1.201:1883/)).toBeTruthy();
    expect(screen.getByText("connection refused")).toBeTruthy();
  });

  it("reports an unreachable backend", () => {
    render(<MqttBanner conn="closed" mqtt={null} />);
    expect(screen.getByText(/Backend unreachable/)).toBeTruthy();
  });
});

describe("DeviceCard", () => {
  const lamp: Device = {
    id: "0x01",
    name: "Ceiling",
    kind: "light",
    type: "Router",
    capabilities: {
      onOff: true,
      stateProperty: "state",
      brightness: true,
      brightnessMax: 254,
      colorTemp: false,
      color: false,
      power: false,
    },
    availability: "online",
    state: { state: "ON", brightness: 254 },
  };

  it("toggling an ON lamp sends OFF and updates optimistically", async () => {
    const fetch = vi.fn(() => Promise.resolve(new Response(null, { status: 202 })));
    vi.stubGlobal("fetch", fetch);
    const onPatch = vi.fn();
    render(<DeviceCard device={lamp} onPatch={onPatch} onError={() => {}} />);

    fireEvent.click(screen.getByRole("button", { name: /Turn Ceiling off/ }));

    expect(onPatch).toHaveBeenCalledWith("0x01", { state: "OFF" });
    await vi.waitFor(() => expect(fetch).toHaveBeenCalled());
    const [url, init] = fetch.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toMatch(/\/api\/devices\/0x01\/set$/);
    expect(JSON.parse(init.body as string)).toEqual({ state: "OFF" });
  });

  it("reports command errors instead of crashing", async () => {
    vi.stubGlobal("fetch", () =>
      Promise.resolve(new Response(JSON.stringify({ error: "not connected" }), { status: 503 })),
    );
    const onError = vi.fn();
    render(<DeviceCard device={lamp} onPatch={() => {}} onError={onError} />);

    fireEvent.click(screen.getByRole("button", { name: /Turn Ceiling off/ }));
    await vi.waitFor(() => expect(onError).toHaveBeenCalledWith("Ceiling: not connected"));
  });

  it("disables controls for offline devices", () => {
    render(
      <DeviceCard device={{ ...lamp, availability: "offline" }} onPatch={() => {}} onError={() => {}} />,
    );
    expect((screen.getByRole("button") as HTMLButtonElement).disabled).toBe(true);
  });
});
