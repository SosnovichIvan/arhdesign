// @vitest-environment jsdom

import { afterEach, describe, expect, it, vi } from "vitest";

import { getWithAccountSession } from "./accountSession";

afterEach(() => {
  document.cookie = "arhdesign_csrf=; Max-Age=0; Path=/";
  Reflect.deleteProperty(navigator, "locks");
  vi.unstubAllGlobals();
});

const ok = () => new Response(null, { status: 200 });
const unauthorized = () => new Response(null, { status: 401 });

describe("account session recovery", () => {
  it("returns the first response for a healthy or signed-out session", async () => {
    const fetchMock = vi.fn().mockResolvedValueOnce(ok()).mockResolvedValueOnce(unauthorized());
    vi.stubGlobal("fetch", fetchMock);
    expect((await getWithAccountSession("/api/v1/me")).status).toBe(200);
    expect((await getWithAccountSession("/api/v1/me")).status).toBe(401);
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it("rotates an expired session once across concurrent requests", async () => {
    document.cookie = "arhdesign_csrf=refresh-csrf; Path=/";
    const fetchMock = vi.fn().mockResolvedValueOnce(unauthorized()).mockResolvedValueOnce(unauthorized()).mockResolvedValue(ok());
    vi.stubGlobal("fetch", fetchMock);
    const responses = await Promise.all([getWithAccountSession("/api/v1/me"), getWithAccountSession("/api/v1/release-notes")]);
    expect(responses.map((response) => response.status)).toEqual([200, 200]);
    expect(fetchMock.mock.calls.filter(([url]) => url.includes("/auth/refresh"))).toEqual([["/api/v1/auth/refresh", expect.objectContaining({ headers: { "X-CSRF-Token": "refresh-csrf" }, method: "POST" })]]);
  });

  it("rechecks access inside the cross-tab lock and keeps a refreshed session", async () => {
    document.cookie = "arhdesign_csrf=refresh-csrf; Path=/";
    const request = vi.fn((_name: string, callback: () => Promise<boolean>) => callback());
    Object.defineProperty(navigator, "locks", { configurable: true, value: { request } });
    const fetchMock = vi.fn().mockResolvedValueOnce(unauthorized()).mockResolvedValue(ok());
    vi.stubGlobal("fetch", fetchMock);
    expect((await getWithAccountSession("/api/v1/release-notes")).status).toBe(200);
    expect(request).toHaveBeenCalledWith("arhdesign:refresh-session", expect.any(Function));
    expect(fetchMock.mock.calls.map(([url]) => url)).toEqual(["/api/v1/release-notes", "/api/v1/me", "/api/v1/release-notes"]);
  });

  it("keeps the unauthorized response when refresh is denied", async () => {
    document.cookie = "arhdesign_csrf=refresh-csrf; Path=/";
    Object.defineProperty(navigator, "locks", { configurable: true, value: { request: (_name: string, callback: () => Promise<boolean>) => callback() } });
    const fetchMock = vi.fn().mockResolvedValue(unauthorized());
    vi.stubGlobal("fetch", fetchMock);
    expect((await getWithAccountSession("/api/v1/me")).status).toBe(401);
    expect(fetchMock).toHaveBeenCalledTimes(3);
  });

  it("skips refresh for requests that were cancelled after the first response", async () => {
    document.cookie = "arhdesign_csrf=refresh-csrf; Path=/";
    const controller = new AbortController();
    const fetchMock = vi.fn(() => { controller.abort(); return Promise.resolve(unauthorized()); });
    vi.stubGlobal("fetch", fetchMock);
    await expect(getWithAccountSession("/api/v1/me", controller.signal)).rejects.toMatchObject({ name: "AbortError" });
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("does not rotate after another tab logs out while waiting for the lock", async () => {
    document.cookie = "arhdesign_csrf=refresh-csrf; Path=/";
    Object.defineProperty(navigator, "locks", { configurable: true, value: { request: (_name: string, callback: () => Promise<boolean>) => {
      document.cookie = "arhdesign_csrf=; Max-Age=0; Path=/";
      return callback();
    } } });
    const fetchMock = vi.fn().mockResolvedValue(unauthorized());
    vi.stubGlobal("fetch", fetchMock);
    expect((await getWithAccountSession("/api/v1/me", new AbortController().signal)).status).toBe(401);
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });
});
