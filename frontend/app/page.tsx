"use client";

import Link from "next/link";
import { useEffect, useMemo, useState } from "react";
import { MqttBanner } from "@/components/MqttBanner";
import { DashboardView } from "@/components/dashboard/DashboardView";
import { AuthButton } from "@/components/LoginForm";
import { useAuth } from "@/lib/auth";
import { saveDashboard } from "@/lib/api";
import { Dashboard, defaultDashboard } from "@/lib/dashboard";
import { useI18n } from "@/lib/i18n";
import { useLive } from "@/lib/live";
import { appTitle } from "@/lib/title";

export default function Home() {
  const { devices, mqtt, conn, dashboard, setDashboard, patchState, title, weatherVersion, calendarVersion } =
    useLive();
  const { t } = useI18n();
  const { canEdit } = useAuth();
  const [editing, setEditing] = useState(false);

  // logged out (or the login ran out) while editing: leave edit mode
  useEffect(() => {
    if (!canEdit) setEditing(false);
  }, [canEdit]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!error) return;
    const timer = setTimeout(() => setError(null), 5000);
    return () => clearTimeout(timer);
  }, [error]);

  // Until a layout is saved, show every light/outlet. Regenerate only when
  // the set of devices changes, so card ids stay stable while editing.
  const deviceKey = devices.map((d) => d.id).sort().join(",");
  const fallback = useMemo(() => defaultDashboard(devices), [deviceKey]); // eslint-disable-line react-hooks/exhaustive-deps
  const shown: Dashboard = dashboard ?? fallback;

  const connected = conn === "open" && mqtt?.state === "connected";
  const statusText = connected
    ? t("status.live")
    : conn === "closed"
      ? t("status.offline")
      : mqtt
        ? t(`status.${mqtt.state}`)
        : t("status.connecting");

  const save = async (d: Dashboard) => {
    const res = await saveDashboard(d);
    setDashboard(res.dashboard);
    setEditing(false);
  };

  return (
    <main className="wide">
      <header>
        <h1>{appTitle(title)}</h1>
        <nav>
          <span className={`conn ${connected ? "open" : conn === "closed" ? "closed" : ""}`}>{statusText}</span>
          {!editing && canEdit && (
            <button type="button" className="navlink" onClick={() => setEditing(true)} disabled={dashboard === undefined}>
              {t("common.edit")}
            </button>
          )}
          {canEdit && (
            <Link href="/settings/" className="navlink">
              {t("common.settings")}
            </Link>
          )}
          {!editing && <AuthButton />}
        </nav>
      </header>

      <MqttBanner conn={conn} mqtt={mqtt} />
      {error && <div className="banner bad">{error}</div>}

      {connected && devices.length === 0 && <p className="meta">{t("home.noDevices")}</p>}

      {dashboard !== undefined && (
        <DashboardView
          dashboard={shown}
          devices={devices}
          editing={editing}
          weatherVersion={weatherVersion}
          calendarVersion={calendarVersion}
          onSave={save}
          onCancel={() => setEditing(false)}
          onPatch={patchState}
          onError={setError}
        />
      )}
    </main>
  );
}
