"use client";

// One WebSocket connection for the whole app, shared by all pages, plus the
// language provider fed by the server's UI setting and the login state.

import { createContext, useContext, useEffect, type ReactNode } from "react";
import { AuthProvider } from "./auth";
import { appTitle, keepTitle, rememberedTitle, rememberTitle } from "./title";
import { I18nProvider } from "./i18n";
import { useDevices } from "./useDevices";

type Live = ReturnType<typeof useDevices>;

const LiveContext = createContext<Live | null>(null);

export function LiveProvider({ children }: { children: ReactNode }) {
  const live = useDevices();
  const tabTitle = appTitle(live.title);
  // Until the server has told us the title, keep the one this browser saw last.
  useEffect(() => {
    if (live.uiLoaded) rememberTitle(tabTitle);
    return keepTitle(live.uiLoaded ? tabTitle : (rememberedTitle() ?? tabTitle));
  }, [tabTitle, live.uiLoaded]);
  return (
    <LiveContext.Provider value={live}>
      <I18nProvider language={live.language}>
        {/* re-check the login whenever the live connection (re)opens */}
        <AuthProvider recheck={live.conn}>{children}</AuthProvider>
      </I18nProvider>
    </LiveContext.Provider>
  );
}

export function useLive(): Live {
  const v = useContext(LiveContext);
  if (!v) throw new Error("useLive must be used inside <LiveProvider>");
  return v;
}
