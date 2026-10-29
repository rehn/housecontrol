/** The app's "h" monogram (same drawing as app/icon.svg), for the header. */
export function AppLogo({ size = 32 }: { size?: number }) {
  return (
    <svg className="app-logo" width={size} height={size} viewBox="0 0 64 64" aria-hidden="true" focusable="false">
      <rect width="64" height="64" rx="14" fill="#1e1e1c" />
      <path
        d="M20 50V14M20 34c0-7 4-10 10-10s10 3 10 10v16"
        fill="none"
        stroke="#f5c242"
        strokeWidth="7"
        strokeLinecap="round"
      />
      <circle cx="46" cy="16" r="5" fill="#f5c242" />
      <path d="M46 4v3M55 16h3M53.5 8.5l2-2" stroke="#f5c242" strokeWidth="2.5" strokeLinecap="round" />
    </svg>
  );
}
