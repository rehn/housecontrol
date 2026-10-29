"use client";

import { useState } from "react";
import { Device, setDevice } from "@/lib/api";
import { errorText, useI18n } from "@/lib/i18n";
import type { Card } from "@/lib/dashboard";
import { BulbIcon, BulbOffIcon, PlugIcon } from "@/components/icons";
import { BigToggle, Tile } from "./Tile";
import { ControlPanel } from "./ControlPanel";

export interface TileActions {
  onPatch: (id: string, patch: Record<string, unknown>) => void;
  onError: (msg: string) => void;
}

export function DeviceTile({
  card,
  device,
  onPatch,
  onError,
  preview,
}: TileActions & { card: Card; device: Device; preview?: boolean }) {
  const { t } = useI18n();
  const [panelOpen, setPanelOpen] = useState(false);
  const caps = device.capabilities;
  const name = card.title || device.name;
  const isOn = device.state[caps.stateProperty ?? "state"] === "ON";
  const offline = device.availability === "offline";
  const isSwitch = device.kind === "switch";

  const send = async (payload: Record<string, unknown>) => {
    onPatch(device.id, payload);
    try {
      await setDevice(device.id, payload);
    } catch (e) {
      onError(`${name}: ${errorText(t, e)}`);
    }
  };
  const toggle = () => send({ state: isOn ? "OFF" : "ON" });

  const max = caps.brightnessMax ?? 254;
  const brightness = typeof device.state.brightness === "number" ? (device.state.brightness as number) : 0;
  const colorTemp = typeof device.state.color_temp === "number" ? (device.state.color_temp as number) : undefined;
  const power = typeof device.state.power === "number" ? (device.state.power as number) : undefined;
  const subtitle = offline
    ? t("tile.offline")
    : power !== undefined
      ? `${power} W`
      : isOn && caps.brightness
        ? `${Math.round((brightness / max) * 100)}%`
        : undefined;
  const icon = isSwitch ? <PlugIcon /> : isOn ? <BulbIcon /> : <BulbOffIcon />;
  const tone = isSwitch ? "switch" : "light";

  return (
    <>
      <Tile
        icon={icon}
        title={name}
        subtitle={subtitle}
        on={isOn}
        tone={tone}
        dimmed={offline}
        onLongPress={preview ? undefined : () => setPanelOpen(true)}
      >
        {caps.onOff && (
          <BigToggle
            on={isOn}
            label={name}
            disabled={offline || preview}
            onToggle={toggle}
            onIcon={isSwitch ? <PlugIcon /> : <BulbIcon />}
            offIcon={<BulbOffIcon />}
          />
        )}
      </Tile>
      {panelOpen && (
        <ControlPanel
          title={name}
          subtitle={subtitle}
          icon={icon}
          tone={tone}
          on={isOn}
          toggleDisabled={offline || !caps.onOff}
          onToggle={toggle}
          brightness={
            caps.brightness
              ? { value: isOn ? brightness : 0, min: 1, max, onCommit: (v) => send({ brightness: v, state: "ON" }) }
              : undefined
          }
          colorTemp={
            caps.colorTemp
              ? {
                  value: colorTemp ?? caps.colorTempMin ?? 250,
                  min: caps.colorTempMin ?? 150,
                  max: caps.colorTempMax ?? 500,
                  onCommit: (v) => send({ color_temp: v }),
                }
              : undefined
          }
          onClose={() => setPanelOpen(false)}
        />
      )}
    </>
  );
}
