"use client";

import { embedSrc } from "@/lib/grafana";
import type { Tab } from "@/lib/dashboard";
import { useI18n } from "@/lib/i18n";
import { effectiveTheme } from "@/lib/theme";

/** A whole Grafana dashboard filling the tab, in kiosk mode (no Grafana menus). */
export function GrafanaPage({ tab }: { tab: Tab }) {
  const { t } = useI18n();
  const url = tab.url ?? "";
  return (
    <div className="grafana-page">
      <iframe
        className="grafana-page-frame"
        title={tab.title || "Grafana"}
        src={embedSrc(url, { kiosk: true, theme: effectiveTheme() })}
        allow="fullscreen"
        referrerPolicy="no-referrer-when-downgrade"
      />
      <p className="meta grafana-help">
        {t("tabs.embedHelp")}{" "}
        <a href={url} target="_blank" rel="noopener noreferrer" className="inline-link">
          {t("tabs.openInGrafana")} ↗
        </a>
      </p>
    </div>
  );
}
