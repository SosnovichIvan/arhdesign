// @vitest-environment jsdom

import { afterEach, describe, expect, it, vi } from "vitest";

import { createGlobalChat, GlobalChatApiError, loadGlobalChat, loadGlobalChats, markGlobalChatRead, searchGlobalChatCandidates, sendGlobalChatMessage } from "./chats";

function json(body: unknown, status = 200) { return Promise.resolve(new Response(JSON.stringify(body), { headers: { "Content-Type": "application/json" }, status })); }

afterEach(() => { vi.unstubAllGlobals(); vi.restoreAllMocks(); Object.defineProperty(document, "cookie", { configurable: true, value: "" }); });

describe("global chat API", () => {
  it("maps list, search and conversation pagination", async () => {
    const fetchMock = vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      if (url.includes("chat-user-candidates")) return json({ items: [{ userId: "user-1" }] });
      if (url.includes("/chats/chat%2Fid?")) return json({ chat: { id: "chat/id" }, messages: [] });
      return json({ items: [], hasMore: false, nextCursor: null });
    });
    vi.stubGlobal("fetch", fetchMock);
    expect((await loadGlobalChats()).items).toEqual([]);
    expect((await searchGlobalChatCandidates("anna"))[0].userId).toBe("user-1");
    await loadGlobalChat("chat/id", "message-id");
    expect(String(fetchMock.mock.calls.at(-1)?.[0])).toContain("before=message-id");
  });

  it("sends CSRF and idempotency headers for mutations", async () => {
    Object.defineProperty(document, "cookie", { configurable: true, value: "__Host-arhdesign_csrf=hello%20world" });
    vi.stubGlobal("crypto", { ...crypto, randomUUID: () => "request-id" });
    const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      void input; void init;
      return json({ id: "chat-1" }, 201);
    });
    vi.stubGlobal("fetch", fetchMock);
    await createGlobalChat({ kind: "direct", memberUserIds: ["user-1"] });
    await sendGlobalChatMessage("chat/id", "Привет", "message-id");
    await markGlobalChatRead("chat/id");
    const createHeaders = fetchMock.mock.calls[0][1]?.headers as Record<string, string>;
    expect(createHeaders["Idempotency-Key"]).toBe("request-id");
    expect(createHeaders["X-CSRF-Token"]).toBe("hello world");
    expect(String(fetchMock.mock.calls[1][0])).toContain("chat%2Fid/messages");
  });

  it("returns typed API errors including read acknowledgement failures", async () => {
    vi.stubGlobal("fetch", vi.fn(() => json({ error: { message: "Нет доступа" } }, 403)));
    await expect(loadGlobalChats()).rejects.toEqual(expect.objectContaining<Partial<GlobalChatApiError>>({ message: "Нет доступа", status: 403 }));
    await expect(markGlobalChatRead("chat-1")).rejects.toEqual(expect.objectContaining<Partial<GlobalChatApiError>>({ message: "Нет доступа", status: 403 }));
  });
});
