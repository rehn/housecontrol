/** Shown when no own title is set under Settings. */
export const DEFAULT_TITLE = "housecontrol";

/** Max length, same as the backend (settings.MaxTitleLen). */
export const MAX_TITLE_LEN = 60;

export function appTitle(title: string | undefined | null): string {
  return title?.trim() || DEFAULT_TITLE;
}

/** The last title this browser saw, so the page starts with it (bookmarks!). */
export const TITLE_KEY = "housecontrol.title";

export function rememberTitle(title: string): void {
  try {
    localStorage.setItem(TITLE_KEY, title);
  } catch {
    // storage blocked: the title is still set, just not remembered
  }
}

export function rememberedTitle(): string | null {
  try {
    return localStorage.getItem(TITLE_KEY) || null;
  } catch {
    return null;
  }
}

/** Inlined in <head>: use the remembered title before anything is drawn. */
export const titleScript = `try{var t=localStorage.getItem(${JSON.stringify(TITLE_KEY)});if(t)document.title=t}catch(e){}`;

/**
 * Sets the tab/bookmark title and the iOS home-screen name, and puts the
 * title back whenever something else changes it (Next.js rewrites <title> on
 * page changes). Returns a function that stops watching.
 */
export function keepTitle(title: string): () => void {
  const apply = () => {
    if (document.title !== title) document.title = title;
    let meta = document.head.querySelector<HTMLMetaElement>('meta[name="apple-mobile-web-app-title"]');
    if (!meta) {
      meta = document.createElement("meta");
      meta.name = "apple-mobile-web-app-title";
      document.head.appendChild(meta);
    }
    if (meta.content !== title) meta.content = title;
  };
  apply();
  const watch = new MutationObserver(() => {
    if (document.title !== title) apply();
  });
  watch.observe(document.head, { childList: true, subtree: true, characterData: true });
  return () => watch.disconnect();
}
