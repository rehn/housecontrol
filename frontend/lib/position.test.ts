import { describe, expect, it, vi } from "vitest";
import type { Place } from "./api";
import { findMyPosition, type PositionDeps } from "./position";

const ipPlace: Place = { name: "Enköping", region: "Uppsala", country: "Sweden", latitude: 59.6356, longitude: 17.0764 };

function geolocation(result: { lat: number; lon: number } | "denied") {
  return {
    getCurrentPosition: vi.fn((ok: PositionCallback, fail?: PositionErrorCallback | null) => {
      if (result === "denied") fail?.({ code: 1, message: "User denied Geolocation" } as GeolocationPositionError);
      else ok({ coords: { latitude: result.lat, longitude: result.lon } } as GeolocationPosition);
    }),
  };
}

function deps(over: Partial<PositionDeps> = {}): PositionDeps {
  return {
    secure: true,
    geolocation: geolocation({ lat: 59.636071, lon: 17.077681 }),
    locateByIP: vi.fn(() => Promise.resolve(ipPlace)),
    lookupArea: vi.fn(() => Promise.resolve("Enköping, Uppsala län")),
    ...over,
  };
}

describe("findMyPosition", () => {
  it("uses the tablet's own position when the browser allows it", async () => {
    const d = deps();
    const p = await findMyPosition(d);
    expect(p).toEqual({
      name: "Enköping",
      area: "Enköping, Uppsala län",
      latitude: 59.6361,
      longitude: 17.0777,
      approximate: false,
    });
    expect(d.locateByIP).not.toHaveBeenCalled();
    expect(d.lookupArea).toHaveBeenCalledWith(59.6361, 17.0777);
  });

  it("falls back to the internet connection if the user says no", async () => {
    const d = deps({ geolocation: geolocation("denied") });
    const p = await findMyPosition(d);
    expect(p).toMatchObject({ name: "Enköping", latitude: 59.6356, longitude: 17.0764, approximate: true });
    expect(d.locateByIP).toHaveBeenCalled();
  });

  it("goes straight to the internet connection on plain http (no GPS allowed)", async () => {
    const gps = geolocation({ lat: 1, lon: 2 });
    const d = deps({ secure: false, geolocation: gps });
    const p = await findMyPosition(d);
    expect(gps.getCurrentPosition).not.toHaveBeenCalled();
    expect(p.approximate).toBe(true);
  });

  it("still works if the area name can't be looked up", async () => {
    const p = await findMyPosition(deps({ lookupArea: () => Promise.reject(new Error("down")) }));
    expect(p).toEqual({ name: "59.6361, 17.0777", latitude: 59.6361, longitude: 17.0777, approximate: false });

    const ip = await findMyPosition(
      deps({ secure: false, lookupArea: () => Promise.reject(new Error("down")) }),
    );
    expect(ip).toMatchObject({ name: "Enköping", approximate: true });
    expect(ip.area).toBeUndefined();
  });

  it("fails if no position can be found at all", async () => {
    await expect(
      findMyPosition(deps({ secure: false, locateByIP: () => Promise.reject(new Error("all down")) })),
    ).rejects.toThrow("all down");
  });
});
