"use client";

import { useEffect, useState } from "react";
import { Device, setDevice } from "@/lib/api";

interface Props {
  device: Device;
  onPatch: (id: string, patch: Record<string, unknown>) => void;
  onError: (msg: string) => void;
}

export function DeviceCard({ device, onPatch, onError }: Props) {
  const caps = device.capabilities;
  const stateKey = caps.stateProperty ?? "state";
  const isOn = device.state[stateKey] === "ON";
  const offline = device.availability === "offline";

  const send = async (payload: Record<string, unknown>) => {
    onPatch(device.id, payload);
    try {
      await setDevice(device.id, payload);
    } catch (e) {
      onError(`${device.name}: ${(e as Error).message}`);
    }
  };

  const power = typeof device.state.power === "number" ? (device.state.power as number) : undefined;

  return (
    <div className={`card ${isOn ? "on" : ""} ${offline ? "offline" : ""}`}>
      <div className="card-head">
        <div>
          <div className="name">{device.name}</div>
          <div className="meta">
            {[device.vendor, device.model].filter(Boolean).join(" · ")}
            {offline && " · offline"}
          </div>
        </div>
        {caps.onOff && (
          <button
            className={`toggle ${isOn ? "on" : ""}`}
            aria-pressed={isOn}
            aria-label={`Turn ${device.name} ${isOn ? "off" : "on"}`}
            disabled={offline}
            onClick={() => send({ state: isOn ? "OFF" : "ON" })}
          >
            <span />
          </button>
        )}
      </div>

      {caps.brightness && (
        <Slider
          label="Brightness"
          min={1}
          max={caps.brightnessMax ?? 254}
          value={num(device.state.brightness, 0)}
          disabled={offline}
          format={(v) => `${Math.round((v / (caps.brightnessMax ?? 254)) * 100)}%`}
          onCommit={(v) => send({ brightness: v, state: "ON" })}
        />
      )}

      {caps.colorTemp && (
        <Slider
          label="Warmth"
          min={caps.colorTempMin ?? 150}
          max={caps.colorTempMax ?? 500}
          value={num(device.state.color_temp, caps.colorTempMin ?? 150)}
          disabled={offline}
          className="ct"
          format={(v) => `${Math.round(1_000_000 / v)} K`}
          onCommit={(v) => send({ color_temp: v })}
        />
      )}

      {power !== undefined && <div className="meta power">{power} W</div>}
    </div>
  );
}

export function Slider(props: {
  label: string;
  min: number;
  max: number;
  value: number;
  disabled?: boolean;
  className?: string;
  format: (v: number) => string;
  onCommit: (v: number) => void;
}) {
  // Local value while dragging; only send to the backend on release.
  const [v, setV] = useState(props.value);
  const [dragging, setDragging] = useState(false);
  useEffect(() => {
    if (!dragging) setV(props.value);
  }, [props.value, dragging]);

  // Fires on pointer up, touch end and key up; only the first one after a
  // change sends the value.
  const commit = () => {
    if (!dragging) return;
    setDragging(false);
    if (v !== props.value) props.onCommit(v);
  };

  return (
    <label className={`slider ${props.className ?? ""}`}>
      <span className="slider-label">
        {props.label} <span className="meta">{props.format(v)}</span>
      </span>
      <input
        type="range"
        min={props.min}
        max={props.max}
        value={v}
        disabled={props.disabled}
        onChange={(e) => {
          setDragging(true);
          setV(Number(e.target.value));
        }}
        onPointerUp={commit}
        onTouchEnd={commit}
        onKeyUp={commit}
      />
    </label>
  );
}

function num(v: unknown, def: number): number {
  return typeof v === "number" ? v : def;
}
