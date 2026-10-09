function openLogin() {
  cy.visit("/");
  cy.get('[data-cy="auth-open"]').click();
}

function submitUnverifiedLogin() {
  cy.intercept("POST", "/api/v1/auth/login", {
    statusCode: 403,
    body: { error: { code: "account_unverified", message: "Учётная запись не подтверждена" } },
  }).as("unverifiedLogin");
  openLogin();
  cy.get('[data-cy="login-identifier"]').type("sveta.design");
  cy.get('[data-cy="login-password"]').type("Надёжный пароль 2026!", { log: false });
  cy.get('[data-cy="login-submit"]').click();
  cy.wait("@unverifiedLogin");
}

describe("Mocked Telegram account flow", () => {
  it("передаёт неподтверждённый профиль в приватный чат бота", () => {
    submitUnverifiedLogin();
    cy.intercept("POST", "/api/v1/auth/verification-channel", (request) => {
      expect(request.body).to.deep.equal({ channel: "telegram", identifier: "sveta.design" });
      request.reply({
        statusCode: 202,
        body: { channel: "telegram", status: "accepted", telegramBotUrl: `${Cypress.config("baseUrl")}/__telegram_confirm_mock` },
      });
    }).as("telegramConfirmation");
    cy.get('[data-cy="verification-telegram"]').click();
    cy.wait("@telegramConfirmation");
    cy.location("pathname").should("eq", "/__telegram_confirm_mock");
  });

  it("запрашивает ссылку восстановления в уже привязанный Telegram без раскрытия профиля", () => {
    cy.intercept("POST", "/api/v1/auth/forgot-password", (request) => {
      expect(request.body).to.deep.equal({ channel: "telegram", identifier: "sveta.design" });
      request.reply({ statusCode: 202, body: { status: "accepted" } });
    }).as("telegramRecovery");
    openLogin();
    cy.contains("button", "Забыли пароль?").click();
    cy.contains("h2", "Восстановить пароль").should("be.visible");
    cy.contains("label", "Логин или почта").find("input").type("sveta.design");
    cy.contains("button", "Получить в Telegram").click();
    cy.wait("@telegramRecovery");
    cy.contains("h2", "Проверьте выбранный канал").should("be.visible");
    cy.contains("Telegram был подключён заранее").should("be.visible");
    cy.contains("Ссылка действует 30 минут").should("be.visible");
  });

  it("показывает нейтральную ошибку Telegram и не запрашивает пароль бота на сайте", () => {
    submitUnverifiedLogin();
    cy.intercept("POST", "/api/v1/auth/verification-channel", {
      statusCode: 429,
      body: { error: { code: "rate_limited", message: "Слишком много попыток. Повторите позже" } },
    }).as("telegramRateLimit");
    cy.get('[data-cy="verification-telegram"]').click();
    cy.wait("@telegramRateLimit");
    cy.get('[role="alert"]').should("contain.text", "Слишком много попыток");
    cy.get('input[name="telegramLogin"], input[name="telegramPassword"]').should("not.exist");
    cy.get('[data-cy="verification-email"]').should("be.visible");
  });
});
