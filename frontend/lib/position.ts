import type { Place } from "./api";

export interface FoundPosition {
  /** short name used as the place name, e.g. "Enköping" */
  name: string;
  /** looked-up area, e.g. "Enköping, Uppsala län" */
  area?: string;
  latitude: number;
  longitude: number;
  /** true when based on the internet connection, not the tablet's own position */
  approximate: boolean;
}

export interface PositionDeps {
  /** window.isSecureContext: browsers only share the position over https/localhost */
  secure: boolean;
  geolocation?: Pick<Geolocation, "getCurrentPosition">;
  locateByIP: () => Promise<Place>;
  lookupArea: (lat: number, lon: number) => Promise<string>;
}

const round4 = (n: number) => Math.round(n * 1e4) / 1e4;

function browserPosition(geo: Pick<Geolocation, "getCurrentPosition">): Promise<{ lat: number; lon: number }> {
  return new Promise((resolve, reject) =>
    geo.getCurrentPosition(
      (p) => resolve({ lat: p.coords.latitude, lon: p.coords.longitude }),
      reject,
      { enableHighAccuracy: false, timeout: 10_000, maximumAge: 10 * 60_000 },
    ),
  );
}

/**
 * Find "my position": the tablet's own position when the browser allows it
 * (https or localhost, and the user says yes), otherwise the home's rough
 * position from its internet connection (looked up by the backend).
 */
export async function findMyPosition(d: PositionDeps): Promise<FoundPosition> {
  if (d.secure && d.geolocation) {
    try {
      const pos = await browserPosition(d.geolocation);
      const latitude = round4(pos.lat);
      const longitude = round4(pos.lon);
      const area = await d.lookupArea(latitude, longitude).catch(() => "");
      return {
        name: area.split(",")[0].trim() || `${latitude.toFixed(4)}, ${longitude.toFixed(4)}`,
        ...(area ? { area } : {}),
        latitude,
        longitude,
        approximate: false,
      };
    } catch {
      // denied, unavailable or timed out: fall back to the internet connection
    }
  }

  const p = await d.locateByIP();
  const latitude = round4(p.latitude);
  const longitude = round4(p.longitude);
  const area = await d.lookupArea(latitude, longitude).catch(() => "");
  return {
    name: p.name || area.split(",")[0].trim() || `${latitude.toFixed(4)}, ${longitude.toFixed(4)}`,
    ...(area ? { area } : {}),
    latitude,
    longitude,
    approximate: true,
  };
}
