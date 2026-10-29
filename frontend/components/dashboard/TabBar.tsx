"use client";

import { useEffect, useState } from "react";
import { newId, type Tab, type TabKind } from "@/lib/dashboard";
import { extractUrl } from "@/lib/grafana";
import { useI18n } from "@/lib/i18n";
import { ChartIcon, PencilIcon } from "@/components/icons";

type T = ReturnType<typeof useI18n>["t"];

/** The name shown for a tab; unnamed tabs are "Home" (cards) or "Grafana". */
export const tabName = (tab: Tab, t: T) => tab.title || (tab.kind === "grafana" ? "Grafana" : t("tabs.home"));

/** Tabs under the title. Hidden when there is only one tab, except while editing. */
export function TabBar(props: {
  tabs: Tab[];
  active: string;
  editing: boolean;
  onSelect: (id: string) => void;
  onAdd: () => void;
  onEdit: (id: string) => void;
}) {
  const { t } = useI18n();
  if (props.tabs.length <= 1 && !props.editing) return null;
  return (
    <div className="tab-bar">
      <div className="tab-list" role="tablist" aria-label={t("tabs.label")}>
        {props.tabs.map((tab) => {
          const selected = tab.id === props.active;
          return (
            <button
              key={tab.id}
              type="button"
              role="tab"
              aria-selected={selected}
              className={`tab ${selected ? "active" : ""}`}
              onClick={() => props.onSelect(tab.id)}
            >
              {tabName(tab, t)}
            </button>
          );
        })}
      </div>
      {props.editing && (
        <div className="tab-actions">
          {props.tabs.map(
            (tab) =>
              tab.id === props.active && (
                <button
                  key={tab.id}
                  type="button"
                  className="icon-btn"
                  aria-label={t("tabs.edit", { name: tabName(tab, t) })}
                  onClick={() => props.onEdit(tab.id)}
                >
                  <PencilIcon />
                </button>
              ),
          )}
          <button type="button" className="btn" onClick={props.onAdd}>
            <span aria-hidden="true">+</span> {t("tabs.add")}
          </button>
        </div>
      )}
    </div>
  );
}

/** Add a tab (page of cards or Grafana dashboard), or rename / move / remove one. */
export function TabDialog(props: {
  initial?: Tab;
  canRemove?: boolean;
  onSubmit: (tab: Tab) => void;
  onRemove?: () => void;
  onMove?: (step: -1 | 1) => void;
  onClose: () => void;
}) {
  const { t } = useI18n();
  const editing = !!props.initial;
  const [title, setTitle] = useState(props.initial?.title ?? "");
  const [kind, setKind] = useState<TabKind>(props.initial?.kind ?? "cards");
  const [link, setLink] = useState(props.initial?.url ?? "");
  const [confirmRemove, setConfirmRemove] = useState(false);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && props.onClose();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [props]);

  const url = extractUrl(link);
  const badLink = kind === "grafana" && link.trim() !== "" && !url;
  // new tabs need a name; an existing unnamed tab (Home) may stay unnamed
  const nameOk = title.trim() !== "" || (editing && !props.initial?.title);
  const valid = nameOk && (kind === "cards" || !!url);

  const submit = () => {
    const base: Tab = props.initial ?? {
      id: newId("tab"),
      kind,
      ...(kind === "cards" ? { columns: [1, 2, 3].map(() => ({ id: newId("col"), cards: [] })) } : {}),
    };
    const trimmed = title.trim();
    const tab: Tab = { ...base, kind, title: trimmed || undefined };
    if (kind === "grafana") tab.url = url ?? undefined;
    props.onSubmit(tab);
  };

  const kinds: { kind: TabKind; label: string; hint: string }[] = [
    { kind: "cards", label: t("tabs.kindCards"), hint: t("tabs.kindCardsHint") },
    { kind: "grafana", label: t("tabs.kindGrafana"), hint: t("tabs.kindGrafanaHint") },
  ];

  return (
    <div className="dialog-backdrop" onClick={props.onClose}>
      <div className="dialog" role="dialog" aria-modal="true" aria-labelledby="tab-dialog-title" onClick={(e) => e.stopPropagation()}>
        <h2 id="tab-dialog-title">{editing ? t("tabs.dialogEdit") : t("tabs.dialogAdd")}</h2>

        <label className="field">
          <span className="field-label">{t("tabs.name")}</span>
          <input value={title} onChange={(e) => setTitle(e.target.value)} maxLength={30} />
        </label>

        {!editing && (
          <div className="type-picker" role="radiogroup" aria-label={t("tabs.kind")}>
            {kinds.map((k) => (
              <button
                key={k.kind}
                type="button"
                role="radio"
                aria-checked={kind === k.kind}
                className={`type-option ${kind === k.kind ? "selected" : ""}`}
                onClick={() => setKind(k.kind)}
              >
                {k.kind === "grafana" && (
                  <span className="type-icon">
                    <ChartIcon size={28} />
                  </span>
                )}
                <span className="type-label">{k.label}</span>
                <span className="type-hint">{k.hint}</span>
              </button>
            ))}
          </div>
        )}

        {kind === "grafana" && (
          <label className="field">
            <span className="field-label">{t("tabs.url")}</span>
            <input
              value={link}
              onChange={(e) => setLink(e.target.value)}
              placeholder="https://grafana…/d/…"
              autoComplete="off"
              spellCheck={false}
            />
          </label>
        )}
        {kind === "grafana" && <p className="meta field-hint">{t("tabs.urlHint")}</p>}
        {badLink && <p className="field-error">{t("tabs.invalidUrl")}</p>}

        {editing && (
          <div className="actions tab-dialog-tools">
            <button type="button" className="btn" onClick={() => props.onMove?.(-1)}>
              <span aria-hidden="true">←</span> {t("tabs.moveLeft")}
            </button>
            <button type="button" className="btn" onClick={() => props.onMove?.(1)}>
              {t("tabs.moveRight")} <span aria-hidden="true">→</span>
            </button>
            <span className="edit-bar-spacer" />
            <button
              type="button"
              className="btn danger"
              disabled={!props.canRemove}
              onClick={() => (confirmRemove ? props.onRemove?.() : setConfirmRemove(true))}
            >
              {confirmRemove ? t("tabs.removeConfirm") : t("tabs.remove")}
            </button>
          </div>
        )}

        <div className="actions">
          <button type="button" className="btn" onClick={props.onClose}>
            {t("common.cancel")}
          </button>
          <button type="button" className="btn primary" disabled={!valid} onClick={submit}>
            {editing ? t("common.done") : t("common.add")}
          </button>
        </div>
      </div>
    </div>
  );
}
