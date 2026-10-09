// @vitest-environment jsdom

import { afterEach, describe, expect, it, vi } from "vitest";

import { loadThemePreference, saveThemePreference } from "./themePreferences";

afterEach(() => {
  vi.unstubAllGlobals();
});

function response(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { headers: { "Content-Type": "application/json" }, status });
}

describe("theme preferences API", () => {
  it("treats an unauthenticated settings response as a public session", async () => {
    vi.stubGlobal("fetch", vi.fn(() => Promise.resolve(response({ error: {} }, 401))));

    await expect(loadThemePreference()).resolves.toBeNull();
  });

  it("reports loading failures and rejects an invalid server theme", async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(response({ error: { message: "Настройки недоступны" } }, 503))
      .mockResolvedValueOnce(response({ theme: "system" }));
    vi.stubGlobal("fetch", fetchMock);

    await expect(loadThemePreference()).rejects.toThrow("Настройки недоступны");
    await expect(loadThemePreference()).rejects.toThrow("Сервис вернул некорректную тему");
  });

  it("reports save failures and rejects an invalid saved theme", async () => {
    Object.defineProperty(document, "cookie", { configurable: true, value: "" });
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(response({ error: { message: "Сохранение недоступно" } }, 503))
      .mockResolvedValueOnce(response({ theme: null }));
    vi.stubGlobal("fetch", fetchMock);

    await expect(saveThemePreference("dark")).rejects.toThrow("Сохранение недоступно");
    await expect(saveThemePreference("dark")).rejects.toThrow("Сервис вернул некорректную тему");
    expect((fetchMock.mock.calls[0]?.[1]?.headers as Record<string, string>)["X-CSRF-Token"]).toBe("");
  });

  it("reads and decodes the secure CSRF cookie", async () => {
    Object.defineProperty(document, "cookie", { configurable: true, value: "other=1; __Host-arhdesign_csrf=secure%20token" });
    const fetchMock = vi.fn((input: string | URL | Request, init?: RequestInit) => {
      void input;
      void init;
      return Promise.resolve(response({ theme: "dark" }));
    });
    vi.stubGlobal("fetch", fetchMock);

    await expect(saveThemePreference("dark")).resolves.toBe("dark");
    expect((fetchMock.mock.calls[0]?.[1]?.headers as Record<string, string>)["X-CSRF-Token"]).toBe("secure token");
  });
});
