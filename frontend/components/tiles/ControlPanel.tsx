"use client";

import { useEffect, type ReactNode } from "react";
import { Slider } from "@/components/DeviceCard";
import { BulbIcon, BulbOffIcon } from "@/components/icons";
import { useI18n } from "@/lib/i18n";
import { BigToggle, type TileTone } from "./Tile";

interface Range {
  value: number;
  min: number;
  max: number;
  onCommit: (v: number) => void;
}

/** Panel opened by holding a tile: on/off, brightness and colour temperature. */
export function ControlPanel(props: {
  title: string;
  subtitle?: string;
  icon: ReactNode;
  tone: TileTone;
  on: boolean;
  toggleDisabled?: boolean;
  onToggle: () => void;
  brightness?: Range;
  colorTemp?: Range;
  onClose: () => void;
}) {
  const { t } = useI18n();
  const { onClose } = props;

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  const pct = (v: number, max: number) => `${Math.round((v / max) * 100)}%`;

  return (
    <div className="dialog-backdrop" onClick={onClose}>
      <div
        className={`dialog control-panel tone-${props.tone} ${props.on ? "on" : ""}`}
        role="dialog"
        aria-modal="true"
        aria-labelledby="control-panel-title"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="tile-head">
          <span className="tile-icon">{props.icon}</span>
          <span className="tile-titles">
            <span className="tile-title" id="control-panel-title">
              {props.title}
            </span>
            {props.subtitle && <span className="tile-subtitle">{props.subtitle}</span>}
          </span>
        </div>

        <BigToggle
          on={props.on}
          label={props.title}
          disabled={props.toggleDisabled}
          onToggle={props.onToggle}
          onIcon={<BulbIcon />}
          offIcon={<BulbOffIcon />}
        />

        {props.brightness && (
          <Slider
            label={t("tile.brightness")}
            min={props.brightness.min}
            max={props.brightness.max}
            value={props.brightness.value}
            className="tile-slider panel-slider"
            format={(v) => pct(v, props.brightness!.max)}
            onCommit={props.brightness.onCommit}
          />
        )}

        {props.colorTemp && (
          <Slider
            label={t("panel.warmth")}
            min={props.colorTemp.min}
            max={props.colorTemp.max}
            value={props.colorTemp.value}
            className="tile-slider panel-slider ct"
            format={(v) => `${Math.round(1_000_000 / v)} K`}
            onCommit={props.colorTemp.onCommit}
          />
        )}

        <div className="actions">
          <button type="button" className="btn" onClick={onClose}>
            {t("panel.close")}
          </button>
        </div>
      </div>
    </div>
  );
}
