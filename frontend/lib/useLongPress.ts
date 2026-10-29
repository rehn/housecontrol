"use client";

import { useRef, type MouseEvent, type PointerEvent } from "react";

/** How long a finger must rest on a tile before the control panel opens. */
export const LONG_PRESS_MS = 500;
/** Moving further than this (px) means scrolling, not holding. */
const MOVE_TOLERANCE = 10;

/**
 * Press-and-hold for touch screens (also mouse), plus right-click on desktop.
 * Spread the returned handlers on an element. The click that browsers send
 * after a hold is swallowed, so holding a toggle doesn't also flip it.
 * Pass `undefined` to disable.
 */
export function useLongPress(onLongPress: (() => void) | undefined) {
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const start = useRef<{ x: number; y: number } | null>(null);
  const fired = useRef(false);

  if (!onLongPress) return {};

  const cancel = () => {
    clearTimeout(timer.current);
    timer.current = undefined;
    start.current = null;
  };

  const fire = () => {
    cancel();
    fired.current = true;
    onLongPress();
  };

  return {
    onPointerDown: (e: PointerEvent) => {
      if (e.button > 0) return; // right/middle mouse button
      fired.current = false;
      start.current = { x: e.clientX ?? 0, y: e.clientY ?? 0 };
      clearTimeout(timer.current);
      timer.current = setTimeout(fire, LONG_PRESS_MS);
    },
    onPointerMove: (e: PointerEvent) => {
      if (!start.current) return;
      const dx = (e.clientX ?? 0) - start.current.x;
      const dy = (e.clientY ?? 0) - start.current.y;
      if (Math.hypot(dx, dy) > MOVE_TOLERANCE) cancel();
    },
    onPointerUp: cancel,
    onPointerLeave: cancel,
    onPointerCancel: cancel,
    onClickCapture: (e: MouseEvent) => {
      if (fired.current) {
        fired.current = false;
        e.stopPropagation();
        e.preventDefault();
      }
    },
    // Right-click on desktop; on Android a long touch also sends this, so
    // only open if the timer hasn't already done it.
    onContextMenu: (e: MouseEvent) => {
      e.preventDefault();
      if (!fired.current) fire();
    },
  };
}
