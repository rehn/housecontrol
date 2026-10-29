import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Dashboard, Layout, Tab } from "@/lib/dashboard";
import { I18nProvider } from "@/lib/i18n";
import { GrafanaPage } from "@/components/GrafanaPage";
import { GrafanaTile } from "@/components/tiles/GrafanaTile";
import { DashboardView } from "./DashboardView";
import { TabBar, TabDialog } from "./TabBar";

afterEach(() => {
  cleanup();
  document.documentElement.removeAttribute("data-theme");
});

const tabs: Tab[] = [
  { id: "home", kind: "cards", columns: [{ id: "c", cards: [] }] },
  { id: "el", title: "El", kind: "grafana", url: "https://g.lan/d/abc/el" },
];

describe("TabBar", () => {
  it("is hidden with a single tab (outside edit mode)", () => {
    const { container } = render(<TabBar tabs={[tabs[0]]} active="home" editing={false} onSelect={vi.fn()} onAdd={vi.fn()} onEdit={vi.fn()} />);
    expect(container.textContent).toBe("");
  });

  it("lists the tabs, marks the open one and switches on tap", () => {
    const onSelect = vi.fn();
    render(<TabBar tabs={tabs} active="home" editing={false} onSelect={onSelect} onAdd={vi.fn()} onEdit={vi.fn()} />);
    const list = screen.getByRole("tablist");
    expect(within(list).getAllByRole("tab").map((t) => t.textContent)).toEqual(["Home", "El"]);
    expect(screen.getByRole("tab", { name: "Home" }).getAttribute("aria-selected")).toBe("true");
    fireEvent.click(screen.getByRole("tab", { name: "El" }));
    expect(onSelect).toHaveBeenCalledWith("el");
    expect(screen.queryByRole("button", { name: "Add tab" })).toBeNull();
  });

  it("offers add and edit while editing", () => {
    const onAdd = vi.fn();
    const onEdit = vi.fn();
    render(<TabBar tabs={[tabs[0]]} active="home" editing onSelect={vi.fn()} onAdd={onAdd} onEdit={onEdit} />);
    fireEvent.click(screen.getByRole("button", { name: "Add tab" }));
    expect(onAdd).toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Edit tab Home" }));
    expect(onEdit).toHaveBeenCalledWith("home");
  });

  it("names unnamed tabs in Swedish", () => {
    render(
      <I18nProvider language="sv">
        <TabBar tabs={tabs} active="home" editing={false} onSelect={vi.fn()} onAdd={vi.fn()} onEdit={vi.fn()} />
      </I18nProvider>,
    );
    expect(screen.getByRole("tab", { name: "Hem" })).toBeTruthy();
  });
});

describe("TabDialog", () => {
  it("adds a Grafana tab from a pasted link", () => {
    const onSubmit = vi.fn();
    render(<TabDialog onSubmit={onSubmit} onClose={vi.fn()} />);
    const dialog = screen.getByRole("dialog", { name: "New tab" });
    fireEvent.change(within(dialog).getByLabelText("Name"), { target: { value: " El " } });
    fireEvent.click(within(dialog).getByRole("radio", { name: /Grafana dashboard/ }));
    const add = within(dialog).getByRole("button", { name: "Add" }) as HTMLButtonElement;
    expect(add.disabled).toBe(true); // needs a link
    fireEvent.change(within(dialog).getByLabelText("Grafana link"), { target: { value: "not a link" } });
    expect(add.disabled).toBe(true);
    expect(within(dialog).getByText("Enter a http or https link.")).toBeTruthy();
    fireEvent.change(within(dialog).getByLabelText("Grafana link"), { target: { value: "https://g.lan/d/abc/el?orgId=1" } });
    fireEvent.click(add);
    const tab = onSubmit.mock.calls[0][0];
    expect(tab).toMatchObject({ title: "El", kind: "grafana", url: "https://g.lan/d/abc/el?orgId=1" });
    expect(tab.id).toBeTruthy();
  });

  it("adds a page of cards", () => {
    const onSubmit = vi.fn();
    render(<TabDialog onSubmit={onSubmit} onClose={vi.fn()} />);
    fireEvent.change(screen.getByLabelText("Name"), { target: { value: "Ute" } });
    fireEvent.click(screen.getByRole("button", { name: "Add" }));
    const tab = onSubmit.mock.calls[0][0];
    expect(tab).toMatchObject({ title: "Ute", kind: "cards" });
    expect(tab.columns.length).toBeGreaterThan(0);
  });

  it("edits, moves and removes an existing tab", () => {
    const onSubmit = vi.fn();
    const onRemove = vi.fn();
    const onMove = vi.fn();
    render(<TabDialog initial={tabs[1]} canRemove onSubmit={onSubmit} onRemove={onRemove} onMove={onMove} onClose={vi.fn()} />);
    const dialog = screen.getByRole("dialog", { name: "Edit tab" });
    expect((within(dialog).getByLabelText("Grafana link") as HTMLInputElement).value).toBe("https://g.lan/d/abc/el");
    fireEvent.click(within(dialog).getByRole("button", { name: "Move left" }));
    expect(onMove).toHaveBeenCalledWith(-1);
    // removing takes two taps, so a page of cards isn't lost by mistake
    fireEvent.click(within(dialog).getByRole("button", { name: "Remove tab" }));
    expect(onRemove).not.toHaveBeenCalled();
    fireEvent.click(within(dialog).getByRole("button", { name: "Tap again to remove" }));
    expect(onRemove).toHaveBeenCalled();
    fireEvent.change(within(dialog).getByLabelText("Name"), { target: { value: "Elförbrukning" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Done" }));
    expect(onSubmit.mock.calls[0][0]).toMatchObject({ id: "el", title: "Elförbrukning", kind: "grafana" });
  });

  it("can't remove the only tab", () => {
    render(<TabDialog initial={tabs[0]} canRemove={false} onSubmit={vi.fn()} onRemove={vi.fn()} onMove={vi.fn()} onClose={vi.fn()} />);
    expect((screen.getByRole("button", { name: "Remove tab" }) as HTMLButtonElement).disabled).toBe(true);
  });
});

describe("Grafana", () => {
  it("shows a whole dashboard in kiosk mode, matching the app's dark look", () => {
    document.documentElement.setAttribute("data-theme", "dark");
    render(<GrafanaPage tab={tabs[1]} />);
    const frame = screen.getByTitle("El") as HTMLIFrameElement;
    const src = new URL(frame.src);
    expect(src.pathname).toBe("/d/abc/el");
    expect(src.searchParams.has("kiosk")).toBe(true);
    expect(src.searchParams.get("theme")).toBe("dark");
    expect(screen.getByRole("link", { name: /Open in Grafana/ }).getAttribute("href")).toBe("https://g.lan/d/abc/el");
  });

  it("shows a single panel as a card at the chosen height", () => {
    document.documentElement.setAttribute("data-theme", "light");
    render(<GrafanaTile card={{ id: "g", type: "grafana", title: "Temperatur", url: "https://g.lan/d-solo/abc/t?panelId=2", height: 240 }} />);
    expect(screen.getByText("Temperatur")).toBeTruthy();
    const frame = screen.getByTitle("Temperatur") as HTMLIFrameElement;
    expect(frame.style.height).toBe("240px");
    const src = new URL(frame.src);
    expect(src.searchParams.get("theme")).toBe("light");
    expect(src.searchParams.has("kiosk")).toBe(false);
  });
});

describe("Grafana panel card in the editor", () => {
  it("is added with a link (a 'view panel' link becomes an embed link) and height", async () => {
    const onSave = vi.fn<(l: Layout) => Promise<void>>(() => Promise.resolve());
    render(
      <DashboardView
        layout={{ columns: [{ id: "c", cards: [] }] }}
        devices={[]}
        editing
        onSave={onSave}
        onCancel={vi.fn()}
        onPatch={vi.fn()}
        onError={vi.fn()}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Add card" }));
    const dialog = screen.getByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: /Grafana panel/ }));
    fireEvent.change(within(dialog).getByLabelText("Panel link"), {
      target: { value: "https://g.lan/d/abc/t?orgId=1&viewPanel=2" },
    });
    fireEvent.change(within(dialog).getByLabelText(/Height/), { target: { value: "400" } });
    fireEvent.change(within(dialog).getByLabelText(/Name/), { target: { value: "Temperatur" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Add" }));
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await vi.waitFor(() => expect(onSave).toHaveBeenCalled());
    const card = onSave.mock.calls[0][0].columns[0].cards[0];
    expect(card).toMatchObject({
      type: "grafana",
      title: "Temperatur",
      url: "https://g.lan/d-solo/abc/t?orgId=1&panelId=2",
      height: 400,
    });
  });
});
