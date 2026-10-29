import type { Metadata, Viewport } from "next";
import { LiveProvider } from "@/lib/live";
import "./globals.css";

export const metadata: Metadata = {
  title: "housecontrol",
  description: "Lights and outlets via zigbee2mqtt",
};

export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body>
        <LiveProvider>{children}</LiveProvider>
      </body>
    </html>
  );
}
