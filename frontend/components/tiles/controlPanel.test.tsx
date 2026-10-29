import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Device } from "@/lib/api";
import { I18nProvider } from "@/lib/i18n";
import { LONG_PRESS_MS } from "@/lib/useLongPress";
import { DeviceTile } from "./DeviceTile";
import { GroupTile } from "./GroupTile";

function light(id: string, name: string, on: boolean, extra: Partial<Device["capabilities"]> = {}): Device {
  return {
    id,
    name,
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
      ...extra,
    },
    availability: "online",
    state: on ? { state: "ON", brightness: 254, color_temp: 300 } : { state: "OFF" },
  };
}

// jsdom has no PointerEvent, so pointer events would lose clientX/clientY and
// finger movement could not be tested. Real browsers have it.
if (typeof window.PointerEvent === "undefined") {
  class PointerEventPolyfill extends MouseEvent {
    constructor(type: string, init?: MouseEventInit) {
      super(type, init);
    }
  }
  (window as unknown as { PointerEvent: typeof MouseEvent }).PointerEvent = PointerEventPolyfill;
}

let fetch: ReturnType<typeof vi.fn>;
beforeEach(() => {
  vi.useFakeTimers();
  fetch = vi.fn((_url: string, _init?: RequestInit) => Promise.resolve(new Response(null, { status: 202 })));
  vi.stubGlobal("fetch", fetch);
});
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.useRealTimers();
});

const sent = (i = 0) => ({
  url: fetch.mock.calls[i][0] as string,
  body: JSON.parse((fetch.mock.calls[i][1] as RequestInit).body as string),
});

/** Press and hold an element like a finger would. */
function longPress(el: Element) {
  fireEvent.pointerDown(el, { clientX: 10, clientY: 10 });
  act(() => vi.advanceTimersByTime(LONG_PRESS_MS + 50));
  fireEvent.pointerUp(el, { clientX: 10, clientY: 10 });
  fireEvent.click(el); // browsers still fire a click after the hold
}

function tap(el: Element) {
  fireEvent.pointerDown(el, { clientX: 10, clientY: 10 });
  act(() => vi.advanceTimersByTime(80));
  fireEvent.pointerUp(el, { clientX: 10, clientY: 10 });
  fireEvent.click(el);
}

const deviceTile = (device: Device, title?: string) =>
  render(
    <DeviceTile
      card={{ id: "d", type: "device", deviceId: device.id, title }}
      device={device}
      onPatch={() => {}}
      onError={() => {}}
    />,
  );

describe("device tile", () => {
  it("has no brightness slider on the tile itself", () => {
    deviceTile(light("0x01", "Taklampa", true));
    expect(screen.queryByRole("slider")).toBeNull();
    expect(screen.getByText("100%")).toBeTruthy(); // brightness still shown as text
  });

  it("a short tap toggles and does not open the panel", async () => {
    deviceTile(light("0x01", "Taklampa", false));
    tap(screen.getByRole("button", { name: "Turn Taklampa on" }));
    expect(screen.queryByRole("dialog")).toBeNull();
    await vi.waitFor(() => expect(fetch).toHaveBeenCalledTimes(1));
    expect(sent().body).toEqual({ state: "ON" });
  });

  it("press and hold opens the panel without toggling", () => {
    deviceTile(light("0x01", "Taklampa", true));
    longPress(screen.getByRole("button", { name: "Turn Taklampa off" }));
    const panel = screen.getByRole("dialog", { name: "Taklampa" });
    expect(within(panel).getByRole("slider", { name: /Brightness/ })).toBeTruthy();
    expect(fetch).not.toHaveBeenCalled();
  });

  it("moving the finger cancels the hold (scrolling)", () => {
    deviceTile(light("0x01", "Taklampa", true));
    const btn = screen.getByRole("button", { name: "Turn Taklampa off" });
    fireEvent.pointerDown(btn, { clientX: 10, clientY: 10 });
    fireEvent.pointerMove(btn, { clientX: 10, clientY: 60 });
    act(() => vi.advanceTimersByTime(LONG_PRESS_MS + 50));
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("right-click opens the panel (desktop)", () => {
    deviceTile(light("0x01", "Taklampa", true));
    fireEvent.contextMenu(screen.getByRole("button", { name: "Turn Taklampa off" }));
    expect(screen.getByRole("dialog", { name: "Taklampa" })).toBeTruthy();
  });

  it("the panel sets brightness and on/off", async () => {
    deviceTile(light("0x01", "Taklampa", true));
    longPress(screen.getByRole("button", { name: "Turn Taklampa off" }));
    const panel = screen.getByRole("dialog");

    const slider = within(panel).getByRole("slider", { name: /Brightness/ });
    fireEvent.change(slider, { target: { value: "127" } });
    fireEvent.pointerUp(slider);
    await vi.waitFor(() => expect(fetch).toHaveBeenCalledTimes(1));
    expect(sent(0)).toEqual({ url: expect.stringMatching(/\/api\/devices\/0x01\/set$/), body: { brightness: 127, state: "ON" } });

    fireEvent.click(within(panel).getByRole("button", { name: "Turn Taklampa off" }));
    await vi.waitFor(() => expect(fetch).toHaveBeenCalledTimes(2));
    expect(sent(1).body).toEqual({ state: "OFF" });
  });

  it("shows colour temperature for lamps that support it", () => {
    deviceTile(light("0x01", "Taklampa", true, { colorTemp: true, colorTempMin: 250, colorTempMax: 454 }));
    longPress(screen.getByRole("button", { name: "Turn Taklampa off" }));
    expect(within(screen.getByRole("dialog")).getByRole("slider", { name: /Warmth/ })).toBeTruthy();
  });

  it("closes with the Close button and with Escape", () => {
    deviceTile(light("0x01", "Taklampa", true));
    const btn = screen.getByRole("button", { name: "Turn Taklampa off" });
    longPress(btn);
    fireEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(screen.queryByRole("dialog")).toBeNull();

    longPress(btn);
    fireEvent.keyDown(window, { key: "Escape" });
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("is in Swedish when Swedish is chosen", () => {
    render(
      <I18nProvider language="sv">
        <DeviceTile
          card={{ id: "d", type: "device", deviceId: "0x01" }}
          device={light("0x01", "Taklampa", true)}
          onPatch={() => {}}
          onError={() => {}}
        />
      </I18nProvider>,
    );
    longPress(screen.getByRole("button", { name: "Stäng av Taklampa" }));
    const panel = screen.getByRole("dialog");
    expect(within(panel).getByRole("slider", { name: /Ljusstyrka/ })).toBeTruthy();
    expect(within(panel).getByRole("button", { name: "Stäng" })).toBeTruthy();
  });

  it("does nothing on hold in edit mode (preview)", () => {
    render(
      <DeviceTile
        card={{ id: "d", type: "device", deviceId: "0x01" }}
        device={light("0x01", "Taklampa", true)}
        onPatch={() => {}}
        onError={() => {}}
        preview
      />,
    );
    longPress(screen.getByRole("button", { name: "Turn Taklampa off" }));
    expect(screen.queryByRole("dialog")).toBeNull();
  });
});

describe("group tile", () => {
  const devices = { "0x01": light("0x01", "K1", true), "0x02": light("0x02", "K2", false) };
  const card = { id: "g", type: "group" as const, title: "Köket", deviceIds: ["0x01", "0x02"] };

  it("has no slider on the tile, and hold opens a panel that sets brightness for all", async () => {
    render(<GroupTile card={card} devices={devices} onPatch={() => {}} onError={() => {}} />);
    expect(screen.queryByRole("slider")).toBeNull();

    longPress(screen.getByRole("button", { name: "Turn Köket off" }));
    const panel = screen.getByRole("dialog", { name: "Köket" });
    expect(fetch).not.toHaveBeenCalled();

    const slider = within(panel).getByRole("slider", { name: /Brightness/ });
    fireEvent.change(slider, { target: { value: "60" } });
    fireEvent.pointerUp(slider);
    await vi.waitFor(() => expect(fetch).toHaveBeenCalledTimes(1));
    expect(sent()).toEqual({
      url: expect.stringMatching(/\/api\/devices\/set-many$/),
      body: { ids: ["0x01", "0x02"], payload: { brightness: 60, state: "ON" } },
    });
  });
});

describe("icons", () => {
  const icon = (container: HTMLElement) =>
    container.querySelector(".tile-icon svg")?.getAttribute("data-icon");

  it("a group uses the same bulb icons as a single lamp", () => {
    const card = { id: "g", type: "group" as const, title: "Köket", deviceIds: ["0x01", "0x02"] };
    const on = render(
      <GroupTile
        card={card}
        devices={{ "0x01": light("0x01", "K1", true), "0x02": light("0x02", "K2", false) }}
        onPatch={() => {}}
        onError={() => {}}
      />,
    );
    expect(icon(on.container)).toBe("bulb");
    cleanup();

    const off = render(
      <GroupTile
        card={card}
        devices={{ "0x01": light("0x01", "K1", false), "0x02": light("0x02", "K2", false) }}
        onPatch={() => {}}
        onError={() => {}}
      />,
    );
    expect(icon(off.container)).toBe("bulb-off");
  });

  it("matches the single lamp icons", () => {
    expect(icon(deviceTile(light("0x01", "L", true)).container)).toBe("bulb");
    cleanup();
    expect(icon(deviceTile(light("0x01", "L", false)).container)).toBe("bulb-off");
  });

  it("the group's hold-panel shows the same icon", () => {
    render(
      <GroupTile
        card={{ id: "g", type: "group", title: "Köket", deviceIds: ["0x01"] }}
        devices={{ "0x01": light("0x01", "K1", true) }}
        onPatch={() => {}}
        onError={() => {}}
      />,
    );
    longPress(screen.getByRole("button", { name: "Turn Köket off" }));
    expect(icon(screen.getByRole("dialog"))).toBe("bulb");
  });
});
