import type { Dashboard } from "./dashboard";
import type { Schedule, SchedulesView } from "./schedules";

export type Kind = "light" | "switch" | "remote" | "other";

export interface Capabilities {
  onOff: boolean;
  stateProperty?: string;
  brightness: boolean;
  brightnessMax?: number;
  colorTemp: boolean;
  colorTempMin?: number;
  colorTempMax?: number;
  color: boolean;
  power: boolean;
  /** a remote's button actions as zigbee2mqtt reports them */
  actions?: string[];
}

export interface Device {
  id: string;
  name: string;
  kind: Kind;
  type: string;
  vendor?: string;
  model?: string;
  description?: string;
  capabilities: Capabilities;
  availability: "online" | "offline" | "unknown" | string;
  state: Record<string, unknown>;
}

export type MqttState = "unconfigured" | "connecting" | "connected" | "error";

export interface MqttStatus {
  state: MqttState;
  broker?: string;
  error?: string;
  since: string;
}

export interface MqttSettings {
  url: string;
  username: string;
  hasPassword: boolean;
  clientId: string;
  baseTopic: string;
  source: "ui" | "config";
  status: MqttStatus;
}

export interface MqttSettingsInput {
  url: string;
  username: string;
  /** omit = keep current password, "" = no password */
  password?: string;
  clientId: string;
  baseTopic: string;
}

export type ServerEvent =
  | { type: "devices"; devices?: Device[] }
  | { type: "state"; id: string; state: Record<string, unknown> }
  | { type: "availability"; id: string; availability: string }
  | { type: "status"; mqtt: MqttStatus }
  | { type: "dashboard"; dashboard?: Dashboard | null }
  | { type: "ui"; ui?: UISettings }
  | { type: "weather" } // weather settings changed: weather cards reload
  | { type: "calendar" } // calendar settings changed: calendar cards reload
  | { type: "remote"; id: string; button: string; press: RemoteEvent }; // a remote's button was pressed

export interface CalendarEvent {
  title: string;
  location?: string;
  /** ISO time in the server's (home's) time zone */
  start: string;
  end: string;
  allDay: boolean;
  calendarId: string;
  color: string;
}

export interface CalendarResult {
  from: string; // first day shown, YYYY-MM-DD
  to: string; // last day shown (inclusive)
  today: string; // the home's today
  days: { date: string; events: CalendarEvent[] }[];
  errors?: { calendarId: string; error: string }[];
}

/** A saved calendar as the server shows it (the secret link is never sent). */
export interface CalendarSetting {
  id: string;
  name: string;
  color: string;
  urlHint: string;
}

/** What is saved: existing calendars by id (url omitted = keep), new ones with url. */
export interface CalendarInput {
  id?: string;
  name: string;
  color: string;
  url?: string;
}

export type WeatherCondition =
  | "clear"
  | "mostly_clear"
  | "partly_cloudy"
  | "cloudy"
  | "fog"
  | "light_rain"
  | "rain"
  | "heavy_rain"
  | "showers"
  | "thunder"
  | "sleet"
  | "light_snow"
  | "snow"
  | "snow_showers"
  | "unknown";

export interface WeatherForecast {
  provider: string;
  location?: string;
  /** area looked up when the place was saved, e.g. "Enköping, Uppsala län" */
  area?: string;
  updated: string;
  stale?: boolean;
  current: { temperature: number; condition: WeatherCondition };
  daily: { date: string; min: number; max: number; condition: WeatherCondition }[];
  /** today's sunrise/sunset (RFC 3339, home time zone), or polar day/night */
  sun?: { sunrise?: string; sunset?: string; polar?: "day" | "night" };
}

export interface WeatherSettings {
  provider: string;
  latitude: number;
  longitude: number;
  location: string;
  area?: string;
  configured: boolean;
  providers: string[];
}

export interface Place {
  name: string;
  region?: string;
  country?: string;
  latitude: number;
  longitude: number;
}

export interface UISettings {
  language: string;
  /** "" = default title */
  title?: string;
}

/** Error from the backend; `code` (if any) can be translated via errors.<code>. */
export class ApiError extends Error {
  constructor(
    message: string,
    public code?: string,
    public status?: number,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

// Empty = same origin (the Go backend serves the built frontend). In
// `next dev` the backend is assumed on localhost:8080 unless
// NEXT_PUBLIC_API_BASE is set.
export const API_BASE =
  process.env.NEXT_PUBLIC_API_BASE ??
  (process.env.NODE_ENV === "development" ? "http://localhost:8080" : "");

export function wsUrl(): string {
  const base = API_BASE || window.location.origin;
  return base.replace(/^http/, "ws") + "/api/ws";
}

/** Sent when the server says the login is missing or has run out (see lib/auth.tsx). */
export const UNAUTHORIZED_EVENT = "housecontrol:unauthorized";

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  let res: Response;
  try {
    res = await fetch(`${API_BASE}${path}`, {
      method,
      credentials: "include", // the login cookie, also when next dev (:3000) talks to :8080
      headers: body === undefined ? undefined : { "Content-Type": "application/json" },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
  } catch {
    throw new ApiError("cannot reach the housecontrol backend", "backend_unreachable");
  }
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    if (res.status === 401 && data.code === "unauthorized" && typeof window !== "undefined") {
      window.dispatchEvent(new Event(UNAUTHORIZED_EVENT));
    }
    throw new ApiError(data.error ?? `request failed (${res.status})`, data.code, res.status);
  }
  return data as T;
}

export const getSchedules = () => request<SchedulesView>("GET", "/api/settings/schedules");
export const saveSchedules = (schedules: Schedule[]) =>
  request<SchedulesView>("PUT", "/api/settings/schedules", { schedules });

/**
 * All settings as a file (Settings → Backup): compressed and encrypted with
 * `password`, or readable JSON when `password` is null.
 */
export async function downloadBackup(password: string | null): Promise<{ blob: Blob; filename: string }> {
  return fetchFile(
    "POST",
    "/api/settings/backup",
    password === null ? { protect: false } : { protect: true, password },
    "housecontrol-backup.hcbackup",
  );
}

/** A file download from the API (with the name from Content-Disposition). */
async function fetchFile(
  method: string,
  path: string,
  body: unknown,
  fallbackName: string,
): Promise<{ blob: Blob; filename: string }> {
  let res: Response;
  try {
    res = await fetch(`${API_BASE}${path}`, {
      method,
      credentials: "include",
      ...(body === undefined
        ? {}
        : { headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) }),
    });
  } catch {
    throw new ApiError("cannot reach the housecontrol backend", "backend_unreachable");
  }
  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    if (res.status === 401 && typeof window !== "undefined") window.dispatchEvent(new Event(UNAUTHORIZED_EVENT));
    throw new ApiError(data.error ?? `request failed (${res.status})`, data.code, res.status);
  }
  const name = /filename="([^"]+)"/.exec(res.headers.get("Content-Disposition") ?? "")?.[1] ?? fallbackName;
  return { blob: await res.blob(), filename: name };
}

/** Replace all settings with a backup file. */
export async function restoreBackup(file: Blob, password: string): Promise<{ exported?: string }> {
  const bytes = new Uint8Array(await file.arrayBuffer());
  let bin = "";
  for (let i = 0; i < bytes.length; i += 0x8000) bin += String.fromCharCode(...bytes.subarray(i, i + 0x8000));
  return request("POST", "/api/settings/restore", { password, data: btoa(bin) });
}

/** When backups are made automatically, and how many are kept. */
export interface AutoBackupSettings {
  enabled: boolean;
  /** HH:MM, every day */
  time: string;
  keep: number;
}

export interface StoredBackup {
  name: string;
  size: number;
  created: string;
}

export interface AutoBackupView extends AutoBackupSettings {
  /** false: no backup folder on the server (BACKUP_DIR) */
  available: boolean;
  dir?: string;
  encrypted: boolean;
  next?: string;
  lastRun?: string;
  lastError?: string;
  /** newest first */
  backups: StoredBackup[];
}

export const getAutoBackup = () => request<AutoBackupView>("GET", "/api/settings/autobackup");
export const saveAutoBackup = (s: AutoBackupSettings) => request<AutoBackupView>("PUT", "/api/settings/autobackup", s);
export const runAutoBackup = () => request<AutoBackupView>("POST", "/api/settings/autobackup/run");
export const downloadStoredBackup = (name: string) =>
  fetchFile("GET", `/api/settings/autobackup/files/${encodeURIComponent(name)}`, undefined, name);
export const restoreStoredBackup = (name: string) =>
  request<{ exported?: string }>("POST", `/api/settings/autobackup/files/${encodeURIComponent(name)}/restore`, {});

/** Kinds of press an action can be chosen for. */
export type RemoteEvent = "press" | "double" | "hold";
export type RemoteAction = "on" | "off" | "toggle" | "brighter" | "darker" | "dim_up" | "dim_down" | "set";

/** What one kind of press on one button does. */
export interface RemoteButtonAction {
  button: string;
  event: RemoteEvent;
  action: RemoteAction;
  /** set: % */
  brightness?: number;
  /** set: mired */
  colorTemp?: number;
}

/** One remote: what it controls, and what its buttons do. */
export interface RemoteConfig {
  remote: string;
  /** "device:<id>" or "group:<group card id>" */
  target: string;
  actions: RemoteButtonAction[];
}

export interface RemoteInfo {
  id: string;
  name: string;
  vendor?: string;
  model?: string;
  description?: string;
  availability: string;
  buttons: { id: string; events: RemoteEvent[] }[];
  /** what the buttons do until changed */
  defaults: RemoteButtonAction[];
}

export interface RemotesView {
  remotes: RemoteInfo[];
  configs: RemoteConfig[];
}

export const getRemotes = () => request<RemotesView>("GET", "/api/settings/remotes");
export const saveRemotes = (configs: RemoteConfig[]) => request<RemotesView>("PUT", "/api/settings/remotes", { configs });

/** Login state as the server sees it. `required` is false when no password is set. */
export interface AuthStatus {
  required: boolean;
  loggedIn: boolean;
  username?: string;
}

export const getAuth = () => request<AuthStatus>("GET", "/api/auth");
export const login = (username: string, password: string) =>
  request<AuthStatus>("POST", "/api/auth/login", { username, password });
export const logout = () => request<AuthStatus>("POST", "/api/auth/logout");

export async function setDevice(id: string, payload: Record<string, unknown>): Promise<void> {
  await request("POST", `/api/devices/${encodeURIComponent(id)}/set`, payload);
}

export const getMqttSettings = () => request<MqttSettings>("GET", "/api/settings/mqtt");

export const saveMqttSettings = (s: MqttSettingsInput) =>
  request<MqttSettings>("PUT", "/api/settings/mqtt", s);

export const testMqttSettings = (s: MqttSettingsInput) =>
  request<{ ok: boolean; error?: string; url: string }>("POST", "/api/settings/mqtt/test", s);

/** Same command to several devices at once (group cards). */
export async function setMany(ids: string[], payload: Record<string, unknown>): Promise<void> {
  await request("POST", "/api/devices/set-many", { ids, payload });
}

export const saveDashboard = (d: Dashboard) =>
  request<{ dashboard: Dashboard }>("PUT", "/api/dashboard", d);

export const getUISettings = () => request<UISettings>("GET", "/api/settings/ui");

/** Only the given fields are changed, e.g. { title } keeps the language. */
export const saveUISettings = (u: Partial<UISettings>) => request<UISettings>("PUT", "/api/settings/ui", u);

export const getWeather = () => request<WeatherForecast>("GET", "/api/weather");

export const getWeatherSettings = () => request<WeatherSettings>("GET", "/api/settings/weather");

export const saveWeatherSettings = (w: Pick<WeatherSettings, "provider" | "latitude" | "longitude" | "location">) =>
  request<WeatherSettings>("PUT", "/api/settings/weather", w);

export const searchPlaces = (q: string, lang: string) =>
  request<Place[]>("GET", `/api/weather/places?q=${encodeURIComponent(q)}&lang=${encodeURIComponent(lang)}`);

/** Name of the area at a position, e.g. "Enköping, Uppsala län". */
export const lookupArea = (lat: number, lon: number, lang: string) =>
  request<{ area: string }>("GET", `/api/weather/area?lat=${lat}&lon=${lon}&lang=${encodeURIComponent(lang)}`).then(
    (r) => r.area,
  );

/** The home's rough position from its internet connection (via the backend). */
export const locateByIP = () => request<Place>("GET", "/api/weather/locate");

/** A week of events starting at `from` (YYYY-MM-DD), or at today when left out. */
export const getCalendar = (from?: string) =>
  request<CalendarResult>("GET", from ? `/api/calendar?from=${encodeURIComponent(from)}` : "/api/calendar");

export const getCalendarSettings = () =>
  request<{ calendars: CalendarSetting[] }>("GET", "/api/settings/calendars").then((r) => r.calendars);

export const saveCalendarSettings = (calendars: CalendarInput[]) =>
  request<{ calendars: CalendarSetting[] }>("PUT", "/api/settings/calendars", { calendars }).then((r) => r.calendars);

export const testCalendarLink = (url: string) =>
  request<{ ok: boolean; events?: number; error?: string }>("POST", "/api/settings/calendars/test", { url });
