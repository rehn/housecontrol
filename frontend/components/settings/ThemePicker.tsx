"use client";

import { useEffect, useState } from "react";
import { useI18n } from "@/lib/i18n";
import { applyTheme, loadTheme, saveTheme, THEMES, type Theme } from "@/lib/theme";

const LABEL: Record<Theme, string> = { light: "settings.themeLight", dark: "settings.themeDark", auto: "settings.themeAuto" };

/** Light / Dark / Automatic for this device. Applies at once. */
export function ThemePicker() {
  const { t } = useI18n();
  const [theme, setTheme] = useState<Theme>("auto");
  useEffect(() => setTheme(loadTheme()), []);

  const choose = (next: Theme) => {
    setTheme(next);
    saveTheme(next);
    applyTheme(next);
  };

  return (
    <div className="choice-row" role="radiogroup" aria-label={t("settings.appearance")}>
      {THEMES.map((th) => (
        <button
          key={th}
          type="button"
          role="radio"
          aria-checked={theme === th}
          className={`choice ${theme === th ? "selected" : ""}`}
          onClick={() => choose(th)}
        >
          {t(LABEL[th])}
        </button>
      ))}
    </div>
  );
}
