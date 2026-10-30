// @vitest-environment jsdom

import { describe, expect, it } from "vitest";

import { readApiError, readApiErrorDetails } from "./readApiError";

describe("readApiError", () => {
  it("reads the canonical nested API error", async () => {
    const response = new Response(JSON.stringify({ error: { code: "identifier_unavailable", message: " Логин или почта недоступны " } }), { status: 409 });
    await expect(readApiError(response, "Ошибка")).resolves.toBe("Логин или почта недоступны");
  });

  it("reads safe field-level errors from the canonical envelope", async () => {
    const response = new Response(JSON.stringify({ error: { message: "Исправьте выделенные поля", fields: { login: "Некорректный логин", password: " ", ignored: 42 } } }), { status: 400 });
    await expect(readApiErrorDetails(response, "fallback")).resolves.toEqual({
      code: "",
      fields: { login: "Некорректный логин" },
      message: "Исправьте выделенные поля",
    });
  });

  it("supports a legacy top-level message", async () => {
    const response = new Response(JSON.stringify({ message: "Попробуйте позже" }), { status: 503 });
    await expect(readApiError(response, "Ошибка")).resolves.toBe("Попробуйте позже");
  });

  it("uses the fallback for malformed and empty payloads", async () => {
    await expect(readApiError(new Response("not-json", { status: 500 }), "Ошибка" )).resolves.toBe("Ошибка");
    await expect(readApiError(new Response(JSON.stringify({ error: { message: " " } }), { status: 500 }), "Ошибка")).resolves.toBe("Ошибка");
    await expect(readApiError(new Response(JSON.stringify([]), { status: 500 }), "Ошибка")).resolves.toBe("Ошибка");
  });
});
