// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ThemeToggle } from "../ui/themeToggle";
import { ThemePreferenceProvider, useThemePreference } from "./themePreferenceProvider";

afterEach(() => {
  cleanup();
  document.documentElement.dataset.theme = "light";
  vi.unstubAllGlobals();
});

function response(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { headers: { "Content-Type": "application/json" }, status });
}

describe("ThemePreferenceProvider", () => {
  it("restores the authenticated user's theme and saves a change", async () => {
    Object.defineProperty(document, "cookie", { configurable: true, value: "arhdesign_csrf=csrf-token" });
    const fetchMock = vi.fn((input: string | URL | Request, init?: RequestInit) => {
      if (init?.method === "PUT") return Promise.resolve(response({ theme: "light" }));
      return Promise.resolve(response({ theme: "dark" }));
    });
    vi.stubGlobal("fetch", fetchMock);

    render(<ThemePreferenceProvider><ThemeToggle /></ThemePreferenceProvider>);
    await waitFor(() => expect(document.documentElement.dataset.theme).toBe("dark"));
    fireEvent.click(screen.getByRole("button", { name: "Включить светлую тему" }));
    await waitFor(() => expect(document.documentElement.dataset.theme).toBe("light"));

    const update = fetchMock.mock.calls.find(([, init]) => init?.method === "PUT");
    expect(String(update?.[0])).toBe("/api/v1/settings");
    expect(update?.[1]).toMatchObject({ body: JSON.stringify({ theme: "light" }), credentials: "same-origin", method: "PUT" });
    expect((update?.[1]?.headers as Record<string, string>)["X-CSRF-Token"]).toBe("csrf-token");
  });

  it("keeps a public theme local and reloads settings after login", async () => {
    let authenticated = false;
    const fetchMock = vi.fn((input: string | URL | Request, init?: RequestInit) => {
      void input; void init;
      return Promise.resolve(authenticated ? response({ theme: "dark" }) : response({ error: {} }, 401));
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<ThemePreferenceProvider><ThemeToggle /></ThemePreferenceProvider>);
    await waitFor(() => expect(fetchMock).toHaveBeenCalled());

    fireEvent.click(screen.getByRole("button", { name: "Включить тёмную тему" }));
    expect(document.documentElement.dataset.theme).toBe("dark");
    expect(fetchMock.mock.calls.every(([, init]) => init?.method !== "PUT")).toBe(true);

    authenticated = true;
    window.dispatchEvent(new CustomEvent("arhdesign:session-changed", { detail: { authenticated: true } }));
    await waitFor(() => expect(fetchMock.mock.calls.length).toBeGreaterThan(1));
    expect(document.documentElement.dataset.theme).toBe("dark");

    window.dispatchEvent(new CustomEvent("arhdesign:session-changed", { detail: { authenticated: false } }));
    expect(document.documentElement.dataset.theme).toBe("light");
  });

  it("keeps the local default when loading authenticated settings fails", async () => {
    vi.stubGlobal("fetch", vi.fn(() => Promise.reject(new Error("network unavailable"))));

    render(<ThemePreferenceProvider><ThemeToggle /></ThemePreferenceProvider>);

    await waitFor(() => expect(screen.getByRole("button", { name: "Включить тёмную тему" })).toBeTruthy());
    fireEvent.click(screen.getByRole("button", { name: "Включить тёмную тему" }));
    expect(document.documentElement.dataset.theme).toBe("dark");
  });

  it("ignores an aborted settings request", async () => {
    vi.stubGlobal("fetch", vi.fn(() => Promise.reject(new DOMException("aborted", "AbortError"))));

    render(<ThemePreferenceProvider><ThemeToggle /></ThemePreferenceProvider>);

    await waitFor(() => expect(screen.getByRole("button", { name: "Включить тёмную тему" })).toBeTruthy());
    window.dispatchEvent(new Event("arhdesign:session-changed"));
    expect(document.documentElement.dataset.theme).toBe("light");
  });

  it("rolls back the optimistic theme when persistence fails", async () => {
    const fetchMock = vi.fn((_: string | URL | Request, init?: RequestInit) => Promise.resolve(init?.method === "PUT" ? response({ error: { message: "Ошибка сохранения" } }, 503) : response({ theme: "light" })));
    vi.stubGlobal("fetch", fetchMock);
    render(<ThemePreferenceProvider><ThemeToggle /></ThemePreferenceProvider>);
    await waitFor(() => expect(fetchMock).toHaveBeenCalled());
    fireEvent.click(screen.getByRole("button", { name: "Включить тёмную тему" }));
    await waitFor(() => expect(document.documentElement.dataset.theme).toBe("light"));
  });

  it("rejects theme context usage outside its provider", () => {
    function Consumer() {
      useThemePreference();
      return null;
    }

    expect(() => render(<Consumer />)).toThrow("useThemePreference must be used inside ThemePreferenceProvider");
  });
});
