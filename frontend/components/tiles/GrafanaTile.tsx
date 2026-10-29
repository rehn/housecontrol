"use client";

import { embedSrc } from "@/lib/grafana";
import { GRAFANA_HEIGHT, type Card } from "@/lib/dashboard";
import { effectiveTheme } from "@/lib/theme";

/** One Grafana panel (d-solo link) as a card. */
export function GrafanaTile({ card }: { card: Card }) {
  const title = card.title || "Grafana";
  return (
    <div className="tile grafana-tile">
      {card.title && <div className="grafana-title">{card.title}</div>}
      <iframe
        className="grafana-frame"
        title={title}
        src={embedSrc(card.url ?? "", { theme: effectiveTheme() })}
        style={{ height: `${card.height ?? GRAFANA_HEIGHT}px` }}
        loading="lazy"
        referrerPolicy="no-referrer-when-downgrade"
      />
    </div>
  );
}
