import { describe, expect, it } from "vitest";
import { embedSrc, extractUrl, panelEmbedUrl } from "./grafana";

describe("extractUrl", () => {
  it("takes a plain link", () => {
    expect(extractUrl("  https://grafana.hemma.lan/d/abc/el?orgId=1 ")).toBe("https://grafana.hemma.lan/d/abc/el?orgId=1");
  });

  it("takes the src from Grafana's embed snippet", () => {
    const snippet =
      '<iframe src="https://grafana.hemma.lan/d-solo/abc/el?orgId=1&amp;from=now-6h&amp;panelId=2" width="450" height="200" frameborder="0"></iframe>';
    expect(extractUrl(snippet)).toBe("https://grafana.hemma.lan/d-solo/abc/el?orgId=1&from=now-6h&panelId=2");
  });

  it("refuses anything that isn't http(s)", () => {
    expect(extractUrl("")).toBeNull();
    expect(extractUrl("grafana.lan/d/x")).toBeNull();
    expect(extractUrl("javascript:alert(1)")).toBeNull();
    expect(extractUrl('<iframe src="file:///etc/passwd">')).toBeNull();
  });
});

describe("panelEmbedUrl", () => {
  it("turns a 'view panel' link into the embeddable single-panel link", () => {
    expect(panelEmbedUrl("https://g.lan/d/abc/el?orgId=1&viewPanel=3")).toBe("https://g.lan/d-solo/abc/el?orgId=1&panelId=3");
    expect(panelEmbedUrl("https://g.lan/grafana/d/abc/el?viewPanel=panel-7")).toBe(
      "https://g.lan/grafana/d-solo/abc/el?panelId=panel-7",
    );
  });

  it("leaves embed links and other links alone", () => {
    expect(panelEmbedUrl("https://g.lan/d-solo/abc/el?panelId=2")).toBe("https://g.lan/d-solo/abc/el?panelId=2");
    expect(panelEmbedUrl("https://g.lan/d/abc/el")).toBe("https://g.lan/d/abc/el");
  });
});

describe("embedSrc", () => {
  it("shows a whole dashboard in kiosk mode, in the app's light/dark look", () => {
    const src = embedSrc("https://g.lan/d/abc/el?orgId=1", { kiosk: true, theme: "dark" });
    const u = new URL(src);
    expect(u.searchParams.has("kiosk")).toBe(true);
    expect(u.searchParams.get("theme")).toBe("dark");
    expect(u.searchParams.get("orgId")).toBe("1");
  });

  it("does not override choices already in the link", () => {
    const u = new URL(embedSrc("https://g.lan/d/abc/el?theme=light&kiosk", { kiosk: true, theme: "dark" }));
    expect(u.searchParams.get("theme")).toBe("light");
    expect(u.searchParams.getAll("kiosk")).toHaveLength(1);
  });

  it("panels get the theme but no kiosk", () => {
    const u = new URL(embedSrc("https://g.lan/d-solo/abc/el?panelId=2", { theme: "light" }));
    expect(u.searchParams.has("kiosk")).toBe(false);
    expect(u.searchParams.get("theme")).toBe("light");
  });
});
