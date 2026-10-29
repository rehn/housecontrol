"use client";

import { useEffect, useState, type MouseEvent, type ReactNode } from "react";
import { useI18n } from "@/lib/i18n";

export interface SettingsSection {
  id: string;
  label: string;
  content: ReactNode;
}

const fromHash = (sections: SettingsSection[]) => {
  const id = typeof window === "undefined" ? "" : decodeURIComponent(window.location.hash.slice(1));
  return sections.some((s) => s.id === id) ? id : sections[0]?.id;
};

/**
 * Settings as a menu on the left and the chosen section on the right (on a
 * narrow screen the menu becomes a row of tabs above). The section is kept in
 * the address (#weather), so reload and the back button keep working.
 */
export function SettingsLayout({ sections }: { sections: SettingsSection[] }) {
  const { t } = useI18n();
  const [active, setActive] = useState(() => fromHash(sections));

  const ids = sections.map((s) => s.id).join(",");
  useEffect(() => {
    const onHash = () => setActive(fromHash(sections));
    onHash();
    window.addEventListener("hashchange", onHash);
    return () => window.removeEventListener("hashchange", onHash);
  }, [ids]); // eslint-disable-line react-hooks/exhaustive-deps

  const open = (e: MouseEvent, id: string) => {
    e.preventDefault();
    if (window.location.hash !== `#${id}`) history.pushState(null, "", `#${id}`);
    setActive(id);
  };

  const current = sections.find((s) => s.id === active) ?? sections[0];

  return (
    <div className="settings-layout">
      <nav className="settings-menu" aria-label={t("settings.menu")}>
        {sections.map((s) => (
          <a
            key={s.id}
            href={`#${s.id}`}
            className={`settings-menu-item ${s.id === current?.id ? "active" : ""}`}
            aria-current={s.id === current?.id ? "page" : undefined}
            onClick={(e) => open(e, s.id)}
          >
            {s.label}
          </a>
        ))}
      </nav>
      {current && (
        <section className="settings-content panel" aria-labelledby={`settings-h-${current.id}`}>
          <h2 id={`settings-h-${current.id}`}>{current.label}</h2>
          {current.content}
        </section>
      )}
    </div>
  );
}
