import { readApiError } from "@/shared/lib";

import type { CreateProjectExpensePayload, CreateProjectMeetingPayload, CreateProjectPayload, CreateProjectTaskPayload, CurrentAccount, ProjectCalendarFeed, ProjectChatContext, ProjectChatContextType, ProjectChatList, ProjectChatMember, ProjectChatMemberList, ProjectChatMessage, ProjectChatPage, ProjectChatSummary, ProjectDocument, ProjectDocumentList, ProjectExpense, ProjectExpenseList, ProjectFinanceSummary, ProjectMaterial, ProjectMaterialInput, ProjectMaterialList, ProjectMember, ProjectMemberCandidateList, ProjectMemberList, ProjectPage, ProjectTask, ProjectTaskList, ProjectTaskStatus, ProjectUpcomingFeed, ProjectUpcomingItem, ProjectUserSettings, ProjectView, UpdateProjectPayload } from "../model/types";

export class ProjectApiError extends Error {
  constructor(message: string, readonly status: number) {
    super(message);
  }
}

export async function loadProjects(signal?: AbortSignal): Promise<ProjectPage> {
  const response = await fetch("/api/v1/projects?pageSize=50", { credentials: "same-origin", signal });
  if (!response.ok) throw new ProjectApiError(await readApiError(response, "Не удалось загрузить проекты"), response.status);
  return response.json() as Promise<ProjectPage>;
}

export async function createProject(payload: CreateProjectPayload): Promise<ProjectView> {
  const response = await fetch("/api/v1/projects", {
    body: JSON.stringify(payload),
    credentials: "same-origin",
    headers: {
      "Content-Type": "application/json",
      "X-CSRF-Token": csrfToken(),
    },
    method: "POST",
  });
  if (!response.ok) throw new ProjectApiError(await readApiError(response, "Не удалось создать проект"), response.status);
  return response.json() as Promise<ProjectView>;
}

export async function loadProject(projectId: string, signal?: AbortSignal): Promise<ProjectView> {
  const response = await fetch(`/api/v1/projects/${encodeURIComponent(projectId)}`, { credentials: "same-origin", signal });
  if (!response.ok) throw new ProjectApiError(await readApiError(response, "Не удалось загрузить проект"), response.status);
  return response.json() as Promise<ProjectView>;
}

export async function loadProjectMembers(projectId: string, signal?: AbortSignal): Promise<ProjectMemberList> {
  const response = await fetch(`/api/v1/projects/${encodeURIComponent(projectId)}/members`, { credentials: "same-origin", signal });
  if (!response.ok) throw new ProjectApiError(await readApiError(response, "Не удалось загрузить участников"), response.status);
  return response.json() as Promise<ProjectMemberList>;
}

export async function searchProjectMemberCandidates(projectId: string, query: string, signal?: AbortSignal): Promise<ProjectMemberCandidateList> {
  const response = await fetch(`/api/v1/projects/${encodeURIComponent(projectId)}/member-candidates?query=${encodeURIComponent(query)}`, { credentials: "same-origin", signal });
  if (!response.ok) throw new ProjectApiError(await readApiError(response, "Не удалось выполнить поиск пользователей"), response.status);
  return response.json() as Promise<ProjectMemberCandidateList>;
}

export async function addProjectMember(projectId: string, userId: string): Promise<ProjectMember> {
  const response = await fetch(`/api/v1/projects/${encodeURIComponent(projectId)}/members`, {
    body: JSON.stringify({ userId }), credentials: "same-origin",
    headers: { "Content-Type": "application/json", "X-CSRF-Token": csrfToken() }, method: "POST",
  });
  if (!response.ok) throw new ProjectApiError(await readApiError(response, "Не удалось добавить участника"), response.status);
  return response.json() as Promise<ProjectMember>;
}

export async function removeProjectMember(projectId: string, userId: string): Promise<void> {
  const response = await fetch(`/api/v1/projects/${encodeURIComponent(projectId)}/members/${encodeURIComponent(userId)}`, {
    credentials: "same-origin", headers: { "X-CSRF-Token": csrfToken() }, method: "DELETE",
  });
  if (!response.ok) throw new ProjectApiError(await readApiError(response, "Не удалось удалить участника"), response.status);
}

export async function loadProjectFinanceSummary(projectId: string, signal?: AbortSignal): Promise<ProjectFinanceSummary> {
  const response = await fetch(`/api/v1/projects/${encodeURIComponent(projectId)}/finance-summary`, { credentials: "same-origin", signal });
  if (!response.ok) throw new ProjectApiError(await readApiError(response, "Не удалось загрузить финансовую сводку"), response.status);
  return response.json() as Promise<ProjectFinanceSummary>;
}

export async function loadProjectExpenses(projectId: string, signal?: AbortSignal): Promise<ProjectExpenseList> {
  const response = await fetch(`/api/v1/projects/${encodeURIComponent(projectId)}/expenses`, { credentials: "same-origin", signal });
  if (!response.ok) throw new ProjectApiError(await readApiError(response, "Не удалось загрузить расходы"), response.status);
  return response.json() as Promise<ProjectExpenseList>;
}

export async function createProjectExpense(projectId: string, payload: CreateProjectExpensePayload, idempotencyKey: string): Promise<ProjectExpense> {
  const response = await fetch(`/api/v1/projects/${encodeURIComponent(projectId)}/expenses`, {
    body: JSON.stringify(payload), credentials: "same-origin",
    headers: { "Content-Type": "application/json", "Idempotency-Key": idempotencyKey, "X-CSRF-Token": csrfToken() }, method: "POST",
  });
  if (!response.ok) throw new ProjectApiError(await readApiError(response, response.status === 409 ? "Этот запрос уже использован с другими данными. Закройте форму и попробуйте снова." : "Не удалось добавить расход"), response.status);
  return response.json() as Promise<ProjectExpense>;
}

export async function loadProjectMaterials(projectId: string, signal?: AbortSignal): Promise<ProjectMaterialList> {
  const response = await fetch(`/api/v1/projects/${encodeURIComponent(projectId)}/materials`, { credentials: "same-origin", signal });
  if (!response.ok) throw new ProjectApiError(await readApiError(response, "Не удалось загрузить материалы"), response.status);
  return response.json() as Promise<ProjectMaterialList>;
}

export async function createProjectMaterial(projectId: string, payload: ProjectMaterialInput): Promise<ProjectMaterial> {
  const response = await fetch(`/api/v1/projects/${encodeURIComponent(projectId)}/materials`, { body: JSON.stringify(payload), credentials: "same-origin", headers: { "Content-Type": "application/json", "X-CSRF-Token": csrfToken() }, method: "POST" });
  if (!response.ok) throw new ProjectApiError(await readApiError(response, "Не удалось добавить материал"), response.status);
  return response.json() as Promise<ProjectMaterial>;
}

export async function updateProjectMaterial(projectId: string, materialId: string, version: number, payload: ProjectMaterialInput): Promise<ProjectMaterial> {
  const response = await fetch(`/api/v1/projects/${encodeURIComponent(projectId)}/materials/${encodeURIComponent(materialId)}`, { body: JSON.stringify(payload), credentials: "same-origin", headers: { "Content-Type": "application/json", "If-Match": `"${version}"`, "X-CSRF-Token": csrfToken() }, method: "PATCH" });
  if (!response.ok) throw new ProjectApiError(await readApiError(response, response.status === 409 ? "Материал уже изменён. Обновите страницу и повторите действие." : "Не удалось сохранить материал"), response.status);
  return response.json() as Promise<ProjectMaterial>;
}

export async function deleteProjectMaterial(projectId: string, materialId: string, version: number, reason: string): Promise<void> {
  const response = await fetch(`/api/v1/projects/${encodeURIComponent(projectId)}/materials/${encodeURIComponent(materialId)}`, { body: JSON.stringify({ reason }), credentials: "same-origin", headers: { "Content-Type": "application/json", "If-Match": `"${version}"`, "X-CSRF-Token": csrfToken() }, method: "DELETE" });
  if (!response.ok) throw new ProjectApiError(await readApiError(response, response.status === 409 ? "Материал уже изменён. Обновите страницу и повторите действие." : "Не удалось удалить материал"), response.status);
}

export async function loadProjectChat(projectId: string, before?: string, signal?: AbortSignal): Promise<ProjectChatPage> {
  const query = new URLSearchParams({ pageSize: "50" });
  if (before) query.set("before", before);
  const response = await fetch(`/api/v1/projects/${encodeURIComponent(projectId)}/chat?${query}`, { credentials: "same-origin", signal });
  if (!response.ok) throw new ProjectApiError(await readApiError(response, "Не удалось загрузить чат проекта"), response.status);
  return response.json() as Promise<ProjectChatPage>;
}

export async function loadProjectChats(projectId: string, signal?: AbortSignal): Promise<ProjectChatList> {
  const response = await fetch(`/api/v1/projects/${encodeURIComponent(projectId)}/chats`, { credentials: "same-origin", signal });
  if (!response.ok) throw new ProjectApiError(await readApiError(response, "Не удалось загрузить список чатов"), response.status);
  return response.json() as Promise<ProjectChatList>;
}

export async function createProjectChat(projectId: string, name: string, memberUserIds: string[]): Promise<ProjectChatSummary> {
  const response = await fetch(`/api/v1/projects/${encodeURIComponent(projectId)}/chats`, { body: JSON.stringify({ name, memberUserIds }), credentials: "same-origin", headers: { "Content-Type": "application/json", "X-CSRF-Token": csrfToken() }, method: "POST" });
  if (!response.ok) throw new ProjectApiError(await readApiError(response, "Не удалось создать чат"), response.status);
  return response.json() as Promise<ProjectChatSummary>;
}

export async function updateProjectChat(projectId: string, chatId: string, name: string, version: number): Promise<ProjectChatSummary> {
  const response = await fetch(`/api/v1/projects/${encodeURIComponent(projectId)}/chats/${encodeURIComponent(chatId)}`, { body: JSON.stringify({ name, version }), credentials: "same-origin", headers: { "Content-Type": "application/json", "X-CSRF-Token": csrfToken() }, method: "PATCH" });
  if (!response.ok) throw new ProjectApiError(await readApiError(response, "Не удалось переименовать чат"), response.status);
  return response.json() as Promise<ProjectChatSummary>;
}

export async function deleteProjectChat(projectId: string, chatId: string, version: number): Promise<void> {
  const response = await fetch(`/api/v1/projects/${encodeURIComponent(projectId)}/chats/${encodeURIComponent(chatId)}?version=${version}`, { credentials: "same-origin", headers: { "X-CSRF-Token": csrfToken() }, method: "DELETE" });
  if (!response.ok) throw new ProjectApiError(await readApiError(response, response.status === 409 ? "Чат уже изменён. Обновите страницу и повторите действие." : "Не удалось удалить чат"), response.status);
}

export async function markProjectChatRead(projectId: string, chatId: string, messageId: string): Promise<void> {
  const response = await fetch(`/api/v1/projects/${encodeURIComponent(projectId)}/chats/${encodeURIComponent(chatId)}/read`, { body: JSON.stringify({ messageId }), credentials: "same-origin", headers: { "Content-Type": "application/json", "X-CSRF-Token": csrfToken() }, method: "POST" });
  if (!response.ok) throw new ProjectApiError(await readApiError(response, "Не удалось отметить сообщения прочитанными"), response.status);
}

export async function loadProjectChatMembers(projectId: string, chatId: string): Promise<ProjectChatMemberList> {
  const response = await fetch(`/api/v1/projects/${encodeURIComponent(projectId)}/chats/${encodeURIComponent(chatId)}/members`, { credentials: "same-origin" });
  if (!response.ok) throw new ProjectApiError(await readApiError(response, "Не удалось загрузить участников чата"), response.status);
  return response.json() as Promise<ProjectChatMemberList>;
}

export async function addProjectChatMember(projectId: string, chatId: string, userId: string): Promise<ProjectChatMember> {
  const response = await fetch(`/api/v1/projects/${encodeURIComponent(projectId)}/chats/${encodeURIComponent(chatId)}/members`, { body: JSON.stringify({ userId }), credentials: "same-origin", headers: { "Content-Type": "application/json", "X-CSRF-Token": csrfToken() }, method: "POST" });
  if (!response.ok) throw new ProjectApiError(await readApiError(response, "Не удалось добавить участника чата"), response.status);
  return response.json() as Promise<ProjectChatMember>;
}

export async function removeProjectChatMember(projectId: string, chatId: string, userId: string): Promise<void> {
  const response = await fetch(`/api/v1/projects/${encodeURIComponent(projectId)}/chats/${encodeURIComponent(chatId)}/members/${encodeURIComponent(userId)}`, { credentials: "same-origin", headers: { "X-CSRF-Token": csrfToken() }, method: "DELETE" });
  if (!response.ok) throw new ProjectApiError(await readApiError(response, "Не удалось удалить участника чата"), response.status);
}

export async function loadProjectDocuments(projectId: string, signal?: AbortSignal): Promise<ProjectDocumentList> {
  const response = await fetch(`/api/v1/projects/${encodeURIComponent(projectId)}/documents`, { credentials: "same-origin", signal });
  if (!response.ok) throw new ProjectApiError(await readApiError(response, "Не удалось загрузить документы"), response.status);
  return response.json() as Promise<ProjectDocumentList>;
}

export async function uploadProjectDocument(projectId: string, file: File): Promise<ProjectDocument> {
  const response = await fetch(`/api/v1/projects/${encodeURIComponent(projectId)}/documents`, { body: file, credentials: "same-origin", headers: { "Content-Type": file.type || "application/octet-stream", "X-CSRF-Token": csrfToken(), "X-File-Name": file.name }, method: "POST" });
  if (!response.ok) throw new ProjectApiError(await readApiError(response, "Не удалось загрузить документ"), response.status);
  return response.json() as Promise<ProjectDocument>;
}

export async function deleteProjectDocument(projectId: string, documentId: string): Promise<void> {
  const response = await fetch(`/api/v1/projects/${encodeURIComponent(projectId)}/documents/${encodeURIComponent(documentId)}`, { credentials: "same-origin", headers: { "X-CSRF-Token": csrfToken() }, method: "DELETE" });
  if (!response.ok) throw new ProjectApiError(await readApiError(response, "Не удалось удалить документ"), response.status);
}

export async function sendProjectChatMessage(projectId: string, body: string, clientMessageId: string): Promise<ProjectChatMessage> {
  const response = await fetch(`/api/v1/projects/${encodeURIComponent(projectId)}/chat/messages`, {
    body: JSON.stringify({ body, clientMessageId }), credentials: "same-origin",
    headers: { "Content-Type": "application/json", "X-CSRF-Token": csrfToken() }, method: "POST",
  });
  if (!response.ok) throw new ProjectApiError(await readApiError(response, response.status === 409 ? "Сообщение уже было отправлено с другим текстом" : "Не удалось отправить сообщение"), response.status);
  return response.json() as Promise<ProjectChatMessage>;
}

export async function openProjectContextChat(projectId: string, contextType: ProjectChatContextType, contextId: string, name: string): Promise<ProjectChatContext> {
  const response = await fetch(`/api/v1/projects/${encodeURIComponent(projectId)}/chats/context`, {
    body: JSON.stringify({ contextType, contextId, name }), credentials: "same-origin",
    headers: { "Content-Type": "application/json", "X-CSRF-Token": csrfToken() }, method: "PUT",
  });
  if (!response.ok) throw new ProjectApiError(await readApiError(response, "Не удалось открыть обсуждение"), response.status);
  return response.json() as Promise<ProjectChatContext>;
}

export async function loadProjectContextChat(projectId: string, chatId: string, before?: string, signal?: AbortSignal): Promise<ProjectChatPage> {
  const query = new URLSearchParams({ pageSize: "50" });
  if (before) query.set("before", before);
  const response = await fetch(`/api/v1/projects/${encodeURIComponent(projectId)}/chats/${encodeURIComponent(chatId)}?${query}`, { credentials: "same-origin", signal });
  if (!response.ok) throw new ProjectApiError(await readApiError(response, "Не удалось загрузить обсуждение"), response.status);
  return response.json() as Promise<ProjectChatPage>;
}

export async function sendProjectContextChatMessage(projectId: string, chatId: string, body: string, clientMessageId: string): Promise<ProjectChatMessage> {
  const response = await fetch(`/api/v1/projects/${encodeURIComponent(projectId)}/chats/${encodeURIComponent(chatId)}/messages`, {
    body: JSON.stringify({ body, clientMessageId }), credentials: "same-origin",
    headers: { "Content-Type": "application/json", "X-CSRF-Token": csrfToken() }, method: "POST",
  });
  if (!response.ok) throw new ProjectApiError(await readApiError(response, response.status === 409 ? "Сообщение уже было отправлено с другим текстом" : "Не удалось отправить сообщение"), response.status);
  return response.json() as Promise<ProjectChatMessage>;
}

export async function loadCurrentAccount(signal?: AbortSignal): Promise<CurrentAccount> {
  const response = await fetch("/api/v1/me", { credentials: "same-origin", signal });
  if (!response.ok) throw new ProjectApiError(await readApiError(response, "Не удалось проверить права"), response.status);
  return response.json() as Promise<CurrentAccount>;
}

export async function updateProject(projectId: string, version: number, payload: UpdateProjectPayload): Promise<ProjectView> {
  const response = await fetch(`/api/v1/projects/${encodeURIComponent(projectId)}`, {
    body: JSON.stringify(payload), credentials: "same-origin",
    headers: { "Content-Type": "application/json", "If-Match": `"${version}"`, "X-CSRF-Token": csrfToken() }, method: "PATCH",
  });
  if (!response.ok) throw new ProjectApiError(await readApiError(response, response.status === 409 ? "Проект уже изменён другим пользователем. Обновите данные." : "Не удалось сохранить проект"), response.status);
  return response.json() as Promise<ProjectView>;
}

export async function archiveProject(projectId: string, version: number): Promise<void> {
  const response = await fetch(`/api/v1/projects/${encodeURIComponent(projectId)}`, {
    credentials: "same-origin", headers: { "If-Match": `"${version}"`, "X-CSRF-Token": csrfToken() }, method: "DELETE",
  });
  if (!response.ok) throw new ProjectApiError(await readApiError(response, response.status === 409 ? "Проект уже изменён другим пользователем. Обновите данные." : "Не удалось удалить проект"), response.status);
}

export async function loadProjectUpcoming(projectId: string, days: number, signal?: AbortSignal): Promise<ProjectUpcomingFeed> {
  const response = await fetch(`/api/v1/projects/${encodeURIComponent(projectId)}/upcoming?days=${days}`, { credentials: "same-origin", signal });
  if (!response.ok) throw new ProjectApiError(await readApiError(response, "Не удалось загрузить ближайшие события"), response.status);
  return response.json() as Promise<ProjectUpcomingFeed>;
}

export async function loadProjectCalendar(projectId: string, from: string, to: string, signal?: AbortSignal): Promise<ProjectCalendarFeed> {
  const query = new URLSearchParams({ from, to });
  const response = await fetch(`/api/v1/projects/${encodeURIComponent(projectId)}/calendar?${query}`, { credentials: "same-origin", signal });
  if (!response.ok) throw new ProjectApiError(await readApiError(response, "Не удалось загрузить календарь проекта"), response.status);
  return response.json() as Promise<ProjectCalendarFeed>;
}

export async function loadProjectUserSettings(projectId: string, signal?: AbortSignal): Promise<ProjectUserSettings> {
  const response = await fetch(`/api/v1/projects/${encodeURIComponent(projectId)}/settings`, { credentials: "same-origin", signal });
  if (!response.ok) throw new ProjectApiError(await readApiError(response, "Не удалось загрузить настройки проекта"), response.status);
  return response.json() as Promise<ProjectUserSettings>;
}

export async function updateProjectUserSettings(projectId: string, settings: ProjectUserSettings): Promise<ProjectUserSettings> {
  const response = await fetch(`/api/v1/projects/${encodeURIComponent(projectId)}/settings`, {
    body: JSON.stringify(settings), credentials: "same-origin",
    headers: { "Content-Type": "application/json", "X-CSRF-Token": csrfToken() }, method: "PUT",
  });
  if (!response.ok) throw new ProjectApiError(await readApiError(response, "Не удалось сохранить настройки проекта"), response.status);
  return response.json() as Promise<ProjectUserSettings>;
}

export async function createProjectTask(projectId: string, payload: CreateProjectTaskPayload): Promise<ProjectUpcomingItem> {
  return createUpcomingItem(projectId, "tasks", payload, "Не удалось создать задачу");
}

export async function loadProjectTasks(projectId: string, signal?: AbortSignal): Promise<ProjectTaskList> {
  const response = await fetch(`/api/v1/projects/${encodeURIComponent(projectId)}/tasks`, { credentials: "same-origin", signal });
  if (!response.ok) throw new ProjectApiError(await readApiError(response, "Не удалось загрузить задачи"), response.status);
  return response.json() as Promise<ProjectTaskList>;
}

export async function updateProjectTaskStatus(projectId: string, taskId: string, version: number, status: ProjectTaskStatus): Promise<ProjectTask> {
  const response = await fetch(`/api/v1/projects/${encodeURIComponent(projectId)}/tasks/${encodeURIComponent(taskId)}`, {
    body: JSON.stringify({ status }), credentials: "same-origin",
    headers: { "Content-Type": "application/json", "If-Match": String(version), "X-CSRF-Token": csrfToken() }, method: "PATCH",
  });
  if (!response.ok) throw new ProjectApiError(await readApiError(response, response.status === 409 ? "Задача уже изменена. Обновите список и повторите действие." : "Не удалось изменить статус задачи"), response.status);
  return response.json() as Promise<ProjectTask>;
}

export async function createProjectMeeting(projectId: string, payload: CreateProjectMeetingPayload): Promise<ProjectUpcomingItem> {
  return createUpcomingItem(projectId, "meetings", payload, "Не удалось создать встречу");
}

async function createUpcomingItem(projectId: string, resource: "tasks" | "meetings", payload: CreateProjectTaskPayload | CreateProjectMeetingPayload, fallback: string): Promise<ProjectUpcomingItem> {
  const response = await fetch(`/api/v1/projects/${encodeURIComponent(projectId)}/${resource}`, {
    body: JSON.stringify(payload), credentials: "same-origin",
    headers: { "Content-Type": "application/json", "X-CSRF-Token": csrfToken() }, method: "POST",
  });
  if (!response.ok) throw new ProjectApiError(await readApiError(response, fallback), response.status);
  return response.json() as Promise<ProjectUpcomingItem>;
}

function csrfToken() {
  const cookie = document.cookie.split("; ").find((value) => value.startsWith("arhdesign_csrf=") || value.startsWith("__Host-arhdesign_csrf="));
  return cookie ? decodeURIComponent(cookie.slice(cookie.indexOf("=") + 1)) : "";
}
