// @vitest-environment jsdom

import { afterEach, describe, expect, it, vi } from "vitest";

import * as projects from "./projects";

afterEach(() => { vi.unstubAllGlobals(); Object.defineProperty(document, "cookie", { configurable: true, value: "" }); });

function calls() {
  const file = new File(["plan"], "План.pdf", { type: "application/pdf" });
  const payload = {} as never;
  return [
    () => projects.loadProjects(),
    () => projects.createProject(payload),
    () => projects.loadProject("project/1"),
    () => projects.loadProjectMembers("project/1"),
    () => projects.searchProjectMemberCandidates("project/1", "anna@example.com"),
    () => projects.addProjectMember("project/1", "user/1"),
    () => projects.removeProjectMember("project/1", "user/1"),
    () => projects.loadProjectFinanceSummary("project/1"),
    () => projects.loadProjectExpenses("project/1"),
    () => projects.createProjectExpense("project/1", payload, "expense-key"),
    () => projects.loadProjectMaterials("project/1"),
    () => projects.createProjectMaterial("project/1", payload),
    () => projects.updateProjectMaterial("project/1", "material/1", 2, payload),
    () => projects.deleteProjectMaterial("project/1", "material/1", 2, "duplicate"),
    () => projects.loadProjectChat("project/1"),
    () => projects.loadProjectChat("project/1", "message/1"),
    () => projects.loadProjectChats("project/1"),
    () => projects.createProjectChat("project/1", "Смета", ["user/1"]),
    () => projects.updateProjectChat("project/1", "chat/1", "Новое имя", 3),
    () => projects.deleteProjectChat("project/1", "chat/1", 3),
    () => projects.markProjectChatRead("project/1", "chat/1", "message/1"),
    () => projects.loadProjectChatMembers("project/1", "chat/1"),
    () => projects.addProjectChatMember("project/1", "chat/1", "user/1"),
    () => projects.removeProjectChatMember("project/1", "chat/1", "user/1"),
    () => projects.loadProjectDocuments("project/1"),
    () => projects.uploadProjectDocument("project/1", file),
    () => projects.deleteProjectDocument("project/1", "document/1"),
    () => projects.sendProjectChatMessage("project/1", "Сообщение", "client-1"),
    () => projects.openProjectContextChat("project/1", "task", "task/1", "Задача"),
    () => projects.loadProjectContextChat("project/1", "chat/1"),
    () => projects.loadProjectContextChat("project/1", "chat/1", "message/1"),
    () => projects.sendProjectContextChatMessage("project/1", "chat/1", "Сообщение", "client-2"),
    () => projects.loadCurrentAccount(),
    () => projects.updateProject("project/1", 4, payload),
    () => projects.archiveProject("project/1", 4),
    () => projects.loadProjectUpcoming("project/1", 14),
    () => projects.loadProjectCalendar("project/1", "2026-10-01", "2026-11-01"),
    () => projects.loadProjectUserSettings("project/1"),
    () => projects.updateProjectUserSettings("project/1", { upcomingDays: 14 }),
    () => projects.createProjectTask("project/1", payload),
    () => projects.loadProjectTasks("project/1"),
    () => projects.updateProjectTaskStatus("project/1", "task/1", 5, "review"),
    () => projects.createProjectMeeting("project/1", payload),
  ];
}

describe("project API", () => {
  it("covers every public request contract with encoded IDs and CSRF", async () => {
    Object.defineProperty(document, "cookie", { configurable: true, value: "arhdesign_csrf=csrf%20token" });
    const fetchMock = vi.fn().mockImplementation(() => Promise.resolve(new Response(JSON.stringify({}), { status: 200 })));
    vi.stubGlobal("fetch", fetchMock);
    for (const call of calls()) await call();
    expect(fetchMock).toHaveBeenCalledTimes(calls().length);
    expect(fetchMock.mock.calls.some(([url]) => String(url).includes("project%2F1"))).toBe(true);
    expect(fetchMock.mock.calls.some(([, init]) => (init?.headers as Record<string, string> | undefined)?.["X-CSRF-Token"] === "csrf token")).toBe(true);
  });

  it("maps every failed request to ProjectApiError", async () => {
    vi.stubGlobal("fetch", vi.fn().mockImplementation(() => Promise.resolve(new Response(JSON.stringify({ error: { message: "API недоступно" } }), { status: 503 }))));
    for (const call of calls()) await expect(call()).rejects.toMatchObject<Partial<projects.ProjectApiError>>({ message: "API недоступно", status: 503 });
  });

  it("uses conflict-specific messages when the server omits a safe message", async () => {
    vi.stubGlobal("fetch", vi.fn().mockImplementation(() => Promise.resolve(new Response("not-json", { status: 409 }))));
    const payload = {} as never;
    const conflictCalls = [
      () => projects.createProjectExpense("p", payload, "key"),
      () => projects.updateProjectMaterial("p", "m", 1, payload),
      () => projects.deleteProjectMaterial("p", "m", 1, "reason"),
      () => projects.deleteProjectChat("p", "c", 1),
      () => projects.sendProjectChatMessage("p", "body", "id"),
      () => projects.sendProjectContextChatMessage("p", "c", "body", "id"),
      () => projects.updateProject("p", 1, payload),
      () => projects.archiveProject("p", 1),
      () => projects.updateProjectTaskStatus("p", "t", 1, "accepted"),
    ];
    for (const call of conflictCalls) await expect(call()).rejects.toBeInstanceOf(projects.ProjectApiError);
  });
});
