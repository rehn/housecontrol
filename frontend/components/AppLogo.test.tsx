import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { AppLogo } from "./AppLogo";

afterEach(cleanup);

describe("AppLogo", () => {
  it("is a decorative icon next to the title (not read out)", () => {
    render(
      <h1>
        <AppLogo /> Hemma
      </h1>,
    );
    const svg = document.querySelector("svg.app-logo")!;
    expect(svg.getAttribute("aria-hidden")).toBe("true");
    expect(screen.getByRole("heading").textContent?.trim()).toBe("Hemma");
  });

  it("can be drawn at another size", () => {
    render(<AppLogo size={48} />);
    const svg = document.querySelector("svg.app-logo")!;
    expect(svg.getAttribute("width")).toBe("48");
    expect(svg.getAttribute("height")).toBe("48");
  });
});
