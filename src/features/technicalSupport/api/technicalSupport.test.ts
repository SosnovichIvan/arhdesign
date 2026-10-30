// @vitest-environment jsdom

import { afterEach, describe, expect, it, vi } from "vitest";

import { reportFrontendError, safePath, sendTechnicalFeedback } from "./technicalSupport";

afterEach(() => { vi.unstubAllGlobals(); vi.restoreAllMocks(); Object.defineProperty(document, "cookie", { configurable: true, value: "" }); });

describe("technical support API", () => {
  it("normalizes paths and sanitizes frontend diagnostics", async () => {
    Object.defineProperty(document, "cookie", { configurable: true, value: "arhdesign_csrf=csrf-value" });
    const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      void input; void init;
      return Promise.resolve(new Response(null, { status: 202 }));
    });
    vi.stubGlobal("fetch", fetchMock);
    await reportFrontendError("Failure https://example.test/a test@example.test bearer secret-token", "/account/chats?secret=yes", "bad digest !");
    const init = fetchMock.mock.calls[0]?.[1];
    const payload = JSON.parse(String(init?.body));
    expect(payload.path).toBe("/account/chats");
    expect(payload.message).toBe("Failure [url] [email] [credential]");
    expect(payload.digest).toBe("bad-digest--");
    expect((init?.headers as Record<string, string>)["X-CSRF-Token"]).toBe("csrf-value");
  });

  it("falls back for external and malformed paths and unknown messages", async () => {
    expect(safePath("https://external.example/a")).toBe("/account");
    expect(safePath("http://[")).toBe("/account");
    expect(safePath("/")).toBe("/");
    const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      void input; void init;
      return Promise.resolve(new Response(null, { status: 202 }));
    });
    vi.stubGlobal("fetch", fetchMock);
    await reportFrontendError("", "/account");
    expect(JSON.parse(String(fetchMock.mock.calls[0]?.[1]?.body)).message).toBe("Неизвестная ошибка интерфейса");
  });

  it("reports feedback failures with the backend message", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: { message: "Лимит обращений исчерпан" } }), { headers: { "Content-Type": "application/json" }, status: 429 })));
    await expect(sendTechnicalFeedback("suggestion", "Добавьте фильтр", "/account")).rejects.toThrow("Лимит обращений исчерпан");
  });
});
