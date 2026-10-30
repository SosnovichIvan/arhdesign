// @vitest-environment jsdom

import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import ResetPasswordPage from "./reset-password/page";
import VerifyEmailPage from "./verify-email/page";

afterEach(cleanup);

describe("auth link bridge pages", () => {
  it("renders a reset handoff status while preserving the fragment", () => {
    window.history.replaceState({}, "", "/reset-password#token=reset-token");
    render(<ResetPasswordPage />);
    expect(screen.getByRole("status").textContent).toContain("восстановления");
  });

  it("renders a verification handoff status while preserving the fragment", () => {
    window.history.replaceState({}, "", "/verify-email#token=verify-token");
    render(<VerifyEmailPage />);
    expect(screen.getByRole("status").textContent).toContain("Проверяем ссылку");
  });
});
