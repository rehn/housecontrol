// Grafana embedding: links are shown in iframes (needs allow_embedding = true
// in grafana.ini). Grafana draws everything itself, so it stays live and
// interactive and the app needs to know nothing about your data sources.

/** A http(s) link from a pasted link or Grafana's <iframe …> embed snippet; null if none. */
export function extractUrl(input: string): string | null {
  let s = input.trim();
  const m = s.match(/<iframe[^>]*\ssrc\s*=\s*["']([^"']+)["']/i);
  if (m) s = m[1].replace(/&amp;/g, "&").trim();
  try {
    const u = new URL(s);
    return (u.protocol === "http:" || u.protocol === "https:") && u.host ? s : null;
  } catch {
    return null;
  }
}

/**
 * A "view panel" link (…/d/<uid>/<slug>?viewPanel=3, what the browser shows
 * when a panel is opened) becomes the single-panel embed link
 * (…/d-solo/<uid>/<slug>?panelId=3). Anything else is returned unchanged.
 */
export function panelEmbedUrl(link: string): string {
  let u: URL;
  try {
    u = new URL(link);
  } catch {
    return link;
  }
  const panel = u.searchParams.get("viewPanel");
  if (!panel || !/\/d\//.test(u.pathname)) return link;
  u.pathname = u.pathname.replace(/\/d\//, "/d-solo/");
  u.searchParams.delete("viewPanel");
  u.searchParams.set("panelId", panel);
  return u.toString();
}

/**
 * The iframe address: adds theme=light|dark to match the app, and for whole
 * dashboards the kiosk flag (no Grafana menus). Values already in the link win.
 * Parameters are appended as text, because Grafana wants a bare "kiosk".
 */
export function embedSrc(link: string, opts: { kiosk?: boolean; theme?: "light" | "dark" }): string {
  let u: URL;
  try {
    u = new URL(link);
  } catch {
    return link;
  }
  const extra: string[] = [];
  if (opts.theme && !u.searchParams.has("theme")) extra.push(`theme=${opts.theme}`);
  if (opts.kiosk && !u.searchParams.has("kiosk")) extra.push("kiosk");
  if (extra.length === 0) return link;
  const [base, hash = ""] = link.split("#");
  const sep = base.includes("?") ? (base.endsWith("?") || base.endsWith("&") ? "" : "&") : "?";
  return base + sep + extra.join("&") + (hash ? `#${hash}` : "");
}
