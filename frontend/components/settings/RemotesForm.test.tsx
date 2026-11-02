import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Device, RemoteAction, RemoteConfig, RemoteEvent, RemotesView } from "@/lib/api";
import { I18nProvider } from "@/lib/i18n";
import { RemotesForm, type LastPress } from "./RemotesForm";

afterEach(cleanup);

const dev = (id: string, name: string, kind: Device["kind"]): Device =>
  ({ id, name, kind, type: "Router", availability: "online", state: {}, capabilities: { onOff: true, brightness: kind === "light" } }) as unknown as Device;

const devices = [dev("0x01", "Taklampa", "light"), dev("0x02", "TV-uttag", "switch"), dev("0x10", "Fjärr soffan", "remote")];
const groups = [{ id: "g-vardagsrum", title: "Vardagsrum" }];
const a = (button: string, event: RemoteEvent, action: RemoteAction) => ({ button, event, action });
const defaults = [a("on", "press", "on"), a("on", "hold", "dim_up"), a("off", "press", "off"), a("off", "hold", "dim_down")];

const view = (configs: RemoteConfig[] = []): RemotesView => ({
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
      defaults,
    },
  ],
  configs,
});

function setup(v = view(), lastPress: LastPress | null = null, expand = true) {
  const onSave = vi.fn((c: RemoteConfig[]) => Promise.resolve({ ...v, configs: c }));
  render(<RemotesForm initial={v} devices={devices} groups={groups} lastPress={lastPress} onSave={onSave} />);
  if (expand) for (const r of v.remotes) fireEvent.click(screen.getByRole("button", { name: r.name }));
  return { onSave };
}

const select = (name: string) => screen.getByRole("combobox", { name }) as HTMLSelectElement;
const configured = (over: Partial<RemoteConfig> = {}): RemoteConfig => ({ remote: "0x10", target: "group:g-vardagsrum", actions: defaults, ...over });

describe("RemotesForm", () => {
  it("starts a new remote with on/off on short presses and dimming while held", () => {
    const { onSave } = setup();
    const box = screen.getByRole("group", { name: "Fjärr soffan" });
    expect(within(box).getByText(/IKEA E2001\/E2002/)).toBeTruthy();
    expect(select("On – short press").value).toBe("on");
    expect(select("On – hold").value).toBe("dim_up");
    expect(select("Off – short press").value).toBe("off");
    expect(select("Off – hold").value).toBe("dim_down");
    expect(select("Left arrow – short press").value).toBe("");
    // dimming is only offered while held
    expect(within(select("On – short press")).queryByRole("option", { name: "Dim up while held" })).toBeNull();
    expect(within(box).getByRole("button", { name: "Choose a group or light" })).toBeTruthy();
    expect(onSave).not.toHaveBeenCalled();
  });

  it("says how to add remotes when there are none", () => {
    setup({ remotes: [], configs: [] });
    expect(screen.getByText(/zigbee2mqtt/)).toBeTruthy();
  });

  it("controls one group or light for the whole remote", async () => {
    const { onSave } = setup();
    fireEvent.click(screen.getByRole("button", { name: "Choose a group or light" }));
    const dialog = screen.getByRole("dialog", { name: "What should Fjärr soffan control?" });
    expect(within(dialog).queryByRole("radio", { name: "Fjärr soffan" })).toBeNull(); // only lights and outlets
    expect(within(dialog).getByRole("radio", { name: "TV-uttag" })).toBeTruthy();
    fireEvent.click(within(dialog).getByRole("radio", { name: "Vardagsrum" }));
    await act(async () => {});
    expect(onSave).toHaveBeenLastCalledWith([configured()]);
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(screen.getByRole("button", { name: /Controls: Vardagsrum/ })).toBeTruthy();
  });

  it("remembers button changes made before choosing what to control", async () => {
    const { onSave } = setup();
    fireEvent.change(select("Left arrow – short press"), { target: { value: "toggle" } });
    expect(onSave).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Choose a group or light" }));
    fireEvent.click(screen.getByRole("radio", { name: "Taklampa" }));
    await act(async () => {});
    expect(onSave).toHaveBeenLastCalledWith([
      configured({ target: "device:0x01", actions: [...defaults, a("arrow_left", "press", "toggle")] }),
    ]);
  });

  it("changes and removes a button's action", async () => {
    const { onSave } = setup(view([configured()]));
    fireEvent.change(select("Off – short press"), { target: { value: "toggle" } });
    await act(async () => {});
    expect(onSave).toHaveBeenLastCalledWith([
      configured({ actions: [defaults[0], defaults[1], a("off", "press", "toggle"), defaults[3]] }),
    ]);
    fireEvent.change(select("On – hold"), { target: { value: "" } });
    await act(async () => {});
    expect(onSave).toHaveBeenLastCalledWith([
      configured({ actions: [defaults[0], a("off", "press", "toggle"), defaults[3]] }),
    ]);
  });

  it("sets brightness and colour temperature", async () => {
    const { onSave } = setup(view([configured({ actions: [] })]));
    fireEvent.change(select("Left arrow – short press"), { target: { value: "set" } });
    await act(async () => {});
    expect(onSave).toHaveBeenLastCalledWith([configured({ actions: [{ ...a("arrow_left", "press", "set"), brightness: 100 }] })]);
    const pct = screen.getByLabelText("Brightness (%)");
    fireEvent.change(pct, { target: { value: "30" } });
    fireEvent.blur(pct);
    await act(async () => {});
    fireEvent.change(screen.getByLabelText("Colour temperature"), { target: { value: "454" } });
    await act(async () => {});
    expect(onSave).toHaveBeenLastCalledWith([
      configured({ actions: [{ ...a("arrow_left", "press", "set"), brightness: 30, colorTemp: 454 }] }),
    ]);
  });

  it("can go back to the standard buttons", async () => {
    const { onSave } = setup(view([configured({ actions: [a("on", "press", "toggle")] })]));
    fireEvent.click(screen.getByRole("button", { name: "Standard buttons" }));
    await act(async () => {});
    expect(onSave).toHaveBeenLastCalledWith([configured()]);
  });

  it("can stop controlling anything", async () => {
    const other: RemoteConfig = { remote: "0x99", target: "device:0x01", actions: [] };
    const { onSave } = setup(view([other, configured()]));
    fireEvent.click(screen.getByRole("button", { name: /Controls: Vardagsrum/ }));
    fireEvent.click(screen.getByRole("radio", { name: "Nothing (remote off)" }));
    await act(async () => {});
    expect(onSave).toHaveBeenLastCalledWith([other]); // other remotes' settings are kept
  });

  it("is folded up until opened, showing what each remote controls", () => {
    setup(view([configured()]), null, false);
    const head = screen.getByRole("button", { name: "Fjärr soffan" });
    expect(head.getAttribute("aria-expanded")).toBe("false");
    expect(screen.queryByRole("combobox")).toBeNull();
    expect(screen.getByText("Controls: Vardagsrum")).toBeTruthy();
    fireEvent.click(head);
    expect(head.getAttribute("aria-expanded")).toBe("true");
    expect(select("On – short press")).toBeTruthy();
    fireEvent.click(head);
    expect(screen.queryByRole("combobox")).toBeNull();
  });

  it("says when a folded remote controls nothing yet", () => {
    setup(view(), null, false);
    expect(screen.getByText("Not set up")).toBeTruthy();
  });

  it("opens the remote whose button is pressed", () => {
    const v = view();
    const { rerender } = render(<RemotesForm initial={v} devices={devices} groups={groups} lastPress={null} onSave={vi.fn()} />);
    expect(screen.queryByRole("combobox")).toBeNull();
    rerender(
      <RemotesForm initial={v} devices={devices} groups={groups} lastPress={{ remote: "0x10", button: "off", event: "press", at: Date.now() }} onSave={vi.fn()} />,
    );
    expect(screen.getByRole("button", { name: "Fjärr soffan" }).getAttribute("aria-expanded")).toBe("true");
  });

  it("shows which button was just pressed", () => {
    setup(view(), { remote: "0x10", button: "arrow_left", event: "hold", at: Date.now() }, false);
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
    expect(screen.getByText("Inte inställd")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Fjärr soffan" }));
    expect(screen.getByRole("combobox", { name: "På – kort tryck" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Välj grupp eller lampa" })).toBeTruthy();
  });
});
