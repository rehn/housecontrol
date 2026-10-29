import { afterEach, describe, expect, it, vi } from "vitest";
import { applyCardSize, CARD_SIZE_KEY, cardSizeScript, loadCardSize, saveCardSize } from "./cardSize";

afterEach(() => {
  localStorage.clear();
  document.documentElement.removeAttribute("data-card-size");
  vi.restoreAllMocks();
});

describe("card size", () => {
  it("is normal until something is chosen", () => {
    expect(loadCardSize()).toBe("normal");
  });

  it("is remembered on this device", () => {
    saveCardSize("mini");
    expect(localStorage.getItem(CARD_SIZE_KEY)).toBe("mini");
    expect(loadCardSize()).toBe("mini");
    saveCardSize("normal");
    expect(loadCardSize()).toBe("normal");
  });

  it("ignores junk and blocked storage", () => {
    localStorage.setItem(CARD_SIZE_KEY, "huge");
    expect(loadCardSize()).toBe("normal");
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new Error("blocked");
    });
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new Error("blocked");
    });
    expect(loadCardSize()).toBe("normal");
    expect(() => saveCardSize("mini")).not.toThrow();
  });

  it("sets data-card-size on <html> (none for normal)", () => {
    applyCardSize("compact");
    expect(document.documentElement.dataset.cardSize).toBe("compact");
    applyCardSize("normal");
    expect(document.documentElement.hasAttribute("data-card-size")).toBe(false);
  });

  it("is applied by the head script before the page is drawn", () => {
    localStorage.setItem(CARD_SIZE_KEY, "mini");
    new Function(cardSizeScript)();
    expect(document.documentElement.dataset.cardSize).toBe("mini");
  });
});
