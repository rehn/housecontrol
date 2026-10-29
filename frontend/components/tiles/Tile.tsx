"use client";

import type { ReactNode } from "react";
import { useI18n } from "@/lib/i18n";
import { useLongPress } from "@/lib/useLongPress";

export type TileTone = "light" | "group" | "switch" | "neutral";

/** Card frame: round icon, title, optional subtitle, then content. */
export function Tile(props: {
  icon: ReactNode;
  title: string;
  subtitle?: string;
  on?: boolean;
  tone?: TileTone;
  dimmed?: boolean;
  /** press and hold (or right-click) anywhere on the tile */
  onLongPress?: () => void;
  children?: ReactNode;
}) {
  const tone = props.tone ?? "light";
  const press = useLongPress(props.onLongPress);
  return (
    <div className={`tile switch-tile tone-${tone} ${props.on ? "on" : ""} ${props.dimmed ? "dimmed" : ""}`} {...press}>
      <div className="tile-head">
        <span className="tile-icon">{props.icon}</span>
        <span className="tile-titles">
          <span className="tile-title">{props.title}</span>
          {props.subtitle && <span className="tile-subtitle">{props.subtitle}</span>}
        </span>
      </div>
      {props.children}
    </div>
  );
}

/**
 * Big two-segment switch (like the Home Assistant tile toggle). The whole bar
 * is one large touch target; the lit half shows the current state.
 */
export function BigToggle(props: {
  on: boolean;
  label: string;
  disabled?: boolean;
  onToggle: () => void;
  onIcon: ReactNode;
  offIcon: ReactNode;
}) {
  const { t } = useI18n();
  return (
    <button
      type="button"
      className={`big-toggle ${props.on ? "on" : "off"}`}
      aria-pressed={props.on}
      aria-label={t(props.on ? "tile.turnOff" : "tile.turnOn", { name: props.label })}
      disabled={props.disabled}
      onClick={props.onToggle}
    >
      <span className="big-toggle-inner">
        <span className="seg seg-off">{props.offIcon}</span>
        <span className="seg seg-on">{props.onIcon}</span>
      </span>
    </button>
  );
}
