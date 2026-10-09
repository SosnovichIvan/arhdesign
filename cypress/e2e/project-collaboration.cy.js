describe("Проект — чаты и документация", () => {
  const projectId = "00000000-0000-0000-0000-000000000010";
  const project = { id: projectId, name: "Квартира на Полянке", type: "interior_design", status: "active", currencyCode: "RUB", autoApproveExpenses: true, createdByUserId: "00000000-0000-0000-0000-000000000001", createdAt: "2026-09-01T00:00:00Z", version: 1 };
  const chat = { canManage: true, contextId: null, contextTitle: null, createdAt: "2026-09-01T00:00:00Z", id: "00000000-0000-0000-0000-000000000020", kind: "project", memberCount: 1, name: "Общий чат проекта", projectId, unreadCount: 0, version: 1 };
  const member = { firstName: "Светлана", joinedAt: "2026-09-01T00:00:00Z", lastName: "Полисмакова", login: "svetlana", middleName: null, professionalRole: { code: "designer", name: "Дизайнер" }, projectRoles: ["project_admin"], removable: false, userId: "00000000-0000-0000-0000-000000000001" };

  beforeEach(() => {
    cy.setCookie("arhdesign_csrf", "test-csrf");
    cy.intercept("GET", `/api/v1/projects/${projectId}`, { statusCode: 200, body: project });
    cy.intercept("GET", `/api/v1/projects/${projectId}/members`, { statusCode: 200, body: { canManage: true, items: [member] } });
  });

  it("создаёт именованный чат и показывает несколько чатов слева", () => {
    const chats = [chat];
    cy.intercept("GET", `/api/v1/projects/${projectId}/chats`, (request) => request.reply({ statusCode: 200, body: { canCreate: true, items: chats } }));
    cy.intercept("GET", `/api/v1/projects/${projectId}/chats/*?*`, { statusCode: 200, body: { canSend: true, chatId: chat.id, currentUserId: member.userId, hasMore: false, messages: [], nextCursor: null, projectId } });
    cy.intercept("POST", `/api/v1/projects/${projectId}/chats`, (request) => {
      expect(request.body.name).to.equal("Согласование кухни");
      const created = { ...chat, id: "00000000-0000-0000-0000-000000000021", name: request.body.name };
      chats.push(created); request.reply({ statusCode: 201, body: created });
    }).as("createChat");
    cy.visit(`/account/projects/${projectId}/chat`);
    cy.get('[data-cy="create-project-chat"]').click();
    cy.get('[data-cy="chat-name"]').type("Согласование кухни");
    cy.contains('button[type="submit"]', "Создать чат").click();
    cy.wait("@createChat");
    cy.get('[aria-label="Список чатов"]').should("contain.text", "Общий чат проекта").and("contain.text", "Согласование кухни");
  });

  it("добавляет участника, показывает непрочитанные и удаляет чат", () => {
    const candidate = { ...member, firstName: "Иван", lastName: "Петров", login: "ivan", removable: true, userId: "00000000-0000-0000-0000-000000000002" };
    cy.intercept("GET", `/api/v1/projects/${projectId}/chats`, { statusCode: 200, body: { canCreate: true, items: [{ ...chat, unreadCount: 2 }] } });
    cy.intercept("GET", `/api/v1/projects/${projectId}/chats/${chat.id}?*`, { statusCode: 200, body: { canSend: true, chatId: chat.id, currentUserId: member.userId, hasMore: false, messages: [], nextCursor: null, projectId } });
    cy.intercept("GET", `/api/v1/projects/${projectId}/chats/${chat.id}/members`, { statusCode: 200, body: { canManage: true, items: [member] } });
    cy.intercept("GET", `/api/v1/projects/${projectId}/members`, { statusCode: 200, body: { canManage: true, items: [member, candidate] } });
    cy.intercept("POST", `/api/v1/projects/${projectId}/chats/${chat.id}/members`, (request) => { expect(request.body.userId).to.equal(candidate.userId); request.reply({ statusCode: 201, body: candidate }); }).as("addChatMember");
    cy.intercept("DELETE", `/api/v1/projects/${projectId}/chats/${chat.id}?version=1`, { statusCode: 204 }).as("deleteChat");
    cy.visit(`/account/projects/${projectId}/chat/${chat.id}`);
	cy.get('button[aria-label="Настроить чат «Общий чат проекта»"]').click();
    cy.get('input[placeholder="Имя или логин"]').type("Иван");
    cy.get('[role="option"]').contains("Иван Петров").click();
    cy.wait("@addChatMember");
	cy.intercept("DELETE", `/api/v1/projects/${projectId}/chats/${chat.id}/members/${candidate.userId}`, { statusCode: 204 }).as("removeChatMember");
	cy.get(`button[aria-label="Удалить ${candidate.firstName} из чата"]`).click();
	cy.wait("@removeChatMember");
	cy.get('button[aria-label="Закрыть форму"]').click();
	cy.on("window:confirm", () => true);
	cy.get('button[aria-label="Удалить чат «Общий чат проекта»"]').click();
    cy.wait("@deleteChat");
    cy.url().should("match", new RegExp(`/account/projects/${projectId}/chat$`));
  });

  it("на мобильном показывает сначала список, затем чат с возвратом", () => {
    cy.viewport(390, 844);
    cy.intercept("GET", `/api/v1/projects/${projectId}/chats`, { statusCode: 200, body: { canCreate: true, items: [{ ...chat, unreadCount: 3 }] } });
    cy.intercept("GET", `/api/v1/projects/${projectId}/chats/${chat.id}?*`, { statusCode: 200, body: { canSend: true, chatId: chat.id, currentUserId: member.userId, hasMore: false, messages: [], nextCursor: null, projectId } });
    cy.visit(`/account/projects/${projectId}/chat`);
    cy.get('[aria-label="Список чатов"]').should("be.visible").and("contain.text", "3");
    cy.get(`[data-cy="chat-list-item-${chat.id}"]`).click();
    cy.get('[aria-label="Список чатов"]').should("not.be.visible");
    cy.get('a[aria-label="Вернуться к списку чатов"]').should("be.visible");
  });

  it("показывает задачи и изменения документации в едином центре уведомлений", () => {
    const notificationId = "00000000-0000-0000-0000-000000000040";
    cy.intercept("GET", "/api/v1/notifications?pageSize=20", { statusCode: 200, body: { unreadCount: 3, items: [
      { body: "В проекте «Полянка» появилась новая задача", createdAt: "2026-09-30T10:00:00Z", eventType: "project.task.assigned", href: `/account/projects/${projectId}/tasks`, id: notificationId, projectId, readAt: null, title: "Изменение задачи" },
      { body: "Загружен plan.pdf", createdAt: "2026-09-30T10:01:00Z", eventType: "project.document.uploaded", href: `/account/projects/${projectId}/documents`, id: "00000000-0000-0000-0000-000000000041", projectId, readAt: null, title: "Изменение документации" },
      { body: "Документ удалён", createdAt: "2026-09-30T10:02:00Z", eventType: "project.document.deleted", href: `/account/projects/${projectId}/documents`, id: "00000000-0000-0000-0000-000000000042", projectId, readAt: null, title: "Изменение документации" },
    ] } });
    cy.intercept("POST", `/api/v1/notifications/${notificationId}/read`, { statusCode: 204 }).as("readNotification");
    cy.intercept("GET", `/api/v1/projects/${projectId}/chats`, { statusCode: 200, body: { canCreate: true, items: [chat] } });
    cy.intercept("GET", `/api/v1/projects/${projectId}/chats/${chat.id}?*`, { statusCode: 200, body: { canSend: true, chatId: chat.id, currentUserId: member.userId, hasMore: false, messages: [], nextCursor: null, projectId } });
    cy.visit(`/account/projects/${projectId}/chat/${chat.id}`);
    cy.get('button[aria-label="Уведомления, непрочитанных: 3"]').click();
    cy.contains("a", "Изменение задачи").should("have.attr", "href", `/account/projects/${projectId}/tasks`);
    cy.get('[role="dialog"]').find(`a[href="/account/projects/${projectId}/documents"]`).should("have.length", 2);
    cy.contains("a", "Изменение задачи").click();
    cy.wait("@readNotification");
  });

  it("загружает, открывает и удаляет документ проекта", () => {
    let documents = [];
    cy.intercept("GET", `/api/v1/projects/${projectId}/documents`, (request) => request.reply({ statusCode: 200, body: { canUpload: true, items: documents } }));
    cy.intercept("POST", `/api/v1/projects/${projectId}/documents`, (request) => {
      expect(request.headers["x-file-name"]).to.equal("plan.pdf");
      const document = { canDelete: true, createdAt: "2026-09-30T10:00:00Z", id: "00000000-0000-0000-0000-000000000030", mediaType: "application/pdf", name: "plan.pdf", projectId, sizeBytes: 4, uploadedByUserId: member.userId, version: 1 };
      documents = [document]; request.reply({ statusCode: 201, body: document });
    }).as("uploadDocument");
    cy.intercept("DELETE", `/api/v1/projects/${projectId}/documents/*`, { statusCode: 204 }).as("deleteDocument");
    cy.visit(`/account/projects/${projectId}/documents`);
    cy.get('input[type="file"]').selectFile({ contents: Cypress.Buffer.from("test"), fileName: "plan.pdf", mimeType: "application/pdf" }, { force: true });
    cy.wait("@uploadDocument");
    cy.contains("a", "plan.pdf").should("have.attr", "target", "_blank");
    cy.on("window:confirm", () => true);
    cy.get('button[aria-label="Удалить plan.pdf"]').click();
    cy.wait("@deleteDocument");
    cy.contains("a", "plan.pdf").should("not.exist");
  });
});
