// Dashboard model + pure helpers. Mirrors backend/internal/dashboard.
// A Dashboard is a list of tabs; a "cards" tab holds a Layout (columns of
// cards), a "grafana" tab shows a whole Grafana dashboard.
import type { Device } from "./api";

export type CardType = "clock" | "device" | "group" | "weather" | "calendar" | "grafana";

export interface Card {
  id: string;
  type: CardType;
  title?: string;
  deviceId?: string; // device card
  deviceIds?: string[]; // group card
  url?: string; // grafana card: single-panel embed link
  height?: number; // grafana card, px
}

export interface Column {
  id: string;
  cards: Card[];
}

/** One page of cards. */
export interface Layout {
  columns: Column[];
}

export type TabKind = "cards" | "grafana";

export interface Tab {
  id: string;
  title?: string; // empty = "Home" / "Grafana"
  kind: TabKind;
  columns?: Column[]; // cards
  url?: string; // grafana
}

export interface Dashboard {
  version: number;
  tabs: Tab[];
}

export const MAX_TABS = 12;
export const GRAFANA_HEIGHT = 300;

export const MIN_COLUMNS = 1;
export const MAX_COLUMNS = 6;

/** Random id; crypto.randomUUID is unavailable on plain-http LAN addresses. */
export function newId(prefix = "card"): string {
  return `${prefix}-${Math.random().toString(36).slice(2, 10)}${Date.now().toString(36).slice(-4)}`;
}

const controllable = (d: Device) => d.kind === "light" || d.kind === "switch";

/** Layout used until the user saves their own: clock + every light/outlet. */
export function defaultLayout(devices: Device[], columns = 3): Layout {
  let d: Layout = {
    columns: Array.from({ length: columns }, () => ({ id: newId("col"), cards: [] })),
  };
  d = addCard(d, { id: newId(), type: "clock" }, d.columns[0].id);
  const sorted = devices
    .filter(controllable)
    .sort((a, b) => (a.kind === b.kind ? a.name.localeCompare(b.name) : a.kind === "light" ? -1 : 1));
  for (const dev of sorted) d = addCard(d, { id: newId(), type: "device", deviceId: dev.id });
  return d;
}

function locate(d: Layout, cardId: string): [number, number] | null {
  for (let c = 0; c < d.columns.length; c++) {
    const i = d.columns[c].cards.findIndex((x) => x.id === cardId);
    if (i >= 0) return [c, i];
  }
  return null;
}

const cloneCols = (d: Layout) => d.columns.map((c) => ({ ...c, cards: [...c.cards] }));

/**
 * Move card `activeId` onto `overId`, which is either another card (the
 * moved card takes its place) or a column id (appended to that column).
 */
export function moveCard<T extends Layout>(d: T, activeId: string, overId: string): T {
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
export function addCard<T extends Layout>(d: T, card: Card, columnId?: string): T {
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

export function updateCard<T extends Layout>(d: T, card: Card): T {
  return {
    ...d,
    columns: d.columns.map((c) => ({ ...c, cards: c.cards.map((x) => (x.id === card.id ? card : x)) })),
  };
}

export function removeCard<T extends Layout>(d: T, cardId: string): T {
  return { ...d, columns: d.columns.map((c) => ({ ...c, cards: c.cards.filter((x) => x.id !== cardId) })) };
}

/** Device ids that already have their own device card. */
export function usedDeviceIds(d: Layout): Set<string> {
  const s = new Set<string>();
  for (const c of d.columns) for (const x of c.cards) if (x.type === "device" && x.deviceId) s.add(x.deviceId);
  return s;
}

/** Change the number of columns; cards of removed columns move to the last one kept. */
export function setColumnCount<T extends Layout>(d: T, n: number): T {
  n = Math.max(MIN_COLUMNS, Math.min(MAX_COLUMNS, Math.round(n)));
  const columns = cloneCols(d);
  while (columns.length < n) columns.push({ id: newId("col"), cards: [] });
  if (columns.length > n) {
    const removed = columns.splice(n);
    for (const col of removed) columns[n - 1].cards.push(...col.cards);
  }
  return { ...d, columns };
}

// --- tabs --------------------------------------------------------------------

/** Until something is saved: one Home tab with the default cards. */
export function defaultTabs(devices: Device[]): Dashboard {
  return { version: 2, tabs: [{ id: "home", kind: "cards", columns: defaultLayout(devices).columns }] };
}

/** A layout saved before tabs existed ({columns}) becomes one Home tab. */
export function upgradeDashboard(d: Dashboard): Dashboard {
  const old = d as Dashboard & { columns?: Column[] };
  if ((!old.tabs || old.tabs.length === 0) && old.columns) {
    return { version: 2, tabs: [{ id: "home", kind: "cards", columns: old.columns }] };
  }
  return d;
}

/** The cards of a cards tab (at least one column). */
export function tabLayout(tab: Tab): Layout {
  return { columns: tab.columns && tab.columns.length > 0 ? tab.columns : [{ id: newId("col"), cards: [] }] };
}

export function setTabLayout(d: Dashboard, tabId: string, layout: Layout): Dashboard {
  return { ...d, tabs: d.tabs.map((t) => (t.id === tabId ? { ...t, columns: layout.columns } : t)) };
}

export function addTab(d: Dashboard, tab: Tab): Dashboard {
  return { ...d, tabs: [...d.tabs, tab] };
}

export function updateTab(d: Dashboard, tab: Tab): Dashboard {
  return { ...d, tabs: d.tabs.map((t) => (t.id === tab.id ? tab : t)) };
}

/** The last tab can't be removed. */
export function removeTab(d: Dashboard, tabId: string): Dashboard {
  if (d.tabs.length <= 1) return d;
  return { ...d, tabs: d.tabs.filter((t) => t.id !== tabId) };
}

/** Move a tab one step left (-1) or right (+1). */
export function moveTab(d: Dashboard, tabId: string, step: -1 | 1): Dashboard {
  const i = d.tabs.findIndex((t) => t.id === tabId);
  const j = i + step;
  if (i < 0 || j < 0 || j >= d.tabs.length) return d;
  const tabs = [...d.tabs];
  [tabs[i], tabs[j]] = [tabs[j], tabs[i]];
  return { ...d, tabs };
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
