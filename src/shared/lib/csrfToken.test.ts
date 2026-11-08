// @vitest-environment jsdom

import { afterEach, describe, expect, it, vi } from "vitest";

import { csrfToken } from "./csrfToken";

afterEach(() => vi.restoreAllMocks());

describe("csrfToken", () => {
  it("selects the non-secure cookie on a local HTTP origin even when a stale secure cookie exists", () => {
    vi.spyOn(document, "cookie", "get").mockReturnValue("__Host-arhdesign_csrf=stale; arhdesign_csrf=current%20token");
    expect(csrfToken()).toBe("current token");
  });

  it("selects the host cookie for a production HTTPS origin", () => {
    expect(csrfToken("arhdesign_csrf=stale; __Host-arhdesign_csrf=secure%20token", "https:")).toBe("secure token");
  });

  it("returns an empty token for a missing or malformed cookie", () => {
    vi.spyOn(document, "cookie", "get").mockReturnValueOnce("").mockReturnValueOnce("arhdesign_csrf=%E0%A4%A");
    expect(csrfToken()).toBe("");
    expect(csrfToken()).toBe("");
  });
});
