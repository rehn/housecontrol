import { describe, expect, it } from "vitest";
import type { Device } from "./api";
import {
  addCard,
  defaultLayout,
  groupSummary,
  moveCard,
  removeCard,
  setColumnCount,
  updateCard,
  usedDeviceIds,
  type Layout,
} from "./dashboard";

function dev(id: string, name: string, kind: Device["kind"], state: Record<string, unknown> = {}): Device {
  return {
    id,
    name,
    kind,
    type: "Router",
    capabilities: {
      onOff: kind !== "other",
      brightness: kind === "light",
      brightnessMax: 254,
      colorTemp: false,
      color: false,
      power: false,
    },
    availability: "online",
    state,
  };
}

const devices = [
  dev("0x01", "Kitchen 1", "light", { state: "ON", brightness: 200 }),
  dev("0x02", "Kitchen 2", "light", { state: "OFF" }),
  dev("0x03", "TV plug", "switch", { state: "ON" }),
  dev("0x04", "Motion sensor", "other"),
];

const layout = (): Layout => ({
  columns: [
    { id: "c1", cards: [{ id: "a", type: "clock" }, { id: "b", type: "device", deviceId: "0x01" }] },
    { id: "c2", cards: [{ id: "c", type: "device", deviceId: "0x02" }] },
  ],
});

const ids = (d: Layout) => d.columns.map((c) => c.cards.map((x) => x.id));

describe("defaultLayout", () => {
  it("starts with a clock and one card per light/outlet, spread over 3 columns", () => {
    const d = defaultLayout(devices);
    expect(d.columns).toHaveLength(3);
    const cards = d.columns.flatMap((c) => c.cards);
    expect(cards[0]).toMatchObject({ type: "clock" });
    expect(cards.filter((c) => c.type === "device").map((c) => c.deviceId).sort()).toEqual([
      "0x01",
      "0x02",
      "0x03",
    ]); // the sensor ("other") is left out
    expect(d.columns.every((c) => c.cards.length > 0)).toBe(true);
  });

  it("works with no devices", () => {
    const d = defaultLayout([]);
    expect(d.columns.flatMap((c) => c.cards)).toHaveLength(1);
  });

  it("gives every card a unique id", () => {
    const all = defaultLayout(devices).columns.flatMap((c) => c.cards.map((x) => x.id));
    expect(new Set(all).size).toBe(all.length);
  });
});

describe("moveCard", () => {
  it("reorders within a column", () => {
    expect(ids(moveCard(layout(), "b", "a"))).toEqual([["b", "a"], ["c"]]);
  });

  it("moves onto a card in another column (takes its place)", () => {
    expect(ids(moveCard(layout(), "a", "c"))).toEqual([["b"], ["a", "c"]]);
  });

  it("moves to the end of a column when dropped on the column itself", () => {
    expect(ids(moveCard(layout(), "a", "c2"))).toEqual([["b"], ["c", "a"]]);
  });

  it("is a no-op for unknown ids and does not mutate the input", () => {
    const d = layout();
    const before = JSON.stringify(d);
    expect(moveCard(d, "zzz", "c")).toBe(d);
    moveCard(d, "a", "c");
    expect(JSON.stringify(d)).toBe(before);
  });
});

describe("add / update / remove", () => {
  it("adds to the shortest column by default", () => {
    const d = addCard(layout(), { id: "n", type: "clock" });
    expect(ids(d)).toEqual([["a", "b"], ["c", "n"]]);
  });

  it("adds to a given column", () => {
    expect(ids(addCard(layout(), { id: "n", type: "clock" }, "c1"))).toEqual([["a", "b", "n"], ["c"]]);
  });

  it("updates a card in place", () => {
    const d = updateCard(layout(), { id: "b", type: "device", deviceId: "0x01", title: "Lamp" });
    expect(d.columns[0].cards[1].title).toBe("Lamp");
  });

  it("removes a card", () => {
    expect(ids(removeCard(layout(), "b"))).toEqual([["a"], ["c"]]);
  });

  it("lists devices that already have a card", () => {
    const d = addCard(layout(), { id: "g", type: "group", title: "G", deviceIds: ["0x03"] });
    expect([...usedDeviceIds(d)].sort()).toEqual(["0x01", "0x02"]); // groups don't count
  });
});

describe("setColumnCount", () => {
  it("adds empty columns", () => {
    const d = setColumnCount(layout(), 4);
    expect(d.columns).toHaveLength(4);
    expect(d.columns[3].cards).toEqual([]);
    expect(new Set(d.columns.map((c) => c.id)).size).toBe(4);
  });

  it("keeps cards from removed columns by moving them to the last remaining column", () => {
    expect(ids(setColumnCount(layout(), 1))).toEqual([["a", "b", "c"]]);
  });

  it("clamps to 1..6", () => {
    expect(setColumnCount(layout(), 0).columns).toHaveLength(1);
    expect(setColumnCount(layout(), 9).columns).toHaveLength(6);
  });
});

describe("groupSummary", () => {
  const byId = Object.fromEntries(devices.map((d) => [d.id, d]));

  it("counts members that are on", () => {
    const s = groupSummary(byId, ["0x01", "0x02", "0x03"]);
    expect(s).toMatchObject({ total: 3, on: 2, anyOn: true, known: 3 });
  });

  it("toggling a group with any light on turns everything off, otherwise on", () => {
    expect(groupSummary(byId, ["0x01", "0x02"]).nextState).toBe("OFF");
    expect(groupSummary(byId, ["0x02"]).nextState).toBe("ON");
  });

  it("ignores devices that no longer exist", () => {
    const s = groupSummary(byId, ["0x02", "0x99"]);
    expect(s).toMatchObject({ total: 2, known: 1, on: 0, knownIds: ["0x02"] });
  });

  it("offers brightness only if every known member supports it", () => {
    expect(groupSummary(byId, ["0x01", "0x02"]).brightness).toBe(true);
    expect(groupSummary(byId, ["0x01", "0x03"]).brightness).toBe(false);
  });

  it("averages the brightness of lights that are on", () => {
    expect(groupSummary(byId, ["0x01", "0x02"]).avgBrightness).toBe(200);
  });
});
