// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ContextChatAction } from "./contextChatAction";

const push = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ push }) }));

afterEach(() => { cleanup(); push.mockReset(); vi.unstubAllGlobals(); });
function response(body: unknown, status = 200) { return Promise.resolve(new Response(JSON.stringify(body), { headers: { "Content-Type": "application/json" }, status })); }

describe("ContextChatAction", () => {
  it("opens an existing contextual chat without showing the creation form", () => {
    render(<ContextChatAction chatId="chat-1" contextId="task-1" contextTitle="Рабочие чертежи" contextType="task" projectId="project-1" />);
    fireEvent.click(screen.getByRole("button", { name: "Обсудить «Рабочие чертежи»" }));
    expect(push).toHaveBeenCalledWith("/account/projects/project-1/chat/chat-1");
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("validates a name, creates a chat and navigates to it", async () => {
    Object.defineProperty(document, "cookie", { configurable: true, value: "arhdesign_csrf=csrf-token" });
    const fetchMock = vi.fn((_input: string | URL | Request, _options?: RequestInit) => { void _input; void _options; return response({ chatId: "chat-2", contextId: "task-1", contextTitle: "Рабочие чертежи", contextType: "task", name: "Планы кухни", projectId: "project-1" }); });
    vi.stubGlobal("fetch", fetchMock);
    render(<ContextChatAction chatId={null} contextId="task-1" contextTitle="Рабочие чертежи" contextType="task" projectId="project-1" />);
    fireEvent.click(screen.getByRole("button", { name: "Обсудить «Рабочие чертежи»" }));
    const dialog = screen.getByRole("dialog", { name: "Создание обсуждения «Рабочие чертежи»" });
    const input = within(dialog).getByLabelText("Название обсуждения *");
    fireEvent.change(input, { target: { value: " " } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Создать обсуждение" }));
    expect(within(dialog).getByRole("alert").textContent).toContain("минимум 2");
    fireEvent.change(input, { target: { value: "Планы кухни" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Создать обсуждение" }));
    await waitFor(() => expect(push).toHaveBeenCalledWith("/account/projects/project-1/chat/chat-2"));
    const [, options] = fetchMock.mock.calls[0];
    expect(options?.method).toBe("PUT");
    expect(options?.headers).toMatchObject({ "X-CSRF-Token": "csrf-token" });
    expect(JSON.parse(String(options?.body))).toEqual({ contextId: "task-1", contextType: "task", name: "Планы кухни" });
  });

  it("keeps the form open after a recoverable server error", async () => {
    vi.stubGlobal("fetch", vi.fn(() => response({ message: "Недостаточно прав" }, 404)));
    render(<ContextChatAction chatId={null} contextId="material-1" contextTitle="Диван" contextType="material" projectId="project-1" />);
    fireEvent.click(screen.getByRole("button", { name: "Обсудить «Диван»" }));
    fireEvent.click(screen.getByRole("button", { name: "Создать обсуждение" }));
    expect((await screen.findByRole("alert")).textContent).toContain("Недостаточно прав");
    expect(screen.getByRole("dialog")).toBeTruthy();
    expect(push).not.toHaveBeenCalled();
  });
});
