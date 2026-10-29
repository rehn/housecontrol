import type { NextConfig } from "next";

// `next build` exports static files to out/, which the Go backend serves
// (one container, same origin). In `next dev` the API defaults to
// http://localhost:8080 (see lib/api.ts).
const nextConfig: NextConfig = {
  output: "export",
  trailingSlash: true, // /settings -> out/settings/index.html
  devIndicators: false, // hide the Next.js "N" badge in `next dev`
  // start.sh runs `next dev` with NEXT_DIST_DIR=.next-dev so a `next build`
  // (verify.sh) can't overwrite the files the running dev server uses.
  distDir: process.env.NEXT_DIST_DIR || ".next",
};

export default nextConfig;
