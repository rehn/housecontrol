// Height of the light/group cards, chosen per device (kept in this browser
// only), like the light/dark choice.

export type CardSize = "normal" | "compact" | "mini";
export const CARD_SIZES: CardSize[] = ["normal", "compact", "mini"];
export const CARD_SIZE_KEY = "housecontrol.cardSize";

export function loadCardSize(): CardSize {
  try {
    const v = localStorage.getItem(CARD_SIZE_KEY);
    return v === "compact" || v === "mini" ? v : "normal";
  } catch {
    return "normal";
  }
}

export function saveCardSize(size: CardSize): void {
  try {
    if (size === "normal") localStorage.removeItem(CARD_SIZE_KEY);
    else localStorage.setItem(CARD_SIZE_KEY, size);
  } catch {
    // not remembered, but still applied for this visit
  }
}

/** compact/mini: data-card-size on <html> (see globals.css); normal: no attribute. */
export function applyCardSize(size: CardSize): void {
  const root = document.documentElement;
  if (size === "normal") root.removeAttribute("data-card-size");
  else root.setAttribute("data-card-size", size);
}

/** Inlined in <head> so the saved size is in place before the first paint. */
export const cardSizeScript = `try{var s=localStorage.getItem(${JSON.stringify(CARD_SIZE_KEY)});if(s==="compact"||s==="mini")document.documentElement.setAttribute("data-card-size",s)}catch(e){}`;
