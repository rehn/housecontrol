import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Device, RemoteBinding, RemotesView } from "@/lib/api";
import { I18nProvider } from "@/lib/i18n";
import { RemotesForm, type LastPress } from "./RemotesForm";

afterEach(cleanup);

const dev = (id: string, name: string, kind: Device["kind"]): Device =>
  ({ id, name, kind, type: "Router", availability: "online", state: {}, capabilities: { onOff: true, brightness: kind === "light" } }) as unknown as Device;

const devices = [dev("0x01", "Taklampa", "light"), dev("0x02", "TV-uttag", "switch"), dev("0x10", "Fjärr soffan", "remote")];
const groups = [{ id: "g-vardagsrum", title: "Vardagsrum" }];

const view = (bindings: RemoteBinding[] = []): RemotesView => ({
  remotes: [
    {
      id: "0x10",
      name: "Fjärr soffan",
      vendor: "IKEA",
      model: "E2001/E2002",
      availability: "online",
      buttons: [
        { id: "on", events: ["press", "hold"] },
        { id: "off", events: ["press", "hold"] },
        { id: "arrow_left", events: ["press", "hold"] },
      ],
    },
  ],
  bindings,
});

function setup(v = view(), lastPress: LastPress | null = null) {
  const onSave = vi.fn((b: RemoteBinding[]) => Promise.resolve({ ...v, bindings: b }));
  const utils = render(<RemotesForm initial={v} devices={devices} groups={groups} lastPress={lastPress} onSave={onSave} />);
  return { onSave, ...utils };
}

const row = (name: string) => screen.getByRole("combobox", { name });

describe("RemotesForm", () => {
  it("lists the remotes with their buttons and kinds of press", () => {
    setup();
    const box = screen.getByRole("group", { name: "Fjärr soffan" });
    expect(within(box).getByText(/IKEA E2001\/E2002/)).toBeTruthy();
    expect(within(box).getByText("Left arrow")).toBeTruthy();
    expect(row("On – short press")).toBeTruthy();
    expect(row("On – hold")).toBeTruthy();
    // dimming is only offered while held
    expect(within(row("On – short press")).queryByRole("option", { name: "Dim up while held" })).toBeNull();
    expect(within(row("On – hold")).getByRole("option", { name: "Dim up while held" })).toBeTruthy();
  });

  it("says how to add remotes when there are none", () => {
    setup({ remotes: [], bindings: [] });
    expect(screen.getByText(/zigbee2mqtt/)).toBeTruthy();
  });

  it("asks what to control, then saves (a group of the home page)", async () => {
    const { onSave } = setup();
    fireEvent.change(row("On – short press"), { target: { value: "toggle" } });
    expect(onSave).not.toHaveBeenCalled(); // nothing to switch yet
    const dialog = screen.getByRole("dialog", { name: "What should it control?" });
    fireEvent.click(within(dialog).getByRole("checkbox", { name: "Vardagsrum" }));
    fireEvent.click(within(dialog).getByRole("checkbox", { name: "Taklampa" }));
    expect(within(dialog).queryByRole("checkbox", { name: "Fjärr soffan" })).toBeNull(); // only lights and outlets
    fireEvent.click(within(dialog).getByRole("button", { name: "Done" }));
    await act(async () => {});
    expect(onSave).toHaveBeenLastCalledWith([
      { remote: "0x10", button: "on", event: "press", action: "toggle", targets: ["group:g-vardagsrum", "device:0x01"] },
    ]);
    expect(screen.getByRole("button", { name: /Vardagsrum, Taklampa/ })).toBeTruthy();
  });

  it("changes and removes an action", async () => {
    const b: RemoteBinding = { remote: "0x10", button: "off", event: "press", action: "off", targets: ["device:0x02"] };
    const { onSave } = setup(view([b]));
    expect((row("Off – short press") as HTMLSelectElement).value).toBe("off");
    fireEvent.change(row("Off – short press"), { target: { value: "toggle" } });
    await act(async () => {});
    expect(onSave).toHaveBeenLastCalledWith([{ ...b, action: "toggle" }]);
    fireEvent.change(row("Off – short press"), { target: { value: "" } });
    await act(async () => {});
    expect(onSave).toHaveBeenLastCalledWith([]);
  });

  it("sets brightness and colour temperature", async () => {
    const b: RemoteBinding = { remote: "0x10", button: "arrow_left", event: "press", action: "on", targets: ["device:0x01"] };
    const { onSave } = setup(view([b]));
    fireEvent.change(row("Left arrow – short press"), { target: { value: "set" } });
    await act(async () => {});
    expect(onSave).toHaveBeenLastCalledWith([{ ...b, action: "set", brightness: 100 }]);
    const pct = screen.getByLabelText("Brightness (%)") as HTMLInputElement;
    fireEvent.change(pct, { target: { value: "30" } });
    fireEvent.blur(pct);
    await act(async () => {});
    expect(onSave).toHaveBeenLastCalledWith([{ ...b, action: "set", brightness: 30 }]);
    fireEvent.change(screen.getByLabelText("Colour temperature"), { target: { value: "454" } });
    await act(async () => {});
    expect(onSave).toHaveBeenLastCalledWith([{ ...b, action: "set", brightness: 30, colorTemp: 454 }]);
  });

  it("keeps the actions of remotes that aren't connected now", async () => {
    const gone: RemoteBinding = { remote: "0x99", button: "on", event: "press", action: "on", targets: ["device:0x01"] };
    const kept: RemoteBinding = { remote: "0x10", button: "on", event: "press", action: "on", targets: ["device:0x01"] };
    const { onSave } = setup(view([gone, kept]));
    fireEvent.change(row("On – short press"), { target: { value: "off" } });
    await act(async () => {});
    expect(onSave).toHaveBeenLastCalledWith([gone, { ...kept, action: "off" }]);
  });

  it("shows which button was just pressed", () => {
    setup(view(), { remote: "0x10", button: "arrow_left", event: "hold", at: Date.now() });
    const box = screen.getByRole("group", { name: "Fjärr soffan" });
    expect(within(box).getByText(/Last press: Left arrow – hold/)).toBeTruthy();
    expect(within(box).getByText("Left arrow").closest(".remote-button")!.className).toContain("pressed");
  });

  it("is in Swedish", () => {
    render(
      <I18nProvider language="sv">
        <RemotesForm initial={view()} devices={devices} groups={groups} lastPress={null} onSave={vi.fn()} />
      </I18nProvider>,
    );
    expect(screen.getByRole("combobox", { name: "På – kort tryck" })).toBeTruthy();
    expect(screen.getByRole("combobox", { name: "Vänsterpil – håll in" })).toBeTruthy();
  });
});
