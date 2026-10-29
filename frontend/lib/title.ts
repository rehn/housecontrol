/** Shown when no own title is set under Settings. */
export const DEFAULT_TITLE = "housecontrol";

/** Max length, same as the backend (settings.MaxTitleLen). */
export const MAX_TITLE_LEN = 60;

export function appTitle(title: string | undefined | null): string {
  return title?.trim() || DEFAULT_TITLE;
}
