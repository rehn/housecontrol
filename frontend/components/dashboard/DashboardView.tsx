"use client";

import { useEffect, useMemo, useState } from "react";
import {
  closestCorners,
  DndContext,
  DragEndEvent,
  DragOverEvent,
  DragOverlay,
  DragStartEvent,
  KeyboardSensor,
  MouseSensor,
  TouchSensor,
  useDroppable,
  useSensor,
  useSensors,
} from "@dnd-kit/core";
import {
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import type { Device } from "@/lib/api";
import {
  addCard,
  Card,
  Dashboard,
  defaultDashboard,
  MAX_COLUMNS,
  MIN_COLUMNS,
  moveCard,
  removeCard,
  setColumnCount,
  updateCard,
  usedDeviceIds,
} from "@/lib/dashboard";
import { ClockTile } from "@/components/tiles/ClockTile";
import { WeatherTile } from "@/components/tiles/WeatherTile";
import { CalendarTile } from "@/components/tiles/CalendarTile";
import { DeviceTile, TileActions } from "@/components/tiles/DeviceTile";
import { GroupTile } from "@/components/tiles/GroupTile";
import { CloseIcon, GripIcon, PencilIcon } from "@/components/icons";
import { CardDialog } from "./CardDialog";
import { errorText, useI18n } from "@/lib/i18n";

type Props = TileActions & {
  dashboard: Dashboard;
  devices: Device[];
  editing: boolean;
  /** bump to make weather cards reload (weather settings changed) */
  weatherVersion?: number;
  /** bump to make calendar cards reload (calendars changed) */
  calendarVersion?: number;
  onSave: (d: Dashboard) => Promise<void>;
  onCancel: () => void;
};

export function DashboardView(props: Props) {
  const { devices, editing } = props;
  const { t } = useI18n();
  const byId = useMemo(() => Object.fromEntries(devices.map((d) => [d.id, d])), [devices]);

  // Edits happen on a draft; only Save sends it to the server.
  const [draft, setDraft] = useState(props.dashboard);
  useEffect(() => {
    if (!editing) setDraft(props.dashboard);
  }, [editing, props.dashboard]);
  const shown = editing ? draft : props.dashboard;

  const [dialog, setDialog] = useState<{ card?: Card } | null>(null);
  const [activeId, setActiveId] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);

  // Touch: press and hold ~0.2 s to pick a card up, so normal swipes still scroll.
  const sensors = useSensors(
    useSensor(MouseSensor, { activationConstraint: { distance: 6 } }),
    useSensor(TouchSensor, { activationConstraint: { delay: 200, tolerance: 8 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );

  const columnOf = (id: string) =>
    draft.columns.find((c) => c.id === id || c.cards.some((x) => x.id === id))?.id;

  const onDragStart = (e: DragStartEvent) => setActiveId(String(e.active.id));
  const onDragOver = ({ active, over }: DragOverEvent) => {
    if (!over) return;
    const a = String(active.id);
    const o = String(over.id);
    if (columnOf(a) !== columnOf(o)) setDraft((d) => moveCard(d, a, o));
  };
  const onDragEnd = ({ active, over }: DragEndEvent) => {
    setActiveId(null);
    if (!over) return;
    const a = String(active.id);
    const o = String(over.id);
    if (a !== o && columnOf(a) === columnOf(o)) setDraft((d) => moveCard(d, a, o));
  };

  const cardName = (c: Card) =>
    c.type === "clock"
      ? t("tile.clock")
      : c.type === "weather"
        ? t("tile.weather")
        : c.type === "calendar"
          ? t("tile.calendar")
      : c.type === "group"
        ? c.title || t("tile.group")
        : c.title || byId[c.deviceId ?? ""]?.name || t("tile.missingDevice");

  const renderTile = (card: Card, preview: boolean) => {
    switch (card.type) {
      case "clock":
        return <ClockTile />;
      case "weather":
        return <WeatherTile refreshKey={props.weatherVersion} />;
      case "calendar":
        return <CalendarTile refreshKey={props.calendarVersion} />;
      case "group":
        return (
          <GroupTile card={card} devices={byId} onPatch={props.onPatch} onError={props.onError} preview={preview} />
        );
      case "device": {
        const d = byId[card.deviceId ?? ""];
        if (!d)
          return preview ? <div className="tile missing">{t("tile.deviceNotFound", { id: card.deviceId ?? "" })}</div> : null;
        return <DeviceTile card={card} device={d} onPatch={props.onPatch} onError={props.onError} preview={preview} />;
      }
    }
  };

  const save = async () => {
    setSaving(true);
    setSaveError(null);
    try {
      await props.onSave(draft);
    } catch (e) {
      setSaveError(errorText(t, e));
    } finally {
      setSaving(false);
    }
  };

  const activeCard = activeId ? draft.columns.flatMap((c) => c.cards).find((c) => c.id === activeId) : null;
  const cols = shown.columns.length;

  return (
    <>
      {editing && (
        <div className="edit-bar" role="toolbar" aria-label={t("dashboard.toolbar")}>
          <button type="button" className="btn primary" onClick={() => setDialog({})}>
            <span aria-hidden="true">+</span> {t("dashboard.addCard")}
          </button>
          <div className="stepper" aria-label={t("dashboard.columns")}>
            <button
              type="button"
              className="btn"
              aria-label={t("dashboard.fewerColumns")}
              disabled={cols <= MIN_COLUMNS}
              onClick={() => setDraft((d) => setColumnCount(d, d.columns.length - 1))}
            >
              −
            </button>
            <span className="stepper-value">{t("dashboard.columnsValue", { count: cols })}</span>
            <button
              type="button"
              className="btn"
              aria-label={t("dashboard.moreColumns")}
              disabled={cols >= MAX_COLUMNS}
              onClick={() => setDraft((d) => setColumnCount(d, d.columns.length + 1))}
            >
              +
            </button>
          </div>
          <button type="button" className="btn" onClick={() => setDraft(defaultDashboard(devices, cols))}>
            {t("dashboard.reset")}
          </button>
          <span className="edit-bar-spacer" />
          {saveError && <span className="edit-error">{saveError}</span>}
          <button type="button" className="btn" onClick={props.onCancel} disabled={saving}>
            {t("common.cancel")}
          </button>
          <button type="button" className="btn primary" onClick={save} disabled={saving}>
            {saving ? t("common.saving") : t("common.save")}
          </button>
        </div>
      )}
      {editing && <p className="meta edit-hint">{t("dashboard.hint")}</p>}

      <DndContext
        sensors={sensors}
        collisionDetection={closestCorners}
        onDragStart={onDragStart}
        onDragOver={onDragOver}
        onDragEnd={onDragEnd}
        onDragCancel={() => setActiveId(null)}
      >
        <div className={`dash-grid ${editing ? "editing" : ""}`} style={{ "--cols": cols } as React.CSSProperties}>
          {shown.columns.map((col) => (
            <DashColumn key={col.id} id={col.id} editing={editing}>
              <SortableContext items={col.cards.map((c) => c.id)} strategy={verticalListSortingStrategy}>
                {col.cards.map((card) =>
                  editing ? (
                    <SortableCard
                      key={card.id}
                      id={card.id}
                      name={cardName(card)}
                      editable={card.type === "device" || card.type === "group"}
                      onEdit={() => setDialog({ card })}
                      onRemove={() => setDraft((d) => removeCard(d, card.id))}
                    >
                      {renderTile(card, true)}
                    </SortableCard>
                  ) : (
                    <div key={card.id} className="dash-card">
                      {renderTile(card, false)}
                    </div>
                  ),
                )}
              </SortableContext>
            </DashColumn>
          ))}
        </div>
        <DragOverlay>
          {activeCard ? <div className="dash-card dragging-overlay">{renderTile(activeCard, true)}</div> : null}
        </DragOverlay>
      </DndContext>

      {dialog && (
        <CardDialog
          devices={devices}
          usedDeviceIds={usedDeviceIds(draft)}
          initial={dialog.card}
          onClose={() => setDialog(null)}
          onSubmit={(card) => {
            setDraft((d) => (dialog.card ? updateCard(d, card) : addCard(d, card)));
            setDialog(null);
          }}
        />
      )}
    </>
  );
}

function DashColumn(props: { id: string; editing: boolean; children: React.ReactNode }) {
  const { t } = useI18n();
  const { setNodeRef, isOver } = useDroppable({ id: props.id, disabled: !props.editing });
  return (
    <div ref={setNodeRef} className={`dash-col ${props.editing ? "editing" : ""} ${isOver ? "over" : ""}`}>
      {props.children}
      {props.editing && <div className="dash-col-drop">{t("dashboard.dropHere")}</div>}
    </div>
  );
}

function SortableCard(props: {
  id: string;
  name: string;
  editable: boolean;
  onEdit: () => void;
  onRemove: () => void;
  children: React.ReactNode;
}) {
  const { t } = useI18n();
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id: props.id });
  return (
    <div
      ref={setNodeRef}
      className={`dash-card edit ${isDragging ? "is-dragging" : ""}`}
      style={{ transform: CSS.Transform.toString(transform), transition }}
    >
      {/* the whole tile is the drag handle; its own controls are disabled in edit mode */}
      <div className="drag-area" {...attributes} {...listeners} aria-label={t("dashboard.move", { name: props.name })}>
        {props.children}
        <span className="grip" aria-hidden="true">
          <GripIcon />
        </span>
      </div>
      <div className="card-actions">
        {props.editable && (
          <button type="button" className="icon-btn" aria-label={t("dashboard.editCard", { name: props.name })} onClick={props.onEdit}>
            <PencilIcon />
          </button>
        )}
        <button type="button" className="icon-btn danger" aria-label={t("dashboard.remove", { name: props.name })} onClick={props.onRemove}>
          <CloseIcon />
        </button>
      </div>
    </div>
  );
}
