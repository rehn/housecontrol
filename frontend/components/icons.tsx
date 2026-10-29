// Small inline icons (no icon font / external assets).
type P = { size?: number };

const svg = (size: number, children: React.ReactNode, name?: string) => (
  <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor" aria-hidden="true" data-icon={name}>
    {children}
  </svg>
);

export const BulbIcon = ({ size = 22 }: P) =>
  svg(
    size,
    <path d="M12 2a7 7 0 0 0-4 12.74V17a1 1 0 0 0 1 1h6a1 1 0 0 0 1-1v-2.26A7 7 0 0 0 12 2Zm-3 18h6v1a1 1 0 0 1-1 1h-4a1 1 0 0 1-1-1v-1Z" />,
    "bulb",
  );

export const BulbOffIcon = ({ size = 22 }: P) =>
  svg(
    size,
    <>
      <path d="M12 2a7 7 0 0 0-5.76 10.98L16.98 2.24A6.97 6.97 0 0 0 12 2Zm6.53 3.3L8 15.83V17a1 1 0 0 0 1 1h6a1 1 0 0 0 1-1v-2.26A7 7 0 0 0 18.53 5.3ZM9 20h6v1a1 1 0 0 1-1 1h-4a1 1 0 0 1-1-1v-1Z" />
      <path d="M3.3 19.3 19.3 3.3l1.4 1.4-16 16z" />
    </>,
    "bulb-off",
  );

export const PlugIcon = ({ size = 22 }: P) =>
  svg(
    size,
    <path d="M8 2a1 1 0 0 1 1 1v4h6V3a1 1 0 1 1 2 0v4h1a1 1 0 0 1 1 1v3a6 6 0 0 1-5 5.92V20a2 2 0 0 1-2 2h-0a2 2 0 0 1-2-2v-3.08A6 6 0 0 1 5 11V8a1 1 0 0 1 1-1h1V3a1 1 0 0 1 1-1Z" />,
    "plug",
  );

export const GroupIcon = ({ size = 22 }: P) =>
  svg(
    size,
    <path d="M6.5 4a4.5 4.5 0 0 0-2.5 8.24V14a1 1 0 0 0 1 1h3a1 1 0 0 0 1-1v-1.76A4.5 4.5 0 0 0 6.5 4Zm11 0a4.5 4.5 0 0 0-2.5 8.24V14a1 1 0 0 0 1 1h3a1 1 0 0 0 1-1v-1.76A4.5 4.5 0 0 0 17.5 4ZM12 9a4.5 4.5 0 0 0-2.5 8.24V19a1 1 0 0 0 1 1h3a1 1 0 0 0 1-1v-1.76A4.5 4.5 0 0 0 12 9Z" />,
    "group",
  );

export const PencilIcon = ({ size = 20 }: P) =>
  svg(size, <path d="M15.2 4.2a2.5 2.5 0 0 1 3.6 3.6L8 18.6 3 20l1.4-5L15.2 4.2Z" />, "pencil");

export const CloseIcon = ({ size = 20 }: P) =>
  svg(
    size,
    <path d="M5.3 5.3a1 1 0 0 1 1.4 0L12 10.6l5.3-5.3a1 1 0 1 1 1.4 1.4L13.4 12l5.3 5.3a1 1 0 0 1-1.4 1.4L12 13.4l-5.3 5.3a1 1 0 0 1-1.4-1.4L10.6 12 5.3 6.7a1 1 0 0 1 0-1.4Z" />,
    "close",
  );

export const GripIcon = ({ size = 20 }: P) =>
  svg(
    size,
    <path d="M9 5a1.5 1.5 0 1 1-3 0 1.5 1.5 0 0 1 3 0Zm9 0a1.5 1.5 0 1 1-3 0 1.5 1.5 0 0 1 3 0ZM9 12a1.5 1.5 0 1 1-3 0 1.5 1.5 0 0 1 3 0Zm9 0a1.5 1.5 0 1 1-3 0 1.5 1.5 0 0 1 3 0ZM9 19a1.5 1.5 0 1 1-3 0 1.5 1.5 0 0 1 3 0Zm9 0a1.5 1.5 0 1 1-3 0 1.5 1.5 0 0 1 3 0Z" />,
    "grip",
  );

export const ClockIcon = ({ size = 22 }: P) =>
  svg(
    size,
    <path d="M12 2a10 10 0 1 0 0 20 10 10 0 0 0 0-20Zm1 5v4.59l3.2 3.2-1.4 1.42L11 12.4V7h2Z" />,
    "clock",
  );

export const CalendarIcon = ({ size = 22 }: P) =>
  svg(
    size,
    <path d="M7 2a1 1 0 0 1 1 1v1h8V3a1 1 0 1 1 2 0v1h1a3 3 0 0 1 3 3v12a3 3 0 0 1-3 3H5a3 3 0 0 1-3-3V7a3 3 0 0 1 3-3h1V3a1 1 0 0 1 1-1ZM4 10v9a1 1 0 0 0 1 1h14a1 1 0 0 0 1-1v-9H4Zm3 3h3v3H7v-3Z" />,
    "calendar",
  );

export const ChevronLeftIcon = ({ size = 22 }: P) =>
  svg(size, <path d="M15.4 5.4 14 4l-8 8 8 8 1.4-1.4L8.8 12z" />, "chevron-left");

export const ChevronRightIcon = ({ size = 22 }: P) =>
  svg(size, <path d="M8.6 5.4 10 4l8 8-8 8-1.4-1.4 6.6-6.6z" />, "chevron-right");

/** arrow into a door: log in */
export const LoginIcon = ({ size = 22 }: P) =>
  svg(
    size,
    <path d="M11 3h8a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2h-8v-2h8V5h-8V3ZM10 7l-1.4 1.4 2.6 2.6H3v2h8.2l-2.6 2.6L10 17l5-5-5-5Z" />,
    "login",
  );

/** arrow out of a door: log out */
export const LogoutIcon = ({ size = 22 }: P) =>
  svg(
    size,
    <path d="M13 3H5a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h8v-2H5V5h8V3Zm3 4-1.4 1.4 2.6 2.6H9v2h8.2l-2.6 2.6L16 17l5-5-5-5Z" />,
    "logout",
  );
