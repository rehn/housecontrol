import type { WeatherCondition } from "@/lib/api";

// Coloured weather icons drawn as inline SVG (no external assets).
const SUN = "#f5c242";
const CLOUD = "#e6e6e3";
const CLOUD_DARK = "#b9b9b4";
const RAIN = "#4fa8e8";
const SNOW = "#ffffff";

const Sun = ({ cx = 12, cy = 12, r = 5.5 }) => <circle cx={cx} cy={cy} r={r} fill={SUN} />;

const Cloud = ({ dx = 0, dy = 0, fill = CLOUD }) => (
  <path
    transform={`translate(${dx} ${dy})`}
    fill={fill}
    d="M7 19a4 4 0 0 1-.6-7.96A5.5 5.5 0 0 1 17 9.3 4.35 4.35 0 1 1 17.3 19H7Z"
  />
);

const drops = (n: number, color = RAIN) =>
  Array.from({ length: n }, (_, i) => (
    <path key={i} d={`M${8 + i * 4} 20.5l-1 2.5`} stroke={color} strokeWidth={1.6} strokeLinecap="round" />
  ));

const flakes = (n: number) =>
  Array.from({ length: n }, (_, i) => <circle key={i} cx={8 + i * 4} cy={21.8} r={1.1} fill={SNOW} stroke={CLOUD_DARK} strokeWidth={0.4} />);

export function WeatherIcon({ condition, size = 48 }: { condition: WeatherCondition; size?: number }) {
  let body: React.ReactNode;
  switch (condition) {
    case "clear":
      body = <Sun r={6} />;
      break;
    case "mostly_clear":
      body = (
        <>
          <Sun cx={11} cy={10} r={6} />
          <Cloud dx={3} dy={4} />
        </>
      );
      break;
    case "partly_cloudy":
      body = (
        <>
          <Sun cx={16} cy={7.5} r={4.5} />
          <Cloud dy={1} />
        </>
      );
      break;
    case "cloudy":
      body = (
        <>
          <Cloud dx={2} dy={-3} fill={CLOUD_DARK} />
          <Cloud dy={1} />
        </>
      );
      break;
    case "fog":
      body = (
        <>
          <Cloud dy={-3} />
          <path d="M4 19h16M6 22h12" stroke={CLOUD_DARK} strokeWidth={1.8} strokeLinecap="round" />
        </>
      );
      break;
    case "light_rain":
    case "rain":
    case "heavy_rain":
    case "showers":
      body = (
        <>
          {condition === "showers" && <Sun cx={16} cy={5.5} r={4} />}
          <Cloud dy={-2} fill={condition === "heavy_rain" ? CLOUD_DARK : CLOUD} />
          {drops(condition === "light_rain" ? 2 : 3)}
        </>
      );
      break;
    case "thunder":
      body = (
        <>
          <Cloud dy={-2} fill={CLOUD_DARK} />
          <path d="M12.5 16.5 10 21h2.5l-1 3 3.5-5h-2.5l1-2.5Z" fill={SUN} />
        </>
      );
      break;
    case "sleet":
      body = (
        <>
          <Cloud dy={-2} />
          {drops(1)}
          <circle cx={14} cy={21.8} r={1.1} fill={SNOW} stroke={CLOUD_DARK} strokeWidth={0.4} />
        </>
      );
      break;
    case "light_snow":
    case "snow":
    case "snow_showers":
      body = (
        <>
          {condition === "snow_showers" && <Sun cx={16} cy={5.5} r={4} />}
          <Cloud dy={-2} />
          {flakes(condition === "light_snow" ? 2 : 3)}
        </>
      );
      break;
    default:
      body = <Cloud dy={1} fill={CLOUD_DARK} />;
  }
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" aria-hidden="true" data-icon={`weather-${condition}`}>
      {body}
    </svg>
  );
}
