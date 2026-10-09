describe("Глобальные чаты и техническая обратная связь", () => {
  const chatId = "00000000-0000-0000-0000-000000000301";
  const userA = { email: "anna@example.test", firstName: "Анна", joinedAt: "2026-09-28T09:00:00Z", lastName: "Иванова", login: "anna", userId: "00000000-0000-0000-0000-000000000001" };
  const userB = { email: "maria@example.test", firstName: "Мария", joinedAt: "2026-09-28T09:00:00Z", lastName: "Петрова", login: "maria", userId: "00000000-0000-0000-0000-000000000002" };
  let actor = userA;
  let messages = [];

  function summary() {
    const peer = actor.userId === userA.userId ? userB : userA;
    return { displayName: `${peer.firstName} ${peer.lastName}`, id: chatId, kind: "direct", lastActivityAt: "2026-09-28T10:00:00Z", lastMessage: messages.at(-1) ?? null, members: [userA, userB], name: null, unreadCount: actor.userId === userB.userId && messages.length ? 1 : 0, version: 1 };
  }

  beforeEach(() => {
    actor = userA;
    messages = [];
    cy.setCookie("arhdesign_csrf", "chat-csrf");
    cy.intercept("GET", "/api/v1/projects?pageSize=1", { statusCode: 200, body: { hasMore: false, items: [], nextCursor: null } });
    cy.intercept("GET", "/api/v1/settings", { statusCode: 200, body: { theme: "light" } });
    cy.intercept("GET", "/api/v1/chats?pageSize=50", (request) => request.reply({ statusCode: 200, body: { hasMore: false, items: [summary()], nextCursor: null } })).as("chats");
    cy.intercept("GET", `/api/v1/chats/${chatId}?*`, (request) => request.reply({ statusCode: 200, body: { canSend: true, chat: summary(), currentUserId: actor.userId, hasMore: false, messages, nextCursor: null } })).as("conversation");
    cy.intercept("POST", `/api/v1/chats/${chatId}/read`, (request) => { expect(request.headers).to.have.property("x-csrf-token", "chat-csrf"); request.reply({ statusCode: 204 }); }).as("readChat");
    cy.intercept("POST", `/api/v1/chats/${chatId}/messages`, (request) => {
      expect(request.headers).to.have.property("x-csrf-token", "chat-csrf");
      expect(request.headers["idempotency-key"]).to.be.a("string");
      expect(request.headers["idempotency-key"]).not.to.equal("");
      const message = { author: { firstName: actor.firstName, lastName: actor.lastName, login: actor.login, userId: actor.userId }, body: request.body.body, chatId, createdAt: new Date().toISOString(), deletedAt: null, editedAt: null, id: request.body.clientMessageId, version: 1 };
      messages.push(message);
      request.reply({ statusCode: 201, body: message });
    }).as("sendMessage");
  });

  it("передаёт сообщение между двумя пользователями и снимает непрочитанный статус", () => {
    cy.visit("/account/chats");
    cy.wait(["@chats", "@conversation", "@readChat"]);
    cy.get('[data-cy="global-chat-message"]').type("Здравствуйте, Мария");
    cy.get('[data-cy="send-global-chat-message"]').click();
    cy.wait("@sendMessage");
    cy.get('[data-cy="global-chat-history"]').should("contain.text", "Здравствуйте, Мария");

    cy.then(() => { actor = userB; });
    cy.reload();
    cy.wait(["@chats", "@conversation", "@readChat"]);
    cy.get('[data-cy="global-chat-history"]').should("contain.text", "Здравствуйте, Мария");
    cy.get('[data-cy="global-chat-conversation"]').should("contain.text", "Анна Иванова");
  });

  it("отправляет жалобу техническому администратору без секретов", () => {
    cy.intercept("POST", "/api/v1/technical/feedback", (request) => {
      expect(request.headers).to.have.property("x-csrf-token", "chat-csrf");
      expect(request.body).to.deep.equal({ category: "complaint", message: "Не загружается история сообщений", path: "/account/chats" });
      request.reply({ statusCode: 202 });
    }).as("technicalFeedback");
    cy.visit("/account/chats");
    cy.wait("@conversation");
    cy.get('[data-cy="technical-feedback-open"]').click();
    cy.get('[data-cy="technical-feedback-form"]').should("contain.text", "Не указывайте пароли");
    cy.get('[data-cy="technical-feedback-message"]').type("Не загружается история сообщений");
    cy.get('[data-cy="technical-feedback-submit"]').click();
    cy.wait("@technicalFeedback");
    cy.get('[data-cy="technical-feedback-success"]').should("contain.text", "Технический администратор получил обращение в Telegram");
  });
});
