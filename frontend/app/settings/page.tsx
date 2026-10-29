"use client";

import Link from "next/link";
import { FormEvent, useEffect, useState } from "react";
import {
  getMqttSettings,
  getCalendarSettings,
  getWeatherSettings,
  locateByIP,
  lookupArea,
  MqttSettings,
  MqttSettingsInput,
  saveMqttSettings,
  saveCalendarSettings,
  saveUISettings,
  saveWeatherSettings,
  searchPlaces,
  testCalendarLink,
  testMqttSettings,
  type CalendarSetting,
  WeatherSettings,
} from "@/lib/api";
import { errorText, languageList, useI18n } from "@/lib/i18n";
import { TitleForm } from "@/components/settings/TitleForm";
import { WeatherForm } from "@/components/settings/WeatherForm";
import { CalendarForm } from "@/components/settings/CalendarForm";
import { LoginForm } from "@/components/LoginForm";
import { useAuth } from "@/lib/auth";
import { findMyPosition } from "@/lib/position";
import { useLive } from "@/lib/live";

interface Form {
  url: string;
  username: string;
  password: string;
  clearPassword: boolean;
  clientId: string;
  baseTopic: string;
}

type Msg = { kind: "ok" | "bad" | "info"; text: string } | null;

export default function SettingsPage() {
  const { mqtt } = useLive(); // live connection status
  const { t } = useI18n();
  const { ready, canEdit } = useAuth();

  return (
    <main className="narrow">
      <header>
        <h1>{t("settings.title")}</h1>
        <nav>
          <Link href="/" className="navlink">
            {t("common.backToDevices")}
          </Link>
        </nav>
      </header>

      {!ready ? null : !canEdit ? (
        <section className="panel">
          <h2>{t("auth.title")}</h2>
          <LoginForm />
        </section>
      ) : (
        <>
          <AccountSection />
          <GeneralSection />
          <LanguageSection />
          <WeatherSection />
          <CalendarSection />
          <MqttSection liveStatus={mqtt} />
        </>
      )}
    </main>
  );
}

function AccountSection() {
  const { required, username, logout } = useAuth();
  const { t } = useI18n();
  const [error, setError] = useState<string | null>(null);
  if (!required) {
    return <div className="banner warn">{t("auth.off")}</div>;
  }
  return (
    <section className="panel">
      <h2>{t("auth.account")}</h2>
      <div className="account-row">
        <span>{t("auth.loggedInAs", { name: username ?? "" })}</span>
        <button
          type="button"
          className="btn"
          onClick={() => logout().catch((e) => setError(errorText(t, e)))}
        >
          {t("auth.logout")}
        </button>
      </div>
      {error && <div className="banner bad">{error}</div>}
    </section>
  );
}

function GeneralSection() {
  const { title, setTitle } = useLive();
  const { t } = useI18n();
  const save = async (next: string) => {
    const res = await saveUISettings({ title: next });
    setTitle(res.title ?? "");
  };
  return (
    <section className="panel">
      <h2>{t("settings.general")}</h2>
      <TitleForm value={title} onSave={save} />
    </section>
  );
}

function WeatherSection() {
  const { t, lang } = useI18n();
  const [settings, setSettings] = useState<WeatherSettings | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = () => {
    setError(null);
    getWeatherSettings()
      .then(setSettings)
      .catch((e) => setError(t("settings.loadFailed", { error: errorText(t, e) })));
  };
  useEffect(load, []); // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <section className="panel">
      <h2>{t("settings.weather")}</h2>
      {error && (
        <div className="banner bad">
          <span>{error}</span>
          <button type="button" className="btn" onClick={load}>
            {t("common.retry")}
          </button>
        </div>
      )}
      {settings && (
        <WeatherForm
          initial={settings}
          onSave={async (w) => {
            const saved = await saveWeatherSettings(w);
            setSettings(saved);
            return saved;
          }}
          search={(q) => searchPlaces(q, lang)}
          locate={() =>
            findMyPosition({
              secure: window.isSecureContext,
              geolocation: navigator.geolocation,
              locateByIP,
              lookupArea: (lat, lon) => lookupArea(lat, lon, lang),
            })
          }
        />
      )}
    </section>
  );
}

function CalendarSection() {
  const { t } = useI18n();
  const [calendars, setCalendars] = useState<CalendarSetting[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = () => {
    setError(null);
    getCalendarSettings()
      .then(setCalendars)
      .catch((e) => setError(t("settings.loadFailed", { error: errorText(t, e) })));
  };
  useEffect(load, []); // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <section className="panel">
      <h2>{t("settings.calendars")}</h2>
      {error && (
        <div className="banner bad">
          <span>{error}</span>
          <button type="button" className="btn" onClick={load}>
            {t("common.retry")}
          </button>
        </div>
      )}
      {calendars && <CalendarForm initial={calendars} onSave={saveCalendarSettings} test={testCalendarLink} />}
    </section>
  );
}

function LanguageSection() {
  const { language, setLanguage } = useLive();
  const { t } = useI18n();
  const [error, setError] = useState<string | null>(null);

  const choose = async (code: string) => {
    const previous = language;
    setError(null);
    setLanguage(code); // switch right away; the server confirms to all clients
    try {
      await saveUISettings({ language: code });
    } catch (e) {
      setLanguage(previous);
      setError(t("settings.languageSaveFailed", { error: errorText(t, e) }));
    }
  };

  return (
    <section className="panel">
      <h2>{t("settings.language")}</h2>
      <p className="meta">{t("settings.languageHint")}</p>
      <div className="choice-row" role="radiogroup" aria-label={t("settings.language")}>
        {languageList.map((l) => (
          <button
            key={l.code}
            type="button"
            role="radio"
            aria-checked={language === l.code}
            lang={l.code}
            className={`choice ${language === l.code ? "selected" : ""}`}
            onClick={() => choose(l.code)}
          >
            {l.name}
          </button>
        ))}
      </div>
      {error && <div className="banner bad">{error}</div>}
    </section>
  );
}

function MqttSection({ liveStatus }: { liveStatus: MqttSettings["status"] | null }) {
  const { t } = useI18n();
  const [loaded, setLoaded] = useState<MqttSettings | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [form, setForm] = useState<Form>({
    url: "",
    username: "",
    password: "",
    clearPassword: false,
    clientId: "",
    baseTopic: "zigbee2mqtt",
  });
  const [busy, setBusy] = useState<"test" | "save" | null>(null);
  const [msg, setMsg] = useState<Msg>(null);

  const load = () => {
    setLoadError(null);
    getMqttSettings()
      .then((s) => {
        setLoaded(s);
        setForm({
          url: s.url,
          username: s.username,
          password: "",
          clearPassword: false,
          clientId: s.clientId,
          baseTopic: s.baseTopic || "zigbee2mqtt",
        });
      })
      .catch((e) => setLoadError(errorText(t, e)));
  };
  useEffect(load, []); // eslint-disable-line react-hooks/exhaustive-deps

  const input = (): MqttSettingsInput => {
    const i: MqttSettingsInput = {
      url: form.url.trim(),
      username: form.username.trim(),
      clientId: form.clientId.trim(),
      baseTopic: form.baseTopic.trim(),
    };
    if (form.clearPassword) i.password = "";
    else if (form.password !== "") i.password = form.password;
    // otherwise omitted: backend keeps the stored password
    return i;
  };

  const test = async () => {
    setBusy("test");
    setMsg({ kind: "info", text: t("settings.testingConnection") });
    try {
      const r = await testMqttSettings(input());
      setMsg(
        r.ok
          ? { kind: "ok", text: t("settings.testOk", { url: r.url }) }
          : { kind: "bad", text: t("settings.testFailed", { url: r.url, error: r.error ?? "" }) },
      );
    } catch (e) {
      setMsg({ kind: "bad", text: errorText(t, e) });
    } finally {
      setBusy(null);
    }
  };

  const save = async (e: FormEvent) => {
    e.preventDefault();
    setBusy("save");
    setMsg(null);
    try {
      const s = await saveMqttSettings(input());
      setLoaded(s);
      setForm((f) => ({ ...f, url: s.url, password: "", clearPassword: false }));
      setMsg({ kind: "ok", text: t("settings.saved") });
    } catch (e) {
      setMsg({ kind: "bad", text: errorText(t, e) });
    } finally {
      setBusy(null);
    }
  };

  const set = <K extends keyof Form>(k: K, v: Form[K]) => setForm((f) => ({ ...f, [k]: v }));
  const status = liveStatus ?? loaded?.status;

  return (
    <section className="panel">
      <h2>{t("settings.mqttBroker")}</h2>
      {status && (
        <p className={`status-line ${status.state}`}>
          <span className="dot" /> {statusText(t, status.state)}
          {status.broker && <span className="meta"> · {status.broker}</span>}
          {status.error && <span className="status-error">{status.error}</span>}
        </p>
      )}

      {loadError && (
        <div className="banner bad">
          <span>{t("settings.loadFailed", { error: loadError })}</span>
          <button type="button" className="btn" onClick={load}>
            {t("common.retry")}
          </button>
        </div>
      )}

      <form onSubmit={save}>
        <Field label={t("settings.broker")} hint={t("settings.brokerHint")}>
          <input
            value={form.url}
            onChange={(e) => set("url", e.target.value)}
            placeholder="tcp://192.168.1.201:1883"
            autoComplete="off"
            spellCheck={false}
          />
        </Field>

        <div className="row">
          <Field label={t("settings.username")}>
            <input value={form.username} onChange={(e) => set("username", e.target.value)} autoComplete="off" />
          </Field>
          <Field label={t("settings.password")}>
            <input
              type="password"
              value={form.password}
              disabled={form.clearPassword}
              onChange={(e) => set("password", e.target.value)}
              placeholder={loaded?.hasPassword ? t("settings.passwordUnchanged") : ""}
              autoComplete="new-password"
            />
          </Field>
        </div>
        {loaded?.hasPassword && (
          <label className="check">
            <input
              type="checkbox"
              checked={form.clearPassword}
              onChange={(e) => set("clearPassword", e.target.checked)}
            />
            {t("settings.removePassword")}
          </label>
        )}

        <div className="row">
          <Field label={t("settings.baseTopic")}>
            <input
              value={form.baseTopic}
              onChange={(e) => set("baseTopic", e.target.value)}
              placeholder="zigbee2mqtt"
              spellCheck={false}
            />
          </Field>
          <Field label={t("settings.clientId")} hint={t("settings.optional")}>
            <input
              value={form.clientId}
              onChange={(e) => set("clientId", e.target.value)}
              placeholder={t("settings.random")}
              spellCheck={false}
            />
          </Field>
        </div>

        {msg && <div className={`banner ${msg.kind}`}>{msg.text}</div>}

        <div className="actions">
          <button type="button" className="btn" onClick={test} disabled={busy !== null || !form.url.trim()}>
            {busy === "test" ? t("settings.testing") : t("settings.testConnection")}
          </button>
          <button type="submit" className="btn primary" disabled={busy !== null}>
            {busy === "save" ? t("common.saving") : t("settings.saveConnect")}
          </button>
        </div>
        {loaded?.source === "config" && <p className="meta">{t("settings.fromConfig")}</p>}
      </form>
    </section>
  );
}

function Field(props: { label: string; hint?: string; children: React.ReactNode }) {
  return (
    <label className="field">
      <span className="field-label">
        {props.label} {props.hint && <span className="meta">{props.hint}</span>}
      </span>
      {props.children}
    </label>
  );
}

function statusText(t: (k: string) => string, s: string) {
  switch (s) {
    case "connected":
      return t("settings.stateConnected");
    case "connecting":
      return t("settings.stateConnecting");
    case "error":
      return t("settings.stateError");
    default:
      return t("settings.stateUnconfigured");
  }
}
