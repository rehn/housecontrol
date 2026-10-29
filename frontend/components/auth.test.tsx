import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Device } from "@/lib/api";
import { AuthContext, AuthProvider, useAuth, type Auth } from "@/lib/auth";
import { I18nProvider } from "@/lib/i18n";
import { AuthButton, LoginForm } from "./LoginForm";
import { DeviceTile } from "./tiles/DeviceTile";
import { GroupTile } from "./tiles/GroupTile";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

type Reply = { status: number; body: unknown };

/** Fake backend: answers per "METHOD path", records the calls. */
function stubBackend(routes: Record<string, Reply | (() => Reply)>) {
  const fetch = vi.fn((url: string, init?: RequestInit) => {
    const path = new URL(url, "http://x").pathname;
    const route = routes[`${init?.method ?? "GET"} ${path}`];
    const r = typeof route === "function" ? route() : (route ?? { status: 404, body: {} });
    return Promise.resolve(new Response(JSON.stringify(r.body), { status: r.status }));
  });
  vi.stubGlobal("fetch", fetch);
  return fetch;
}

function Probe() {
  const a = useAuth();
  return (
    <p data-testid="auth">
      {a.ready ? "ready" : "loading"} {a.canEdit ? "can-edit" : "read-only"} {a.username ?? "-"}
    </p>
  );
}
const probe = () => screen.getByTestId("auth").textContent;

describe("AuthProvider", () => {
  it("is read-only until logged in, then can edit, and read-only again after logout", async () => {
    let loggedIn = false;
    const fetch = stubBackend({
      "GET /api/auth": () => ({ status: 200, body: { required: true, loggedIn } }),
      "POST /api/auth/login": () => {
        loggedIn = true;
        return { status: 200, body: { required: true, loggedIn: true, username: "admin" } };
      },
      "POST /api/auth/logout": () => {
        loggedIn = false;
        return { status: 200, body: { required: true, loggedIn: false } };
      },
    });
    let auth: Auth | null = null;
    const Grab = () => {
      auth = useAuth();
      return null;
    };
    render(
      <AuthProvider>
        <Probe />
        <Grab />
      </AuthProvider>,
    );
    await vi.waitFor(() => expect(probe()).toBe("ready read-only -"));

    await act(() => auth!.login("admin", "hemligt"));
    expect(probe()).toBe("ready can-edit admin");
    const [, init] = fetch.mock.calls.find(([u]) => String(u).endsWith("/api/auth/login"))!;
    expect(init?.credentials).toBe("include"); // the cookie must travel in dev (:3000 -> :8080)
    expect(JSON.parse(String(init?.body))).toEqual({ username: "admin", password: "hemligt" });

    await act(() => auth!.logout());
    expect(probe()).toBe("ready read-only -");
  });

  it("can edit straight away when the server has no password", async () => {
    stubBackend({ "GET /api/auth": { status: 200, body: { required: false, loggedIn: true } } });
    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>,
    );
    await vi.waitFor(() => expect(probe()).toBe("ready can-edit -"));
  });

  it("notices an expired login when the server says 401", async () => {
    let loggedIn = true;
    stubBackend({
      "GET /api/auth": () => ({ status: 200, body: { required: true, loggedIn, username: "admin" } }),
      "PUT /api/settings/ui": { status: 401, body: { error: "log in", code: "unauthorized" } },
    });
    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>,
    );
    await vi.waitFor(() => expect(probe()).toBe("ready can-edit admin"));
    loggedIn = false;
    const { saveUISettings } = await import("@/lib/api");
    await saveUISettings({ title: "x" }).catch(() => {});
    await vi.waitFor(() => expect(probe()).toBe("ready read-only -"));
  });
});

describe("LoginForm", () => {
  it("logs in with what was typed", async () => {
    const login = vi.fn(() => Promise.resolve());
    const done = vi.fn();
    render(
      <AuthContext.Provider value={{ ready: true, required: true, loggedIn: false, canEdit: false, login, logout: vi.fn() }}>
        <LoginForm onDone={done} />
      </AuthContext.Provider>,
    );
    fireEvent.change(screen.getByLabelText("User name"), { target: { value: "admin" } });
    fireEvent.change(screen.getByLabelText("Password"), { target: { value: "hemligt" } });
    fireEvent.click(screen.getByRole("button", { name: "Log in" }));
    await vi.waitFor(() => expect(done).toHaveBeenCalled());
    expect(login).toHaveBeenCalledWith("admin", "hemligt");
    expect((screen.getByLabelText("Password") as HTMLInputElement).type).toBe("password");
  });

  it("says when the password is wrong, in Swedish too", async () => {
    stubBackend({
      "GET /api/auth": { status: 200, body: { required: true, loggedIn: false } },
      "POST /api/auth/login": { status: 401, body: { error: "wrong", code: "bad_login" } },
    });
    render(
      <I18nProvider language="sv">
        <AuthProvider>
          <LoginForm />
        </AuthProvider>
      </I18nProvider>,
    );
    fireEvent.change(screen.getByLabelText("Användarnamn"), { target: { value: "admin" } });
    fireEvent.change(screen.getByLabelText("Lösenord"), { target: { value: "fel" } });
    fireEvent.click(screen.getByRole("button", { name: "Logga in" }));
    expect(await screen.findByText("Fel användarnamn eller lösenord.")).toBeTruthy();
    expect((screen.getByLabelText("Lösenord") as HTMLInputElement).value).toBe(""); // cleared for a new try
  });

  it("asks to wait after too many wrong passwords", async () => {
    stubBackend({
      "GET /api/auth": { status: 200, body: { required: true, loggedIn: false } },
      "POST /api/auth/login": { status: 429, body: { error: "slow down", code: "too_many_attempts" } },
    });
    render(
      <AuthProvider>
        <LoginForm />
      </AuthProvider>,
    );
    fireEvent.change(screen.getByLabelText("User name"), { target: { value: "admin" } });
    fireEvent.change(screen.getByLabelText("Password"), { target: { value: "x" } });
    fireEvent.click(screen.getByRole("button", { name: "Log in" }));
    expect(await screen.findByText(/Too many wrong attempts/)).toBeTruthy();
  });
});

describe("tiles without login", () => {
  const loggedOut: Auth = { ready: true, required: true, loggedIn: false, canEdit: false, login: vi.fn(), logout: vi.fn() };
  const lamp: Device = {
    id: "lamp",
    name: "Taklampa",
    kind: "light",
    availability: "online",
    state: { state: "ON", brightness: 200 },
    capabilities: { onOff: true, brightness: true, brightnessMax: 254 },
  } as unknown as Device;
  const card = { id: "c1", type: "device" as const, deviceId: "lamp" };

  it("can still be switched and opened (controlling needs no login)", () => {
    const onPatch = vi.fn();
    vi.stubGlobal("fetch", vi.fn(() => Promise.resolve(new Response("{}", { status: 202 }))));
    render(
      <AuthContext.Provider value={loggedOut}>
        <DeviceTile card={card} device={lamp} onPatch={onPatch} onError={vi.fn()} />
      </AuthContext.Provider>,
    );
    const toggle = screen.getByRole("button", { name: "Turn Taklampa off" }) as HTMLButtonElement;
    expect(toggle.disabled).toBe(false);
    fireEvent.click(toggle);
    expect(onPatch).toHaveBeenCalledWith("lamp", { state: "OFF" });
    fireEvent.contextMenu(toggle.closest(".tile")!); // right-click = long press: brightness panel
    expect(screen.getByRole("dialog")).toBeTruthy();
  });

  it("group tiles too", () => {
    render(
      <AuthContext.Provider value={loggedOut}>
        <GroupTile
          card={{ id: "g", type: "group", title: "Kök", deviceIds: ["lamp"] }}
          devices={{ lamp }}
          onPatch={vi.fn()}
          onError={vi.fn()}
        />
      </AuthContext.Provider>,
    );
    expect((screen.getByRole("button", { name: "Turn Kök off" }) as HTMLButtonElement).disabled).toBe(false);
  });
});

describe("AuthButton (home page menu)", () => {
  const auth = (over: Partial<Auth>): Auth => ({
    ready: true, required: true, loggedIn: false, canEdit: false, login: vi.fn(), logout: vi.fn(() => Promise.resolve()), ...over,
  });

  it("offers Log out when logged in", async () => {
    const a = auth({ loggedIn: true, canEdit: true, username: "admin" });
    render(
      <AuthContext.Provider value={a}>
        <AuthButton />
      </AuthContext.Provider>,
    );
    expect(screen.queryByRole("button", { name: "Log in" })).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Log out" }));
    await vi.waitFor(() => expect(a.logout).toHaveBeenCalled());
  });

  it("offers Log in when logged out, and opens the login dialog", () => {
    render(
      <AuthContext.Provider value={auth({})}>
        <AuthButton />
      </AuthContext.Provider>,
    );
    fireEvent.click(screen.getByRole("button", { name: "Log in" }));
    expect(screen.getByRole("dialog", { name: "Log in" })).toBeTruthy();
  });

  it("shows nothing when the server has no password", () => {
    const { container } = render(
      <AuthContext.Provider value={auth({ required: false, loggedIn: true, canEdit: true })}>
        <AuthButton />
      </AuthContext.Provider>,
    );
    expect(container.textContent).toBe("");
  });

  it("is in Swedish", () => {
    render(
      <I18nProvider language="sv">
        <AuthContext.Provider value={auth({ loggedIn: true, canEdit: true })}>
          <AuthButton />
        </AuthContext.Provider>
      </I18nProvider>,
    );
    expect(screen.getByRole("button", { name: "Logga ut" })).toBeTruthy();
  });
});
