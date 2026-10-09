// @vitest-environment jsdom

import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import AccountLayout from "./layout";
import AccountPage from "./page";
import NewProjectPage from "./projects/new/page";
import ProjectPage from "./projects/[projectId]/page";
import ProjectFinancesRoute from "./projects/[projectId]/finances/page";
import ProjectTasksRoute from "./projects/[projectId]/tasks/page";
import ProjectCalendarRoute from "./projects/[projectId]/calendar/page";
import ProjectChatRoute from "./projects/[projectId]/chat/page";
import ProjectContextChatRoute from "./projects/[projectId]/chat/[chatId]/page";
import ProjectDocumentsRoute from "./projects/[projectId]/documents/page";
import ProjectMaterialsRoute from "./projects/[projectId]/materials/page";
import AccountCalendarPage from "./calendar/page";
import AccountChatsPage from "./chats/page";
import AccountUsersPage from "./users/page";

vi.mock("@/widgets/accountShell", () => ({ AccountShell: ({ children }: { children: React.ReactNode }) => <div data-testid="account-shell">{children}</div> }));
vi.mock("@/features/accountProjects", () => ({ AccountProjectsPage: () => <div>Список проектов</div>, CreateProjectForm: () => <div>Форма проекта</div>, ProjectWorkspacePage: ({ projectId }: { projectId: string }) => <div>Проект {projectId}</div>, ProjectFinancesPage: ({ projectId }: { projectId: string }) => <div>Финансы {projectId}</div>, ProjectTasksPage: ({ projectId }: { projectId: string }) => <div>Задачи {projectId}</div>, ProjectCalendarPage: ({ projectId }: { projectId: string }) => <div>Календарь {projectId}</div>, ProjectChatPage: ({ chatId, projectId }: { chatId?: string; projectId: string }) => <div>Чат {projectId} {chatId}</div>, ProjectDocumentsPage: ({ projectId }: { projectId: string }) => <div>Документы {projectId}</div>, ProjectMaterialsPage: ({ projectId }: { projectId: string }) => <div>Материалы {projectId}</div> }));
vi.mock("@/features/accountCalendar", () => ({ GlobalProjectsCalendarPage: () => <div>Общий календарь</div> }));
vi.mock("@/features/accountChats", () => ({ GlobalChatsPage: () => <div>Общие чаты</div> }));
vi.mock("@/features/accountUsers", () => ({ AdminUsersPage: () => <div>Пользователи</div> }));

afterEach(cleanup);

describe("account route composition", () => {
  it("wraps account content in its shell", () => {
    render(<AccountLayout><span>Содержимое</span></AccountLayout>);
    expect(screen.getByTestId("account-shell").textContent).toContain("Содержимое");
  });

  it("renders the projects and creation pages", async () => {
    const { rerender } = render(<AccountPage />);
    expect(screen.getByText("Список проектов")).toBeTruthy();
    rerender(<NewProjectPage />);
    expect(screen.getByText("Форма проекта")).toBeTruthy();
    rerender(await ProjectPage({ params: Promise.resolve({ projectId: "project-1" }) }));
    expect(screen.getByText("Проект project-1")).toBeTruthy();
    rerender(await ProjectFinancesRoute({ params: Promise.resolve({ projectId: "project-1" }) }));
    expect(screen.getByText("Финансы project-1")).toBeTruthy();
    rerender(await ProjectTasksRoute({ params: Promise.resolve({ projectId: "project-1" }) }));
    expect(screen.getByText("Задачи project-1")).toBeTruthy();
    rerender(<AccountCalendarPage />);
    expect(screen.getByText("Общий календарь")).toBeTruthy();
  });

  it("composes the remaining global and project routes", async () => {
    const { rerender } = render(<AccountChatsPage />);
    expect(screen.getByText("Общие чаты")).toBeTruthy();
    rerender(<AccountUsersPage />);
    expect(screen.getByText("Пользователи")).toBeTruthy();
    rerender(await ProjectCalendarRoute({ params: Promise.resolve({ projectId: "project-2" }) }));
    expect(screen.getByText("Календарь project-2")).toBeTruthy();
    rerender(await ProjectChatRoute({ params: Promise.resolve({ projectId: "project-2" }) }));
    expect(screen.getByText("Чат project-2")).toBeTruthy();
    rerender(await ProjectContextChatRoute({ params: Promise.resolve({ chatId: "chat-1", projectId: "project-2" }) }));
    expect(screen.getByText("Чат project-2 chat-1")).toBeTruthy();
    rerender(await ProjectDocumentsRoute({ params: Promise.resolve({ projectId: "project-2" }) }));
    expect(screen.getByText("Документы project-2")).toBeTruthy();
    rerender(await ProjectMaterialsRoute({ params: Promise.resolve({ projectId: "project-2" }) }));
    expect(screen.getByText("Материалы project-2")).toBeTruthy();
  });
});
