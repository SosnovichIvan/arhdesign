const activeUser = {
  email: "anna@example.test",
  firstName: "Анна",
  globalRole: null,
  id: "00000000-0000-0000-0000-000000000101",
  lastInteractiveLoginAt: "2026-10-08T10:00:00Z",
  lastName: "Иванова",
  middleName: null,
  login: "anna.customer",
  professionalRole: { code: "customer", name: "Заказчик" },
  registeredAt: "2026-09-24T10:00:00Z",
  status: "active",
  version: 1,
};

describe("Восстановление доступа и управление сеансами", () => {
  it("меняет пароль по одноразовой ссылке и сообщает об отзыве прежних сеансов", () => {
    const token = "r".repeat(43);
    cy.intercept("POST", "/api/v1/auth/reset-password", (request) => {
      expect(request.body).to.deep.equal({
        password: "Новый надёжный пароль 2026!",
        passwordConfirmation: "Новый надёжный пароль 2026!",
        token,
      });
      request.reply({ statusCode: 204 });
    }).as("resetPassword");
    cy.visit(`/?auth=reset#token=${token}`);
    cy.get('[data-cy="reset-password"]').type("Новый надёжный пароль 2026!", { log: false });
    cy.get('[data-cy="reset-password-confirmation"]').type("Новый надёжный пароль 2026!", { log: false });
    cy.contains("button", "Сохранить пароль").click();
    cy.wait("@resetPassword");
    cy.contains("h2", "Пароль изменён").should("be.visible");
    cy.contains("Все ранее открытые сеансы завершены").should("be.visible");
  });

  it("не принимает истёкшую или уже использованную ссылку восстановления", () => {
    const token = "x".repeat(43);
    cy.intercept("POST", "/api/v1/auth/reset-password", {
      statusCode: 400,
      body: { error: { code: "invalid_or_expired_token", message: "Ссылка недействительна, уже использована или истекла" } },
    }).as("reusedResetToken");
    cy.visit(`/?auth=reset#token=${token}`);
    cy.get('[data-cy="reset-password"]').type("Новый надёжный пароль 2026!", { log: false });
    cy.get('[data-cy="reset-password-confirmation"]').type("Новый надёжный пароль 2026!", { log: false });
    cy.contains("button", "Сохранить пароль").click();
    cy.wait("@reusedResetToken");
    cy.get('[role="alert"]').should("contain.text", "уже использована");
    cy.contains("h2", "Пароль изменён").should("not.exist");
  });

  it("завершает текущий сеанс и возвращает пользователя в начало лендинга", () => {
    cy.viewport(390, 844);
    cy.setCookie("arhdesign_csrf", "logout-csrf");
    cy.intercept("GET", "/api/v1/projects?pageSize=50", { statusCode: 200, body: { hasMore: false, items: [], nextCursor: null } });
    cy.intercept("POST", "/api/v1/auth/logout", (request) => {
      expect(request.headers).to.have.property("x-csrf-token", "logout-csrf");
      request.reply({ statusCode: 204 });
    }).as("logout");
    cy.visit("/account");
    cy.get('button[aria-label="Выйти из личного кабинета"]').click();
    cy.wait("@logout");
    cy.location("pathname").should("eq", "/");
    cy.window().its("scrollY").should("eq", 0);
  });

  it("отключает пользователя с отзывом сеансов и затем восстанавливает доступ", () => {
    let current = activeUser;
    cy.setCookie("arhdesign_csrf", "admin-csrf");
    cy.intercept("GET", "/api/v1/admin/users?*", (request) => {
      request.reply({ statusCode: 200, body: { hasMore: false, items: [current], nextCursor: null } });
    }).as("users");
    cy.intercept("POST", `/api/v1/admin/users/${activeUser.id}/disable`, (request) => {
      expect(request.headers).to.include({ "if-match": '"1"', "x-csrf-token": "admin-csrf" });
      current = { ...current, status: "disabled", version: 2 };
      request.reply({ statusCode: 200, body: current });
    }).as("disableUser");
    cy.intercept("POST", `/api/v1/admin/users/${activeUser.id}/restore`, (request) => {
      expect(request.headers).to.include({ "if-match": '"2"', "x-csrf-token": "admin-csrf" });
      current = { ...current, status: "active", version: 3 };
      request.reply({ statusCode: 200, body: current });
    }).as("restoreUser");

    cy.visit("/account/users");
    cy.wait("@users");
    cy.contains("button", "Отключить").click();
    cy.get('[role="dialog"][aria-label="Отключение доступа"]').should("contain.text", "Все активные сеансы anna.customer будут немедленно завершены");
    cy.get('[role="dialog"]').contains("button", "Отключить").click();
    cy.wait("@disableUser");
    cy.contains("button", "Восстановить").click();
    cy.get('[role="dialog"][aria-label="Восстановление доступа"]').contains("button", "Восстановить").click();
    cy.wait("@restoreUser");
    cy.contains("button", "Отключить").should("be.visible");
  });
});
