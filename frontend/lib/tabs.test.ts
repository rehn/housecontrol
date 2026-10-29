import { describe, expect, it } from "vitest";
import type { Device } from "./api";
import {
  addTab,
  defaultTabs,
  moveTab,
  removeTab,
  setTabLayout,
  tabLayout,
  updateTab,
  upgradeDashboard,
  type Dashboard,
} from "./dashboard";

const lamp: Device = {
  id: "0x01",
  name: "Kök",
  kind: "light",
  type: "Router",
  capabilities: { onOff: true, brightness: true, colorTemp: false, color: false, power: false },
  availability: "online",
  state: {},
};

const two = (): Dashboard => ({
  version: 2,
  tabs: [
    { id: "home", kind: "cards", columns: [{ id: "c", cards: [{ id: "a", type: "clock" }] }] },
    { id: "el", title: "El", kind: "grafana", url: "https://g.lan/d/abc/el" },
  ],
});

describe("tabs", () => {
  it("starts with one Home tab holding the default cards", () => {
    const d = defaultTabs([lamp]);
    expect(d.version).toBe(2);
    expect(d.tabs).toHaveLength(1);
    expect(d.tabs[0]).toMatchObject({ id: "home", kind: "cards" });
    expect(tabLayout(d.tabs[0]).columns.flatMap((c) => c.cards.map((x) => x.type))).toEqual(["clock", "device"]);
  });

  it("replaces the cards of one tab", () => {
    const d = setTabLayout(two(), "home", { columns: [{ id: "x", cards: [] }, { id: "y", cards: [] }] });
    expect(d.tabs[0].columns).toHaveLength(2);
    expect(d.tabs[1]).toEqual(two().tabs[1]);
  });

  it("adds, updates and removes tabs without touching the input", () => {
    const start = two();
    let d = addTab(start, { id: "ute", title: "Ute", kind: "cards", columns: [{ id: "u", cards: [] }] });
    expect(d.tabs.map((t) => t.id)).toEqual(["home", "el", "ute"]);
    d = updateTab(d, { ...d.tabs[1], title: "Elförbrukning" });
    expect(d.tabs[1].title).toBe("Elförbrukning");
    d = removeTab(d, "el");
    expect(d.tabs.map((t) => t.id)).toEqual(["home", "ute"]);
    expect(start.tabs).toHaveLength(2);
  });

  it("never removes the last tab", () => {
    const one: Dashboard = { version: 2, tabs: [two().tabs[0]] };
    expect(removeTab(one, "home").tabs).toHaveLength(1);
  });

  it("moves a tab left or right, stopping at the ends", () => {
    expect(moveTab(two(), "el", -1).tabs.map((t) => t.id)).toEqual(["el", "home"]);
    expect(moveTab(two(), "el", 1).tabs.map((t) => t.id)).toEqual(["home", "el"]);
    expect(moveTab(two(), "home", -1).tabs.map((t) => t.id)).toEqual(["home", "el"]);
  });

  it("gives a cards tab without columns one empty column", () => {
    expect(tabLayout({ id: "x", kind: "cards" }).columns).toHaveLength(1);
  });
});

describe("upgradeDashboard", () => {
  it("turns an old single-page layout into one Home tab", () => {
    const old = { version: 1, columns: [{ id: "c", cards: [{ id: "a", type: "clock" as const }] }] };
    const d = upgradeDashboard(old as unknown as Dashboard);
    expect(d.tabs).toEqual([{ id: "home", kind: "cards", columns: old.columns }]);
  });

  it("leaves a tabbed dashboard alone", () => {
    expect(upgradeDashboard(two())).toEqual(two());
  });
});
