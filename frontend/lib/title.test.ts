import { afterEach, describe, expect, it, vi } from "vitest";
import { keepTitle, rememberedTitle, rememberTitle, TITLE_KEY, titleScript } from "./title";

const tick = () => new Promise((r) => setTimeout(r, 0));

afterEach(() => {
  localStorage.clear();
  document.title = "";
  document.head.querySelectorAll('meta[name="apple-mobile-web-app-title"]').forEach((m) => m.remove());
  vi.restoreAllMocks();
});

describe("keepTitle", () => {
  it("sets the tab/bookmark title and the home-screen name", () => {
    const stop = keepTitle("Hemma");
    expect(document.title).toBe("Hemma");
    expect(document.querySelector('meta[name="apple-mobile-web-app-title"]')?.getAttribute("content")).toBe("Hemma");
    stop();
  });

  it("puts it back when something else changes it (Next.js does on navigation)", async () => {
    const stop = keepTitle("Hemma");
    document.title = "housecontrol";
    await tick();
    expect(document.title).toBe("Hemma");
    stop();
    document.title = "other";
    await tick();
    expect(document.title).toBe("other"); // stopped watching
  });
});

describe("remembered title", () => {
  it("is remembered in this browser", () => {
    expect(rememberedTitle()).toBeNull();
    rememberTitle("Hemma");
    expect(localStorage.getItem(TITLE_KEY)).toBe("Hemma");
    expect(rememberedTitle()).toBe("Hemma");
  });

  it("is applied by the head script before the page is drawn", () => {
    rememberTitle("Stugan");
    document.title = "housecontrol";
    new Function(titleScript)();
    expect(document.title).toBe("Stugan");
  });

  it("copes with blocked storage", () => {
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new Error("blocked");
    });
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new Error("blocked");
    });
    expect(() => rememberTitle("x")).not.toThrow();
    expect(rememberedTitle()).toBeNull();
    expect(() => new Function(titleScript)()).not.toThrow();
  });
});
