"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { Device, MqttStatus, ServerEvent, wsUrl } from "./api";
import type { Dashboard } from "./dashboard";

export type ConnState = "connecting" | "open" | "closed";

const LANGUAGE_KEY = "housecontrol.language";

function cachedLanguage(): string {
  try {
    return localStorage.getItem(LANGUAGE_KEY) ?? "en";
  } catch {
    return "en";
  }
}

// Keeps a live device list and MQTT status from the backend WebSocket,
// reconnecting with backoff when the connection drops.
export function useDevices() {
  const [devices, setDevices] = useState<Record<string, Device>>({});
  const [mqtt, setMqtt] = useState<MqttStatus | null>(null);
  // undefined = not received yet, null = none saved on the server
  const [dashboard, setDashboard] = useState<Dashboard | null | undefined>(undefined);
  const [conn, setConn] = useState<ConnState>("connecting");
  // Start in English on both the pre-rendered HTML and the first client render
  // (avoids a hydration mismatch), then switch to the cached language at once.
  const [language, setLanguageState] = useState<string>("en");
  useEffect(() => setLanguageState(cachedLanguage()), []);
  const [title, setTitle] = useState<string>(""); // "" = default title
  const [uiLoaded, setUiLoaded] = useState(false); // the server has sent language/title
  const [weatherVersion, setWeatherVersion] = useState(0); // bumps when weather settings change
  const [calendarVersion, setCalendarVersion] = useState(0); // bumps when calendars change
  const retry = useRef(0);

  useEffect(() => {
    let ws: WebSocket | null = null;
    let timer: ReturnType<typeof setTimeout> | undefined;
    let stopped = false;

    const connect = () => {
      setConn("connecting");
      try {
        ws = new WebSocket(wsUrl());
      } catch {
        scheduleRetry();
        return;
      }
      ws.onopen = () => {
        retry.current = 0;
        setConn("open");
      };
      ws.onmessage = (msg) => {
        let ev: ServerEvent;
        try {
          ev = JSON.parse(msg.data);
        } catch {
          return;
        }
        if (ev.type === "status") {
          if (ev.mqtt) setMqtt(ev.mqtt);
          return;
        }
        if (ev.type === "dashboard") {
          setDashboard(ev.dashboard ?? null);
          return;
        }
        if (ev.type === "weather") {
          setWeatherVersion((v) => v + 1);
          return;
        }
        if (ev.type === "calendar") {
          setCalendarVersion((v) => v + 1);
          return;
        }
        if (ev.type === "ui") {
          if (ev.ui?.language) setLanguage(ev.ui.language);
          if (ev.ui) {
            setTitle(ev.ui.title ?? "");
            setUiLoaded(true);
          }
          return;
        }
        setDevices((prev) => applyEvent(prev, ev));
      };
      ws.onclose = () => {
        setConn("closed");
        scheduleRetry();
      };
    };
    const scheduleRetry = () => {
      if (stopped) return;
      const delay = Math.min(30000, 1000 * 2 ** retry.current++);
      timer = setTimeout(connect, delay);
    };
    connect();

    return () => {
      stopped = true;
      clearTimeout(timer);
      ws?.close();
    };
  }, []);

  // Remember the language locally too, so the next page load starts in the
  // right language before the WebSocket has delivered the server setting.
  const setLanguage = useCallback((lang: string) => {
    setLanguageState(lang);
    try {
      localStorage.setItem(LANGUAGE_KEY, lang);
    } catch {
      // private mode etc.: fine, the server setting arrives a moment later
    }
  }, []);

  // Optimistic local update while the command round-trips through MQTT.
  const patchState = useCallback((id: string, patch: Record<string, unknown>) => {
    setDevices((prev) =>
      prev[id] ? { ...prev, [id]: { ...prev[id], state: { ...prev[id].state, ...patch } } } : prev,
    );
  }, []);

  return {
    language,
    setLanguage,
    title,
    setTitle,
    uiLoaded,
    weatherVersion,
    calendarVersion,
    devices: Object.values(devices),
    byId: devices,
    mqtt,
    conn,
    dashboard,
    setDashboard,
    patchState,
  };
}

/** Pure reducer for device events (exported for tests). */
export function applyEvent(prev: Record<string, Device>, ev: ServerEvent): Record<string, Device> {
  switch (ev.type) {
    case "devices":
      // `devices` is omitted when the list is empty.
      return Object.fromEntries((ev.devices ?? []).map((d) => [d.id, d]));
    case "state":
      if (!prev[ev.id]) return prev;
      return { ...prev, [ev.id]: { ...prev[ev.id], state: ev.state ?? {} } };
    case "availability":
      if (!prev[ev.id]) return prev;
      return { ...prev, [ev.id]: { ...prev[ev.id], availability: ev.availability } };
    default:
      return prev; // status / dashboard events are handled by the hook
  }
}
