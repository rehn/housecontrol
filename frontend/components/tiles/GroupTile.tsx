"use client";

import { useState } from "react";
import { Device, setMany } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { errorText, useI18n } from "@/lib/i18n";
import { Card, groupSummary } from "@/lib/dashboard";
import { BulbIcon, BulbOffIcon } from "@/components/icons";
import { BigToggle, Tile } from "./Tile";
import { ControlPanel } from "./ControlPanel";
import type { TileActions } from "./DeviceTile";

/** Toggles several lights/outlets at once; hold for brightness of all. */
export function GroupTile({
  card,
  devices,
  onPatch,
  onError,
  preview,
}: TileActions & { card: Card; devices: Record<string, Device>; preview?: boolean }) {
  const { t } = useI18n();
  const { canEdit } = useAuth(); // without a login the tile only shows the state
  const [panelOpen, setPanelOpen] = useState(false);
  const title = card.title || t("tile.group");
  const s = groupSummary(devices, card.deviceIds ?? []);
  const missing = s.total - s.known;

  const send = async (payload: Record<string, unknown>) => {
    if (s.knownIds.length === 0) return;
    for (const id of s.knownIds) onPatch(id, payload);
    try {
      await setMany(s.knownIds, payload);
    } catch (e) {
      onError(`${title}: ${errorText(t, e)}`);
    }
  };
  const toggle = () => send({ state: s.nextState });
  // same icons as a single lamp: lit when anything in the group is on
  const icon = s.anyOn ? <BulbIcon /> : <BulbOffIcon />;

  const subtitle =
    s.known === 0
      ? t("tile.noDevicesFound")
      : t("tile.groupOn", { on: s.on, total: s.known }) +
        (missing > 0 ? ` · ${t("tile.groupMissing", { count: missing })}` : "");

  return (
    <>
      <Tile
        icon={icon}
        title={title}
        subtitle={subtitle}
        on={s.anyOn}
        tone="group"
        onLongPress={!canEdit || preview || s.known === 0 ? undefined : () => setPanelOpen(true)}
      >
        <BigToggle
          on={s.anyOn}
          label={title}
          disabled={!canEdit || preview || s.known === 0}
          onToggle={toggle}
          onIcon={<BulbIcon />}
          offIcon={<BulbOffIcon />}
        />
      </Tile>
      {panelOpen && (
        <ControlPanel
          title={title}
          subtitle={subtitle}
          icon={icon}
          tone="group"
          on={s.anyOn}
          toggleDisabled={s.known === 0}
          onToggle={toggle}
          brightness={
            s.brightness
              ? { value: s.anyOn ? s.avgBrightness : 0, min: 1, max: 254, onCommit: (v) => send({ brightness: v, state: "ON" }) }
              : undefined
          }
          onClose={() => setPanelOpen(false)}
        />
      )}
    </>
  );
}
