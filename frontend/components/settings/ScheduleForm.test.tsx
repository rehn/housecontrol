import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Device } from "@/lib/api";
import { I18nProvider } from "@/lib/i18n";
import type { Schedule, SchedulesView } from "@/lib/schedules";
import { ScheduleForm } from "./ScheduleForm";

afterEach(cleanup);

const lamp = (id: string, name: string, brightness = true): Device =>
  ({
    id,
    name,
    kind: "light",
    type: "Router",
    capabilities: { onOff: true, brightness, colorTemp: false, color: false, power: false },
    availability: "online",
    state: {},
  }) as Device;
const devices = [lamp("0x01", "Ute vägg"), lamp("0x02", "Ute trappa"), lamp("0x03", "Hall")];

const ute: Schedule = {
  id: "s1",
  name: "Ute",
  enabled: true,
  deviceIds: ["0x01", "0x02"],
  days: [1, 2, 3, 4, 5],
  on: { kind: "sunset", offset: -30, notBefore: "16:00" },
  off: { kind: "time", time: "23:30" },
  brightness: 40,
};

const view = (schedules: Schedule[] = [ute], known = true): SchedulesView => ({
  schedules,
  place: known
    ? { known: true, name: "Enköping", sunrise: "2026-10-29T07:12:00+01:00", sunset: "2026-10-29T16:23:00+01:00" }
    : { known: false },
  next: { s1: { action: "on", at: "2026-10-29T16:00:00+01:00" } },
});

function setup(initial = view()) {
  const onSave = vi.fn((list: Schedule[]) => Promise.resolve({ ...initial, schedules: list }));
  render(<ScheduleForm initial={initial} devices={devices} onSave={onSave} now={new Date("2026-10-29T12:00:00+01:00")} />);
  return { onSave };
}

describe("ScheduleForm", () => {
  it("lists schedules with what they do, on which days and when next", () => {
    setup();
    const row = screen.getByRole("listitem");
    expect(within(row).getByText("Ute")).toBeTruthy();
    expect(within(row).getByText(/On 30 min before sunset, not before 16:00/)).toBeTruthy();
    expect(within(row).getByText(/Off 23:30/)).toBeTruthy();
    expect(within(row).getByText(/Weekdays/)).toBeTruthy();
    expect(within(row).getByText(/turns on today 16:00/)).toBeTruthy();
    expect(within(row).getByText(/Ute vägg, Ute trappa/)).toBeTruthy();
    expect(screen.getByText("Today in Enköping: sunrise 07:12, sunset 16:23")).toBeTruthy();
  });

  it("switches a schedule off and on at once", async () => {
    const { onSave } = setup();
    fireEvent.click(screen.getByRole("switch", { name: "Ute on" }));
    await vi.waitFor(() => expect(onSave).toHaveBeenCalled());
    expect(onSave.mock.calls[0][0][0].enabled).toBe(false);
  });

  it("adds a schedule with sun, limits, days and brightness", async () => {
    const { onSave } = setup(view([]));
    fireEvent.click(screen.getByRole("button", { name: "Add schedule" }));
    const dialog = screen.getByRole("dialog", { name: "New schedule" });
    fireEvent.change(within(dialog).getByLabelText("Name"), { target: { value: " Kväll " } });
    fireEvent.click(within(dialog).getByRole("checkbox", { name: "Hall" }));
    fireEvent.click(within(dialog).getByRole("checkbox", { name: "Sat" }));
    fireEvent.click(within(dialog).getByRole("checkbox", { name: "Sun" }));

    const on = within(dialog).getByRole("group", { name: "Turn on" });
    fireEvent.click(within(on).getByRole("checkbox", { name: "Turn on" }));
    fireEvent.click(within(on).getByRole("radio", { name: "Sunset" }));
    fireEvent.change(within(on).getByLabelText("Minutes"), { target: { value: "30" } });
    fireEvent.change(within(on).getByLabelText("before / after"), { target: { value: "before" } });
    fireEvent.change(within(on).getByLabelText(/Not before/), { target: { value: "16:00" } });

    const off = within(dialog).getByRole("group", { name: "Turn off" });
    fireEvent.click(within(off).getByRole("checkbox", { name: "Turn off" }));
    fireEvent.change(within(off).getByLabelText("Time"), { target: { value: "23:30" } });

    fireEvent.click(within(dialog).getByRole("checkbox", { name: "Set brightness" }));
    fireEvent.change(within(dialog).getByLabelText(/^Brightness/), { target: { value: "40" } });

    fireEvent.click(within(dialog).getByRole("button", { name: "Add" }));
    await vi.waitFor(() => expect(onSave).toHaveBeenCalled());
    const s = onSave.mock.calls[0][0][0];
    expect(s).toMatchObject({
      name: "Kväll",
      enabled: true,
      deviceIds: ["0x03"],
      days: [0, 6],
      on: { kind: "sunset", offset: -30, notBefore: "16:00" },
      off: { kind: "time", time: "23:30" },
      brightness: 40,
    });
    expect(s.id).toBeTruthy();
    expect(s.on.notAfter).toBeUndefined();
  });

  it("needs a name, a light and something to do", () => {
    setup(view([]));
    fireEvent.click(screen.getByRole("button", { name: "Add schedule" }));
    const dialog = screen.getByRole("dialog");
    const add = within(dialog).getByRole("button", { name: "Add" }) as HTMLButtonElement;
    expect(add.disabled).toBe(true);
    fireEvent.change(within(dialog).getByLabelText("Name"), { target: { value: "X" } });
    fireEvent.click(within(dialog).getByRole("checkbox", { name: "Hall" }));
    expect(add.disabled).toBe(true);
    expect(within(dialog).getByText("Choose when to turn on and/or off.")).toBeTruthy();
    const on = within(dialog).getByRole("group", { name: "Turn on" });
    fireEvent.click(within(on).getByRole("checkbox", { name: "Turn on" }));
    fireEvent.change(within(on).getByLabelText("Time"), { target: { value: "07:00" } });
    expect(add.disabled).toBe(false);
  });

  it("edits and removes (two taps)", async () => {
    const { onSave } = setup();
    fireEvent.click(screen.getByRole("button", { name: "Edit Ute" }));
    const dialog = screen.getByRole("dialog", { name: "Edit schedule" });
    expect((within(dialog).getByLabelText("Name") as HTMLInputElement).value).toBe("Ute");
    fireEvent.click(within(dialog).getByRole("button", { name: "Remove schedule" }));
    expect(onSave).not.toHaveBeenCalled();
    fireEvent.click(within(dialog).getByRole("button", { name: "Tap again to remove" }));
    await vi.waitFor(() => expect(onSave).toHaveBeenCalledWith([]));
  });

  it("warns that sun times need a place", () => {
    setup(view([ute], false));
    expect(screen.getByText("Choose a place under Weather to use sunrise and sunset.")).toBeTruthy();
  });

  it("is in Swedish", () => {
    render(
      <I18nProvider language="sv">
        <ScheduleForm initial={view()} devices={devices} onSave={vi.fn()} now={new Date("2026-10-29T12:00:00+01:00")} />
      </I18nProvider>,
    );
    expect(screen.getByText(/Tänds 30 min före solnedgång, tidigast 16:00/)).toBeTruthy();
    expect(screen.getByText(/Vardagar/)).toBeTruthy();
    expect(screen.getByRole("button", { name: "Lägg till schema" })).toBeTruthy();
  });
});
