"use client";

import Link from "next/link";
import { useEffect, useMemo, useState } from "react";
import { MqttBanner } from "@/components/MqttBanner";
import { DashboardView } from "@/components/dashboard/DashboardView";
import { TabBar, TabDialog } from "@/components/dashboard/TabBar";
import { GrafanaPage } from "@/components/GrafanaPage";
import { AuthButton } from "@/components/LoginForm";
import { useAuth } from "@/lib/auth";
import { saveDashboard } from "@/lib/api";
import {
  addTab,
  Dashboard,
  defaultTabs,
  Layout,
  moveTab,
  removeTab,
  setTabLayout,
  tabLayout,
  updateTab,
  upgradeDashboard,
} from "@/lib/dashboard";
import { errorText, useI18n } from "@/lib/i18n";
import { useLive } from "@/lib/live";
import { appTitle } from "@/lib/title";

/** The open tab lives in the address (#<tab id>), so reload and back work. */
const tabFromHash = () => (typeof window === "undefined" ? "" : decodeURIComponent(window.location.hash.slice(1)));

export default function Home() {
  const { devices, mqtt, conn, dashboard, setDashboard, patchState, title, weatherVersion, calendarVersion } =
    useLive();
  const { t } = useI18n();
  const { canEdit } = useAuth();
  const [editing, setEditing] = useState(false);
  const [tabDialog, setTabDialog] = useState<{ edit?: string } | null>(null);
  const [activeId, setActiveId] = useState("");

  useEffect(() => {
    const onHash = () => setActiveId(tabFromHash());
    onHash();
    window.addEventListener("hashchange", onHash);
    return () => window.removeEventListener("hashchange", onHash);
  }, []);

  // logged out (or the login ran out) while editing: leave edit mode
  useEffect(() => {
    if (!canEdit) {
      setEditing(false);
      setTabDialog(null);
    }
  }, [canEdit]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!error) return;
    const timer = setTimeout(() => setError(null), 5000);
    return () => clearTimeout(timer);
  }, [error]);

  // Until something is saved, one Home tab shows every light/outlet. Regenerate
  // only when the set of devices changes, so card ids stay stable while editing.
  const deviceKey = devices.map((d) => d.id).sort().join(",");
  const fallback = useMemo(() => defaultTabs(devices), [deviceKey]); // eslint-disable-line react-hooks/exhaustive-deps
  const shown: Dashboard = dashboard ? upgradeDashboard(dashboard) : fallback;
  const active = shown.tabs.find((tab) => tab.id === activeId) ?? shown.tabs[0];

  const select = (id: string) => {
    setActiveId(id);
    if (window.location.hash !== `#${id}`) history.pushState(null, "", `#${id}`);
  };

  const connected = conn === "open" && mqtt?.state === "connected";
  const statusText = connected
    ? t("status.live")
    : conn === "closed"
      ? t("status.offline")
      : mqtt
        ? t(`status.${mqtt.state}`)
        : t("status.connecting");

  const store = async (d: Dashboard) => {
    const res = await saveDashboard(d);
    setDashboard(res.dashboard);
  };

  /** Save the cards of the open tab (from the card editor). */
  const saveLayout = async (layout: Layout) => {
    await store(setTabLayout(shown, active.id, layout));
    setEditing(false);
  };

  /** Tab changes (add, rename, move, remove) are saved right away. */
  const saveTabs = async (d: Dashboard, open?: string, keepDialog = false) => {
    try {
      await store(d);
      if (!keepDialog) setTabDialog(null);
      if (open) select(open);
    } catch (e) {
      setError(errorText(t, e));
    }
  };

  const editedTab = tabDialog?.edit ? shown.tabs.find((tab) => tab.id === tabDialog.edit) : undefined;

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

      {dashboard !== undefined && (
        <TabBar
          tabs={shown.tabs}
          active={active.id}
          editing={editing}
          onSelect={select}
          onAdd={() => setTabDialog({})}
          onEdit={(id) => setTabDialog({ edit: id })}
        />
      )}

      <MqttBanner conn={conn} mqtt={mqtt} />
      {error && <div className="banner bad">{error}</div>}

      {connected && devices.length === 0 && active?.kind === "cards" && <p className="meta">{t("home.noDevices")}</p>}

      {dashboard !== undefined && active.kind === "cards" && (
        <DashboardView
          key={active.id}
          layout={tabLayout(active)}
          devices={devices}
          editing={editing}
          weatherVersion={weatherVersion}
          calendarVersion={calendarVersion}
          onSave={saveLayout}
          onCancel={() => setEditing(false)}
          onPatch={patchState}
          onError={setError}
        />
      )}

      {dashboard !== undefined && active.kind === "grafana" && (
        <>
          {editing && (
            <div className="edit-bar" role="toolbar" aria-label={t("dashboard.toolbar")}>
              <span className="meta">{t("tabs.grafanaEditHint")}</span>
              <span className="edit-bar-spacer" />
              <button type="button" className="btn primary" onClick={() => setEditing(false)}>
                {t("common.done")}
              </button>
            </div>
          )}
          <GrafanaPage key={active.id} tab={active} />
        </>
      )}

      {tabDialog && (
        <TabDialog
          key={tabDialog.edit ?? "new"}
          initial={editedTab}
          canRemove={shown.tabs.length > 1}
          onClose={() => setTabDialog(null)}
          onSubmit={(tab) => (editedTab ? saveTabs(updateTab(shown, tab)) : saveTabs(addTab(shown, tab), tab.id))}
          onRemove={() => editedTab && saveTabs(removeTab(shown, editedTab.id), shown.tabs.find((x) => x.id !== editedTab.id)?.id)}
          onMove={(step) => editedTab && saveTabs(moveTab(shown, editedTab.id, step), undefined, true)}
        />
      )}
    </main>
  );
}
