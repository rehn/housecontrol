import type { Metadata, Viewport } from "next";
import { LiveProvider } from "@/lib/live";
import { themeScript } from "@/lib/theme";
import { titleScript } from "@/lib/title";
import "./globals.css";

export const metadata: Metadata = {
  title: "housecontrol",
  description: "Lights and outlets via zigbee2mqtt",
};

export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
  themeColor: [
    { media: "(prefers-color-scheme: light)", color: "#f4f4f2" },
    { media: "(prefers-color-scheme: dark)", color: "#141413" },
  ],
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    // suppressHydrationWarning: the theme script may set data-theme before React starts
    <html lang="en" suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{ __html: themeScript }} />
        <script dangerouslySetInnerHTML={{ __html: titleScript }} />
      </head>
      <body>
        <LiveProvider>{children}</LiveProvider>
      </body>
    </html>
  );
}
