"use client";

import { useEffect, useState } from "react";
import { useI18n } from "@/lib/i18n";
import { applyCardSize, CARD_SIZES, loadCardSize, saveCardSize, type CardSize } from "@/lib/cardSize";

const LABEL: Record<CardSize, string> = {
  normal: "settings.cardSizeNormal",
  compact: "settings.cardSizeCompact",
  mini: "settings.cardSizeMini",
};

/** Normal / Compact / Mini light and group cards on this device. Applies at once. */
export function CardSizePicker() {
  const { t } = useI18n();
  const [size, setSize] = useState<CardSize>("normal");
  useEffect(() => setSize(loadCardSize()), []);

  const choose = (next: CardSize) => {
    setSize(next);
    saveCardSize(next);
    applyCardSize(next);
  };

  return (
    <div className="choice-row" role="radiogroup" aria-label={t("settings.cardSize")}>
      {CARD_SIZES.map((s) => (
        <button
          key={s}
          type="button"
          role="radio"
          aria-checked={size === s}
          className={`choice ${size === s ? "selected" : ""}`}
          onClick={() => choose(s)}
        >
          {t(LABEL[s])}
        </button>
      ))}
    </div>
  );
}
