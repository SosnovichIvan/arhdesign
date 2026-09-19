const worker = {
  async fetch(request, env) {
    const url = new URL(request.url);
    const headers = { "Cache-Control": "no-store", "Content-Type": "application/json; charset=utf-8" };

    if (request.method === "GET" && url.pathname === "/healthz") {
      return new Response(JSON.stringify({ status: "ok" }), { status: 200, headers });
    }
    if (url.pathname !== "/sendMessage" && url.pathname !== "/deleteMessage") {
      return new Response(JSON.stringify({ error: "not_found" }), { status: 404, headers });
    }
    if (request.method !== "POST") {
      return new Response(JSON.stringify({ error: "method_not_allowed" }), { status: 405, headers });
    }

    const expectedAuthorization = env.RELAY_SECRET ? `Bearer ${env.RELAY_SECRET}` : "";
    if (!expectedAuthorization || request.headers.get("Authorization") !== expectedAuthorization) {
      return new Response(JSON.stringify({ error: "unauthorized" }), { status: 401, headers });
    }

    const contentLength = Number(request.headers.get("Content-Length") || "0");
    if (contentLength > 16384) {
      return new Response(JSON.stringify({ error: "payload_too_large" }), { status: 413, headers });
    }

    let payload;
    try {
      payload = await request.json();
    } catch {
      return new Response(JSON.stringify({ error: "invalid_json" }), { status: 400, headers });
    }

    const chatID = payload && payload.chat_id;
    const text = payload && payload.text;
    const messageID = payload && payload.message_id;
    const validChatID = typeof chatID === "string" || typeof chatID === "number";
    const isSend = url.pathname === "/sendMessage";
    const validPayload = isSend
      ? validChatID && typeof text === "string" && text.length >= 1 && text.length <= 4096
      : validChatID && Number.isInteger(messageID) && messageID > 0;
    if (!validPayload) {
      return new Response(JSON.stringify({ error: "invalid_payload" }), { status: 400, headers });
    }
    if (!env.TELEGRAM_BOT_TOKEN) {
      return new Response(JSON.stringify({ error: "relay_not_configured" }), { status: 503, headers });
    }

    try {
      const telegramResponse = await fetch(`https://api.telegram.org/bot${env.TELEGRAM_BOT_TOKEN}${url.pathname}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(isSend ? { chat_id: chatID, text } : { chat_id: chatID, message_id: messageID }),
      });
      if (!telegramResponse.ok) {
        return new Response(JSON.stringify({ error: "telegram_rejected_request" }), { status: 502, headers });
      }
      const telegramPayload = await telegramResponse.json();
      return new Response(JSON.stringify(telegramPayload), { status: 200, headers });
    } catch {
      return new Response(JSON.stringify({ error: "telegram_unavailable" }), { status: 502, headers });
    }
  },
};

export default worker;
