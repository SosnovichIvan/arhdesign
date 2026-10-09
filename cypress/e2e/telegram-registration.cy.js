describe("Регистрация и подтверждение через Telegram", () => {
  const openRegistration = () => {
    cy.visit("/");
    cy.get('[data-cy="auth-open"]').click();
    cy.get('[data-cy="auth-switch-register"]').click();
    cy.get('[role="dialog"][aria-label="Регистрация"]').should("be.visible");
  };

  const fillRegistration = () => {
    cy.get('[data-cy="register-login"]').type("sveta.design");
    cy.get('[data-cy="register-email"]').type("sveta@example.com");
    cy.get('[data-cy="register-first-name"]').type("Светлана");
    cy.get('[data-cy="register-role"]').click();
    cy.get('[data-cy="register-role-option-designer"]').click();
    cy.get('[data-cy="register-password"]').type("Надёжный пароль 2026!", { log: false });
    cy.get('[data-cy="register-password-confirmation"]').type("Надёжный пароль 2026!", { log: false });
  };

  const register = () => {
    cy.intercept("POST", "/api/v1/auth/register", (request) => {
      expect(request.body).to.include({ login: "sveta.design", email: "sveta@example.com", firstName: "Светлана", professionalRoleCode: "designer" });
      expect(request.body).to.have.property("password");
      request.reply({ statusCode: 202, body: { status: "verification_pending", channelSelectionRequired: true } });
    }).as("register");
    openRegistration();
    fillRegistration();
    cy.get('[data-cy="register-submit"]').click();
    cy.wait("@register");
    cy.contains("h2", "Как подтвердить профиль?").should("be.visible");
    cy.get('[data-cy="verification-required"]').should("contain.text", "Профиль ещё не подтверждён").and("contain.text", "Подтвердите данные");
  };

  it("открывает авторизацию в модальном окне и выполняет вход", () => {
    cy.intercept("POST", "/api/v1/auth/login", (request) => {
      expect(request.body).to.deep.equal({ identifier: "sveta.design", password: "Надёжный пароль 2026!", rememberMe: true });
      request.reply({ statusCode: 200, body: { account: { firstName: "Светлана" }, expiresAt: "2026-09-25T00:00:00Z" } });
    }).as("login");
    cy.intercept("GET", "/api/v1/projects?pageSize=50", {
      statusCode: 200,
      body: { items: [], nextCursor: null },
    }).as("projects");
    cy.visit("/");
    cy.get('[data-cy="auth-open"]').click();
    cy.get('[role="dialog"][aria-label="Авторизация"]').should("be.visible");
    cy.get('[data-cy="login-identifier"]').should("be.focused").and("have.css", "padding-left", "12px").type("sveta.design");
    cy.get('[data-cy="login-password"]').should("have.css", "padding-left", "12px").and("have.css", "padding-right", "48px").type("Надёжный пароль 2026!", { log: false });
    cy.contains("label", "Запомнить меня").find('input[type="checkbox"]').check();
    cy.get('[data-cy="login-submit"]').click();
    cy.wait("@login");
    cy.location("pathname").should("eq", "/account");
    cy.wait("@projects");
    cy.contains("h1", "Проекты").should("be.visible");
    cy.contains("Перейти в кабинет").should("not.exist");
  });

  it("открывает авторизацию из мобильного меню", () => {
    cy.viewport(390, 844);
    cy.visit("/");
    cy.get('button[aria-label="Открыть меню"]').click();
    cy.get('[data-cy="auth-open-mobile"]').click();
    cy.get('[role="dialog"][aria-label="Авторизация"]').should("be.visible");
    cy.get('[data-cy="login-identifier"]').should("be.focused");
    cy.get('button[aria-label="Закрыть форму"]').click();
    cy.get('[role="dialog"]').should("not.exist");
  });

  it("затемняет лендинг фирменным оверлеем в тёмной теме", () => {
    cy.visit("/");
    cy.get('button[aria-label="Включить тёмную тему"]').filter(":visible").click();
    cy.get('[data-cy="auth-open"]').click();
    cy.get("[data-dialog-overlay]").should("have.css", "background-color", "rgba(5, 7, 5, 0.86)");
    cy.get('[role="dialog"][aria-label="Авторизация"]').should("be.visible");
  });

  it("предлагает подтверждение при входе в неподтверждённый профиль", () => {
    cy.intercept("POST", "/api/v1/auth/login", { statusCode: 403, body: { error: { code: "account_unverified", message: "Учётная запись не подтверждена" } } }).as("unverifiedLogin");
    cy.visit("/");
    cy.get('[data-cy="auth-open"]').click();
    cy.get('[data-cy="login-identifier"]').type("sveta.design");
    cy.get('[data-cy="login-password"]').type("Надёжный пароль 2026!", { log: false });
    cy.get('[data-cy="login-submit"]').click();
    cy.wait("@unverifiedLogin");
    cy.get('[data-cy="verification-required"]').should("contain.text", "Профиль ещё не подтверждён");
    cy.get('[data-cy="verification-email"]').should("be.visible");
    cy.get('[data-cy="verification-telegram"]').should("be.visible");
    cy.get('[role="alert"]').should("not.exist");
  });

  it("выбирает роль с клавиатуры в фирменном селекте", () => {
    openRegistration();
    cy.get('[data-cy="register-role"]').focus().type("{downArrow}{enter}").should("contain.text", "Дизайнер").and("have.attr", "aria-expanded", "false");
  });

  it("показывает и повторно скрывает пароли регистрации", () => {
    openRegistration();
    cy.get('[data-cy="register-password"]').type("Надёжный пароль 2026!", { log: false }).should("have.attr", "type", "password");
    cy.get('[data-cy="register-password-visibility"]').should("have.attr", "aria-label", "Показать пароль").click();
    cy.get('[data-cy="register-password"]').should("have.attr", "type", "text").and("have.value", "Надёжный пароль 2026!");
    cy.get('[data-cy="register-password-visibility"]').should("have.attr", "aria-label", "Скрыть пароль").click();
    cy.get('[data-cy="register-password"]').should("have.attr", "type", "password");
    cy.get('[data-cy="register-password-confirmation-visibility"]').should("be.visible");
  });

  it("открывает Telegram и не запрашивает Telegram-логин или пароль на сайте", () => {
    register();
    cy.contains("Логин и пароль вводятся только внутри Telegram").should("be.visible");
    cy.get('input[name="telegramLogin"], input[name="telegramPassword"]').should("not.exist");
    cy.intercept("POST", "/api/v1/auth/verification-channel", (request) => {
      expect(request.body).to.deep.equal({ identifier: "sveta.design", channel: "telegram" });
      request.reply({
        statusCode: 202,
        body: {
          status: "accepted",
          channel: "telegram",
          telegramBotUrl: `${Cypress.config("baseUrl")}/__telegram_mock`,
        },
      });
    }).as("telegramChannel");
    cy.get('[data-cy="verification-telegram"]').click();
    cy.wait("@telegramChannel");
    cy.location("pathname").should("eq", "/__telegram_mock");
  });

  it("отправляет письмо только после явного выбора почты", () => {
    register();
    cy.intercept("POST", "/api/v1/auth/verification-channel", (request) => {
      expect(request.body).to.deep.equal({ identifier: "sveta.design", channel: "email" });
      request.reply({ statusCode: 202, body: { status: "accepted", channel: "email" } });
    }).as("emailChannel");
    cy.get('[data-cy="verification-email"]').click();
    cy.wait("@emailChannel");
    cy.contains("h2", "Проверьте почту").should("be.visible");
  });

  it("показывает ошибки регистрации и сохраняет введённые данные", () => {
    cy.intercept("POST", "/api/v1/auth/register", { statusCode: 409, body: { error: { code: "identifier_unavailable", message: "Логин или почта недоступны" } } }).as("duplicate");
    openRegistration();
    fillRegistration();
    cy.get('[data-cy="register-submit"]').click();
    cy.wait("@duplicate");
    cy.get('[role="alert"]').should("contain.text", "Логин или почта недоступны");
    cy.get('[data-cy="register-login"]').should("have.value", "sveta.design");
  });

  it("не отправляет регистрацию со слабым паролем", () => {
    cy.intercept("POST", "/api/v1/auth/register").as("weakPasswordRegistration");
    openRegistration();
    fillRegistration();
    cy.get('[data-cy="register-password"]').clear().type("короткий", { log: false });
    cy.get('[data-cy="register-password-confirmation"]').clear().type("короткий", { log: false });
    cy.get('[data-cy="register-submit"]').click();
    cy.get('[role="alert"]').should("contain.text", "Исправьте ошибки в выделенных полях");
    cy.contains("Пароль должен содержать от 15 до 128 символов").should("be.visible");
    cy.get('[data-cy="register-password"]').should("have.attr", "aria-invalid", "true");
    cy.get("@weakPasswordRegistration.all").should("have.length", 0);
  });

  it("подтверждает профиль по одноразовой ссылке и предлагает войти", () => {
    const token = "v".repeat(43);
    cy.intercept("POST", "/api/v1/auth/verify-email", (request) => {
      expect(request.body).to.deep.equal({ token });
      request.reply({ statusCode: 204 });
    }).as("verifyEmail");
    cy.visit(`/?auth=verify#token=${token}`);
    cy.wait("@verifyEmail");
    cy.contains("h2", "Почта подтверждена").should("be.visible");
    cy.contains("Профиль активирован").should("be.visible");
    cy.get('[role="dialog"][aria-label="Подтверждение профиля"]').contains("button", "Войти").click();
    cy.get('[role="dialog"][aria-label="Авторизация"]').should("be.visible");
  });

  it("повторно отправляет письмо после защитной паузы", () => {
    cy.clock();
    register();
    cy.intercept("POST", "/api/v1/auth/verification-channel", {
      statusCode: 202,
      body: { status: "accepted", channel: "email" },
    }).as("selectEmail");
    cy.intercept("POST", "/api/v1/auth/resend-verification", (request) => {
      expect(request.body).to.deep.equal({ identifier: "sveta.design" });
      request.reply({ statusCode: 202, body: { status: "accepted" } });
    }).as("resendVerification");
    cy.get('[data-cy="verification-email"]').click();
    cy.wait("@selectEmail");
    cy.contains("button", "Отправить ещё раз (60 сек.)").should("be.disabled");
    Cypress._.times(60, () => cy.tick(1_000));
    cy.contains("button", "Отправить ещё раз").should("be.enabled").click();
    cy.wait("@resendVerification");
    cy.contains("button", "Отправить ещё раз (60 сек.)").should("be.disabled");
  });

  it("не отправляет форму при несовпадении паролей", () => {
    cy.intercept("POST", "/api/v1/auth/register").as("registerRequest");
    openRegistration();
    fillRegistration();
    cy.get('[data-cy="register-password-confirmation"]').clear().type("Другой пароль 2026!", { log: false });
    cy.get('[data-cy="register-submit"]').click();
    cy.get('[role="alert"]').should("contain.text", "Исправьте ошибки в выделенных полях");
    cy.contains("Пароли не совпадают").should("be.visible");
    cy.get('[data-cy="register-password-confirmation"]').should("have.attr", "aria-invalid", "true");
    cy.get("@registerRequest.all").should("have.length", 0);
  });

  it("даёт fallback на почту, если Telegram handoff недоступен", () => {
    register();
    cy.intercept("POST", "/api/v1/auth/verification-channel", { statusCode: 202, body: { status: "accepted", channel: "telegram", telegramBotUrl: null } }).as("missingHandoff");
    cy.get('[data-cy="verification-telegram"]').click();
    cy.wait("@missingHandoff");
    cy.contains("h2", "Не удалось открыть Telegram").should("be.visible");
    cy.contains("button", "Выбрать почту").should("be.visible");
  });
});
