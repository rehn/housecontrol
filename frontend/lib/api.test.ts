import { afterEach, describe, expect, it, vi } from "vitest";
import { saveMqttSettings, setDevice, testMqttSettings } from "./api";

function mockFetch(impl: (url: string, init?: RequestInit) => Promise<Response>) {
  const fn = vi.fn(impl);
  vi.stubGlobal("fetch", fn);
  return fn;
}

const json = (status: number, body: unknown) =>
  Promise.resolve(new Response(JSON.stringify(body), { status }));

afterEach(() => vi.unstubAllGlobals());

describe("api", () => {
  it("setDevice posts the payload to the device's set endpoint", async () => {
    const fetch = mockFetch(() => Promise.resolve(new Response(null, { status: 202 })));
    await setDevice("0x01", { state: "ON" });
    const [url, init] = fetch.mock.calls[0];
    expect(url).toMatch(/\/api\/devices\/0x01\/set$/);
    expect(init?.method).toBe("POST");
    expect(JSON.parse(init?.body as string)).toEqual({ state: "ON" });
  });

  it("surfaces the backend's error message", async () => {
    mockFetch(() => json(503, { error: "not connected to the MQTT broker" }));
    await expect(setDevice("0x01", { state: "ON" })).rejects.toThrow(
      "not connected to the MQTT broker",
    );
  });

  it("gives a readable error when the backend is unreachable", async () => {
    mockFetch(() => Promise.reject(new TypeError("Failed to fetch")));
    await expect(setDevice("0x01", { state: "ON" })).rejects.toThrow(
      "cannot reach the housecontrol backend",
    );
  });

  it("saveMqttSettings omits the password when not given (keep current)", async () => {
    const fetch = mockFetch(() => json(200, {}));
    await saveMqttSettings({ url: "192.168.1.201", username: "", clientId: "", baseTopic: "" });
    const body = JSON.parse(fetch.mock.calls[0][1]?.body as string);
    expect(fetch.mock.calls[0][1]?.method).toBe("PUT");
    expect("password" in body).toBe(false);
  });

  it("testMqttSettings returns the result even when the connection fails", async () => {
    mockFetch(() => json(200, { ok: false, error: "connection refused", url: "tcp://x:1883" }));
    const r = await testMqttSettings({ url: "x", username: "", clientId: "", baseTopic: "" });
    expect(r).toEqual({ ok: false, error: "connection refused", url: "tcp://x:1883" });
  });
});
