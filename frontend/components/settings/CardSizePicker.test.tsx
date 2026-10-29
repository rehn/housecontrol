import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { I18nProvider } from "@/lib/i18n";
import { CARD_SIZE_KEY } from "@/lib/cardSize";
import { CardSizePicker } from "./CardSizePicker";

afterEach(() => {
  cleanup();
  localStorage.clear();
  document.documentElement.removeAttribute("data-card-size");
});

describe("CardSizePicker", () => {
  it("offers normal, compact and mini, normal first", () => {
    render(<CardSizePicker />);
    expect(screen.getAllByRole("radio").map((r) => r.textContent)).toEqual(["Normal", "Compact", "Mini"]);
    expect(screen.getByRole("radio", { name: "Normal" }).getAttribute("aria-checked")).toBe("true");
  });

  it("applies at once and remembers it on this device", () => {
    render(<CardSizePicker />);
    fireEvent.click(screen.getByRole("radio", { name: "Mini" }));
    expect(document.documentElement.dataset.cardSize).toBe("mini");
    expect(localStorage.getItem(CARD_SIZE_KEY)).toBe("mini");
    expect(screen.getByRole("radio", { name: "Mini" }).getAttribute("aria-checked")).toBe("true");
  });

  it("is in Swedish", () => {
    render(
      <I18nProvider language="sv">
        <CardSizePicker />
      </I18nProvider>,
    );
    expect(screen.getAllByRole("radio").map((r) => r.textContent)).toEqual(["Normal", "Kompakt", "Mini"]);
  });
});
