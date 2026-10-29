import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Device } from "@/lib/api";
import type { Dashboard } from "@/lib/dashboard";
import { ClockTile } from "@/components/tiles/ClockTile";
import { DeviceTile } from "@/components/tiles/DeviceTile";
import { GroupTile } from "@/components/tiles/GroupTile";
import { DashboardView } from "./DashboardView";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.useRealTimers();
});

function light(id: string, name: string, on: boolean): Device {
  return {
    id,
    name,
    kind: "light",
    type: "Router",
    capabilities: { onOff: true, stateProperty: "state", brightness: true, brightnessMax: 254, colorTemp: false, color: false, power: false },
    availability: "online",
    state: on ? { state: "ON", brightness: 200 } : { state: "OFF" },
  };
}

const k1 = light("0x01", "Kitchen 1", true);
const k2 = light("0x02", "Kitchen 2", false);
const byId = { [k1.id]: k1, [k2.id]: k2 };

function stubFetch() {
  const fetch = vi.fn((_url: string, _init?: RequestInit) => Promise.resolve(new Response(null, { status: 202 })));
  vi.stubGlobal("fetch", fetch);
  return fetch;
}
const body = (fetch: ReturnType<typeof stubFetch>, i = 0) => JSON.parse(fetch.mock.calls[i][1]?.body as string);

describe("GroupTile", () => {
  const card = { id: "g", type: "group" as const, title: "Köket", deviceIds: ["0x01", "0x02"] };

  it("shows how many members are on", () => {
    render(<GroupTile card={card} devices={byId} onPatch={() => {}} onError={() => {}} />);
    expect(screen.getByText("Köket")).toBeTruthy();
    expect(screen.getByText("1 of 2 on")).toBeTruthy();
  });

  it("one tap turns the whole group off when any light is on", async () => {
    const fetch = stubFetch();
    const onPatch = vi.fn();
    render(<GroupTile card={card} devices={byId} onPatch={onPatch} onError={() => {}} />);

    fireEvent.click(screen.getByRole("button", { name: "Turn Köket off" }));

    await vi.waitFor(() => expect(fetch).toHaveBeenCalledTimes(1));
    expect(fetch.mock.calls[0][0]).toMatch(/\/api\/devices\/set-many$/);
    expect(body(fetch)).toEqual({ ids: ["0x01", "0x02"], payload: { state: "OFF" } });
    expect(onPatch).toHaveBeenCalledWith("0x01", { state: "OFF" });
    expect(onPatch).toHaveBeenCalledWith("0x02", { state: "OFF" });
  });

  it("turns everything on when all are off", async () => {
    const fetch = stubFetch();
    const offK1 = light("0x01", "Kitchen 1", false);
    render(<GroupTile card={card} devices={{ ...byId, "0x01": offK1 }} onPatch={() => {}} onError={() => {}} />);
    fireEvent.click(screen.getByRole("button", { name: "Turn Köket on" }));
    await vi.waitFor(() => expect(body(fetch).payload).toEqual({ state: "ON" }));
  });
});

describe("DeviceTile", () => {
  it("uses the card title if set and toggles the device", async () => {
    const fetch = stubFetch();
    render(
      <DeviceTile
        card={{ id: "d", type: "device", deviceId: "0x02", title: "Tvättstuga" }}
        device={k2}
        onPatch={() => {}}
        onError={() => {}}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Turn Tvättstuga on" }));
    await vi.waitFor(() => expect(fetch).toHaveBeenCalled());
    expect(fetch.mock.calls[0][0]).toMatch(/\/api\/devices\/0x02\/set$/);
    expect(body(fetch)).toEqual({ state: "ON" });
  });
});

describe("ClockTile", () => {
  it("shows the current time and keeps ticking", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 9, 28, 22, 16, 7));
    render(<ClockTile />);
    expect(screen.getByTestId("clock-time").textContent).toBe("22:16");
    act(() => vi.advanceTimersByTime(60_000));
    expect(screen.getByTestId("clock-time").textContent).toBe("22:17");
  });
});

describe("DashboardView", () => {
  const dashboard: Dashboard = {
    version: 1,
    columns: [
      { id: "c1", cards: [{ id: "clock", type: "clock" }, { id: "d1", type: "device", deviceId: "0x01" }] },
      { id: "c2", cards: [{ id: "d2", type: "device", deviceId: "0x02" }] },
    ],
  };
  const devices = [k1, k2];

  const saveMock = () => vi.fn<(d: Dashboard) => Promise<void>>(() => Promise.resolve());

  function renderView(onSave = saveMock()) {
    const onCancel = vi.fn();
    render(
      <DashboardView
        dashboard={dashboard}
        devices={devices}
        editing
        onSave={onSave}
        onCancel={onCancel}
        onPatch={() => {}}
        onError={() => {}}
      />,
    );
    return { onSave, onCancel };
  }

  it("removes a card and saves the new layout", async () => {
    const { onSave } = renderView();
    fireEvent.click(screen.getByRole("button", { name: "Remove Kitchen 1" }));
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await vi.waitFor(() => expect(onSave).toHaveBeenCalled());
    const saved = onSave.mock.calls[0][0];
    expect(saved.columns.flatMap((c) => c.cards.map((x) => x.id))).toEqual(["clock", "d2"]);
  });

  it("adds a group card through the dialog", async () => {
    const { onSave } = renderView();
    fireEvent.click(screen.getByRole("button", { name: "Add card" }));
    const dialog = screen.getByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: /Group/ }));
    fireEvent.change(within(dialog).getByLabelText("Name"), { target: { value: "Köket" } });
    fireEvent.click(within(dialog).getByRole("checkbox", { name: "Kitchen 1" }));
    fireEvent.click(within(dialog).getByRole("checkbox", { name: "Kitchen 2" }));
    fireEvent.click(within(dialog).getByRole("button", { name: "Add" }));

    expect(screen.queryByRole("dialog")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await vi.waitFor(() => expect(onSave).toHaveBeenCalled());
    const group = onSave.mock.calls[0][0].columns
      .flatMap((c) => c.cards)
      .find((c) => c.type === "group");
    expect(group).toMatchObject({ title: "Köket", deviceIds: ["0x01", "0x02"] });
  });

  it("adds a weather card in one tap", async () => {
    const { onSave } = renderView();
    fireEvent.click(screen.getByRole("button", { name: "Add card" }));
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: /Weather/ }));
    expect(screen.queryByRole("dialog")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await vi.waitFor(() => expect(onSave).toHaveBeenCalled());
    expect(onSave.mock.calls[0][0].columns.flatMap((c) => c.cards).some((c) => c.type === "weather")).toBe(true);
  });

  it("adds a calendar card in one tap", async () => {
    const { onSave } = renderView();
    fireEvent.click(screen.getByRole("button", { name: "Add card" }));
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: /Calendar/ }));
    expect(screen.queryByRole("dialog")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await vi.waitFor(() => expect(onSave).toHaveBeenCalled());
    expect(onSave.mock.calls[0][0].columns.flatMap((c) => c.cards).some((c) => c.type === "calendar")).toBe(true);
  });

  it("cannot add a group without a name or devices", () => {
    renderView();
    fireEvent.click(screen.getByRole("button", { name: "Add card" }));
    const dialog = screen.getByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: /Group/ }));
    expect((within(dialog).getByRole("button", { name: "Add" }) as HTMLButtonElement).disabled).toBe(true);
  });

  it("changes the number of columns", async () => {
    const { onSave } = renderView();
    fireEvent.click(screen.getByRole("button", { name: "More columns" }));
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await vi.waitFor(() => expect(onSave).toHaveBeenCalled());
    expect(onSave.mock.calls[0][0].columns).toHaveLength(3);
  });

  it("cancel discards changes", () => {
    const { onSave, onCancel } = renderView();
    fireEvent.click(screen.getByRole("button", { name: "Remove Kitchen 1" }));
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(onCancel).toHaveBeenCalled();
    expect(onSave).not.toHaveBeenCalled();
  });

  it("does not show edit controls outside edit mode", () => {
    render(
      <DashboardView
        dashboard={dashboard}
        devices={devices}
        editing={false}
        onSave={saveMock()}
        onCancel={vi.fn()}
        onPatch={() => {}}
        onError={() => {}}
      />,
    );
    expect(screen.queryByRole("button", { name: /Remove/ })).toBeNull();
    expect(screen.getByRole("button", { name: "Turn Kitchen 1 off" })).toBeTruthy();
  });
});
