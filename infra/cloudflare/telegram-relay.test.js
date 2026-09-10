import { afterEach, describe, expect, it, vi } from "vitest";

import worker from "./telegram-relay.js";

const env = { RELAY_SECRET: "relay-secret", TELEGRAM_BOT_TOKEN: "bot-token" };

afterEach(() => vi.unstubAllGlobals());

describe("Telegram relay Worker", () => {
  it("exposes a health endpoint without secrets", async () => {
    const response = await worker.fetch(new Request("https://relay.example/healthz"), env);

    expect(response.status).toBe(200);
    await expect(response.json()).resolves.toEqual({ status: "ok" });
  });

  it("rejects unauthenticated delivery requests", async () => {
    const telegramFetch = vi.fn();
    vi.stubGlobal("fetch", telegramFetch);

    const response = await worker.fetch(new Request("https://relay.example/sendMessage", { method: "POST" }), env);

    expect(response.status).toBe(401);
    expect(telegramFetch).not.toHaveBeenCalled();
  });

  it("forwards only the validated notification fields", async () => {
    const telegramFetch = vi.fn().mockResolvedValue(new Response("{}", { status: 200 }));
    vi.stubGlobal("fetch", telegramFetch);
    const request = new Request("https://relay.example/sendMessage", {
      body: JSON.stringify({ chat_id: 123, ignored: "value", text: "Новая заявка" }),
      headers: { Authorization: "Bearer relay-secret", "Content-Type": "application/json" },
      method: "POST",
    });

    const response = await worker.fetch(request, env);

    expect(response.status).toBe(200);
    expect(telegramFetch).toHaveBeenCalledOnce();
    const [url, options] = telegramFetch.mock.calls[0];
    expect(url).toBe("https://api.telegram.org/botbot-token/sendMessage");
    expect(JSON.parse(options.body)).toEqual({ chat_id: 123, text: "Новая заявка" });
  });
});
