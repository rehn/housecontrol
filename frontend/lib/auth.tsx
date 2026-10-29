"use client";

// The admin login. Anyone may look; switching devices, editing the home page
// and settings need a login when the server has a password (ADMIN_PASSWORD).

import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from "react";
import { getAuth, login as apiLogin, logout as apiLogout, UNAUTHORIZED_EVENT, type AuthStatus } from "./api";

export interface Auth {
  ready: boolean; // the server has answered
  required: boolean;
  loggedIn: boolean;
  username?: string;
  /** may switch devices, edit and see settings */
  canEdit: boolean;
  login: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
}

// Outside a provider (e.g. in component tests) everything is allowed, as before login existed.
export const AuthContext = createContext<Auth>({
  ready: true,
  required: false,
  loggedIn: true,
  canEdit: true,
  login: async () => {},
  logout: async () => {},
});

export const useAuth = () => useContext(AuthContext);

export function AuthProvider({ children, recheck }: { children: ReactNode; recheck?: unknown }) {
  const [status, setStatus] = useState<AuthStatus & { ready: boolean }>({
    ready: false,
    required: true,
    loggedIn: false,
  });

  const refresh = useCallback(() => {
    getAuth()
      .then((s) => setStatus({ ...s, ready: true }))
      .catch(() => setStatus((s) => ({ ...s, ready: true }))); // backend down: stay as we were
  }, []);

  // ask on start, whenever `recheck` changes (e.g. the live connection came back),
  // and when a request was refused because the login ran out
  useEffect(refresh, [refresh, recheck]);
  useEffect(() => {
    window.addEventListener(UNAUTHORIZED_EVENT, refresh);
    return () => window.removeEventListener(UNAUTHORIZED_EVENT, refresh);
  }, [refresh]);

  const login = useCallback(async (username: string, password: string) => {
    const s = await apiLogin(username, password);
    setStatus({ ...s, ready: true });
  }, []);
  const logout = useCallback(async () => {
    const s = await apiLogout();
    setStatus({ ...s, ready: true });
  }, []);

  const value: Auth = {
    ...status,
    username: status.loggedIn ? status.username : undefined,
    canEdit: status.ready && (!status.required || status.loggedIn),
    login,
    logout,
  };
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}
