// Dashboard layout model + pure helpers. Mirrors backend/internal/dashboard.
import type { Device } from "./api";

export type CardType = "clock" | "device" | "group" | "weather" | "calendar";

export interface Card {
  id: string;
  type: CardType;
  title?: string;
  deviceId?: string; // device card
  deviceIds?: string[]; // group card
}

export interface Column {
  id: string;
  cards: Card[];
}

export interface Dashboard {
  version: number;
  columns: Column[];
}

export const MIN_COLUMNS = 1;
export const MAX_COLUMNS = 6;

/** Random id; crypto.randomUUID is unavailable on plain-http LAN addresses. */
export function newId(prefix = "card"): string {
  return `${prefix}-${Math.random().toString(36).slice(2, 10)}${Date.now().toString(36).slice(-4)}`;
}

const controllable = (d: Device) => d.kind === "light" || d.kind === "switch";

/** Layout used until the user saves their own: clock + every light/outlet. */
export function defaultDashboard(devices: Device[], columns = 3): Dashboard {
  let d: Dashboard = {
    version: 1,
    columns: Array.from({ length: columns }, () => ({ id: newId("col"), cards: [] })),
  };
  d = addCard(d, { id: newId(), type: "clock" }, d.columns[0].id);
  const sorted = devices
    .filter(controllable)
    .sort((a, b) => (a.kind === b.kind ? a.name.localeCompare(b.name) : a.kind === "light" ? -1 : 1));
  for (const dev of sorted) d = addCard(d, { id: newId(), type: "device", deviceId: dev.id });
  return d;
}

function locate(d: Dashboard, cardId: string): [number, number] | null {
  for (let c = 0; c < d.columns.length; c++) {
    const i = d.columns[c].cards.findIndex((x) => x.id === cardId);
    if (i >= 0) return [c, i];
  }
  return null;
}

const cloneCols = (d: Dashboard) => d.columns.map((c) => ({ ...c, cards: [...c.cards] }));

/**
 * Move card `activeId` onto `overId`, which is either another card (the
 * moved card takes its place) or a column id (appended to that column).
 */
export function moveCard(d: Dashboard, activeId: string, overId: string): Dashboard {
  const from = locate(d, activeId);
  if (!from || activeId === overId) return d;
  const columns = cloneCols(d);
  const [card] = columns[from[0]].cards.splice(from[1], 1);

  const overCol = columns.findIndex((c) => c.id === overId);
  if (overCol >= 0) {
    columns[overCol].cards.push(card);
    return { ...d, columns };
  }
  const to = locate(d, overId);
  if (!to) return d;
  // Insert at the target's original index: moving up places the card before
  // the target, moving down within a column places it after (the removal
  // shifted the target one step up).
  columns[to[0]].cards.splice(to[1], 0, card);
  return { ...d, columns };
}

/** Add a card to `columnId`, or to the column with the fewest cards. */
export function addCard(d: Dashboard, card: Card, columnId?: string): Dashboard {
  const columns = cloneCols(d);
  let target = columns.findIndex((c) => c.id === columnId);
  if (target < 0) {
    target = 0;
    columns.forEach((c, i) => {
      if (c.cards.length < columns[target].cards.length) target = i;
    });
  }
  columns[target].cards.push(card);
  return { ...d, columns };
}

export function updateCard(d: Dashboard, card: Card): Dashboard {
  return {
    ...d,
    columns: d.columns.map((c) => ({ ...c, cards: c.cards.map((x) => (x.id === card.id ? card : x)) })),
  };
}

export function removeCard(d: Dashboard, cardId: string): Dashboard {
  return { ...d, columns: d.columns.map((c) => ({ ...c, cards: c.cards.filter((x) => x.id !== cardId) })) };
}

/** Device ids that already have their own device card. */
export function usedDeviceIds(d: Dashboard): Set<string> {
  const s = new Set<string>();
  for (const c of d.columns) for (const x of c.cards) if (x.type === "device" && x.deviceId) s.add(x.deviceId);
  return s;
}

/** Change the number of columns; cards of removed columns move to the last one kept. */
export function setColumnCount(d: Dashboard, n: number): Dashboard {
  n = Math.max(MIN_COLUMNS, Math.min(MAX_COLUMNS, Math.round(n)));
  const columns = cloneCols(d);
  while (columns.length < n) columns.push({ id: newId("col"), cards: [] });
  if (columns.length > n) {
    const removed = columns.splice(n);
    for (const col of removed) columns[n - 1].cards.push(...col.cards);
  }
  return { ...d, columns };
}

export interface GroupSummary {
  total: number; // configured members
  known: number; // members that still exist
  knownIds: string[];
  on: number;
  anyOn: boolean;
  /** what a tap on the group toggle should send */
  nextState: "ON" | "OFF";
  /** every known member supports brightness */
  brightness: boolean;
  avgBrightness: number;
}

export function groupSummary(byId: Record<string, Device>, ids: string[]): GroupSummary {
  const members = ids.map((id) => byId[id]).filter((d): d is Device => !!d);
  const isOn = (d: Device) => d.state[d.capabilities.stateProperty ?? "state"] === "ON";
  const onMembers = members.filter(isOn);
  const brightnesses = onMembers
    .map((d) => d.state.brightness)
    .filter((b): b is number => typeof b === "number");
  return {
    total: ids.length,
    known: members.length,
    knownIds: members.map((d) => d.id),
    on: onMembers.length,
    anyOn: onMembers.length > 0,
    nextState: onMembers.length > 0 ? "OFF" : "ON",
    brightness: members.length > 0 && members.every((d) => d.capabilities.brightness),
    avgBrightness: brightnesses.length
      ? Math.round(brightnesses.reduce((a, b) => a + b, 0) / brightnesses.length)
      : 0,
  };
}
