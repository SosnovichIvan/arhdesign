// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import AccountError from "./error";

const reportFrontendError = vi.hoisted(() => vi.fn());
vi.mock("@/features/technicalSupport/api/technicalSupport", () => ({ reportFrontendError }));

afterEach(() => { cleanup(); vi.clearAllMocks(); });

describe("AccountError", () => {
  it("reports a sanitized client failure and retries on demand", async () => {
    reportFrontendError.mockResolvedValue(undefined);
    const reset = vi.fn();
    const error = Object.assign(new Error("render failed"), { digest: "digest-1" });
    render(<AccountError error={error} reset={reset} />);
    expect(screen.getByRole("alert").textContent).toContain("Технический администратор");
    await waitFor(() => expect(reportFrontendError).toHaveBeenCalledWith("render failed", window.location.pathname, "digest-1"));
    fireEvent.click(screen.getByRole("button", { name: "Повторить" }));
    expect(reset).toHaveBeenCalledOnce();
  });

  it("keeps the recovery UI available if error reporting fails", async () => {
    reportFrontendError.mockRejectedValue(new Error("offline"));
    render(<AccountError error={new Error("failure")} reset={vi.fn()} />);
    await waitFor(() => expect(reportFrontendError).toHaveBeenCalled());
    expect(screen.getByRole("button", { name: "Повторить" })).toBeTruthy();
  });
});
