import { describe, expect, it } from "vitest";
import type { Device } from "./api";
import { applyEvent } from "./useDevices";

const lamp: Device = {
  id: "0x01",
  name: "lamp",
  kind: "light",
  type: "Router",
  capabilities: { onOff: true, brightness: true, colorTemp: false, color: false, power: false },
  availability: "online",
  state: { state: "OFF" },
};

describe("applyEvent", () => {
  it("replaces the device list", () => {
    const next = applyEvent({}, { type: "devices", devices: [lamp] });
    expect(Object.keys(next)).toEqual(["0x01"]);
  });

  it("treats a missing device list as empty (regression: crashed on .map of undefined)", () => {
    const next = applyEvent({ "0x01": lamp }, { type: "devices" });
    expect(next).toEqual({});
  });

  it("updates state of a known device", () => {
    const next = applyEvent({ "0x01": lamp }, { type: "state", id: "0x01", state: { state: "ON" } });
    expect(next["0x01"].state).toEqual({ state: "ON" });
  });

  it("ignores state for unknown devices", () => {
    const prev = { "0x01": lamp };
    expect(applyEvent(prev, { type: "state", id: "0x99", state: { state: "ON" } })).toBe(prev);
  });

  it("updates availability", () => {
    const next = applyEvent({ "0x01": lamp }, { type: "availability", id: "0x01", availability: "offline" });
    expect(next["0x01"].availability).toBe("offline");
  });

  it("ignores status events", () => {
    const prev = { "0x01": lamp };
    expect(
      applyEvent(prev, { type: "status", mqtt: { state: "connected", since: "" } }),
    ).toBe(prev);
  });
});
