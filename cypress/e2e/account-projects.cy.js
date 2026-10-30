describe("Личный кабинет — проекты", () => {
  const project = {
    id: "00000000-0000-0000-0000-000000000010",
    name: "Квартира на Полянке",
    type: "Дизайн интерьера",
    address: "Москва",
    status: "active",
    customerUserId: "00000000-0000-0000-0000-000000000001",
    plannedStartOn: "2026-09-01",
    plannedFinishOn: "2026-12-01",
    createdAt: "2026-09-01T00:00:00Z",
    createdByUserId: "00000000-0000-0000-0000-000000000001",
    currencyCode: "RUB",
    description: null,
    autoApproveExpenses: true,
    version: 1,
  };
  const emptyUpcoming = { days: 7, items: [], rangeEnd: "2026-10-02T12:00:00Z", rangeStart: "2026-09-25T12:00:00Z" };
  const customerMember = {
    email: "customer@example.com",
    firstName: "Светлана",
    lastName: "Полисмакова",
    login: "customer.one",
    middleName: null,
    professionalRole: "customer",
    projectRoles: ["customer", "project_admin"],
    removable: false,
    userId: project.customerUserId,
  };

  beforeEach(() => {
    cy.intercept("GET", /\/api\/v1\/projects\/[^/]+\/members$/, {
      statusCode: 200,
      body: { canManage: true, items: [customerMember] },
    });
    cy.intercept("GET", /\/api\/v1\/projects\/[^/]+\/finance-summary$/, {
      statusCode: 200,
      body: { availableBalanceMinor: 0, canCreateExpense: true, confirmedExpenseMinor: 0, confirmedIncomeMinor: 0, currencyCode: "RUB", pendingExpenseMinor: 0 },
    });
  });

  it("показывает пустое состояние и переход к созданию", () => {
    cy.intercept("GET", "/api/v1/projects?pageSize=50", { statusCode: 200, body: { items: [], nextCursor: null, hasMore: false } }).as("projects");
	cy.intercept("GET", "/api/v1/me", { statusCode: 200, body: { id: "admin-1", login: "svetaZZZ", globalRole: "super_admin" } }).as("accessCheck");
    cy.visit("/account");
    cy.wait("@projects");
    cy.get('[data-cy="projects-empty"]').should("contain.text", "У вас пока нет проектов");
    cy.get('[data-cy="create-project-link"]').click();
    cy.location("pathname").should("eq", "/account/projects/new");
    cy.get('[data-cy="create-project-form"]').should("be.visible");
  });

  it("создаёт проект и отправляет выбранные данные", () => {
	cy.intercept("GET", "/api/v1/me", { statusCode: 200, body: { id: "admin-1", login: "svetaZZZ", globalRole: "super_admin" } }).as("accessCheck");
    cy.setCookie("arhdesign_csrf", "test-csrf");
    cy.intercept("POST", "/api/v1/projects", (request) => {
      expect(request.headers).to.have.property("x-csrf-token", "test-csrf");
      expect(request.body).to.include({ name: "Дом 26", type: "architecture", autoApproveExpenses: true, customerUserId: null, currencyCode: "RUB" });
      request.reply({ statusCode: 201, body: { ...project, name: "Дом 26", type: "architecture" } });
    }).as("createProject");
    cy.intercept("GET", "/api/v1/projects?pageSize=50", { statusCode: 200, body: { items: [project], nextCursor: null, hasMore: false } }).as("projects");
    cy.visit("/account/projects/new");
    cy.get('[data-cy="project-name"]').type("Дом 26");
    cy.get('[data-cy="project-type"]').click();
    cy.get('[data-cy="project-type-option-architecture"]').click();
    cy.get('[data-cy="project-address"]').type("Домодедово");
    cy.get('[data-cy="project-submit"]').click();
    cy.wait("@createProject");
    cy.location("pathname").should("eq", "/account");
    cy.wait("@projects");
  });

  it("показывает фирменный календарь с видимой иконкой в тёмной теме", () => {
    cy.intercept("GET", "/api/v1/me", { statusCode: 200, body: { id: "admin-1", login: "svetaZZZ", globalRole: "super_admin" } }).as("accessCheck");
    cy.intercept("GET", "/api/v1/settings", { statusCode: 200, body: { theme: "light" } }).as("userSettings");
    cy.intercept("PUT", "/api/v1/settings", (request) => {
      expect(request.body).to.deep.equal({ theme: "dark" });
      request.reply({ statusCode: 200, body: { theme: "dark" } });
    }).as("saveUserSettings");
    cy.visit("/account/projects/new");
    cy.get('[data-cy="project-address"]').should("have.css", "padding-left", "12px");
    cy.get('[data-cy="project-type"]').should("have.css", "padding-left", "12px");
    cy.get('[data-cy="project-start"]').should("have.css", "padding-left", "12px");
    cy.get('[data-cy="project-description"]').should("have.css", "padding-left", "12px");
    cy.get('button[aria-label="Включить тёмную тему"]').filter(":visible").click();
    cy.wait("@saveUserSettings");
    cy.get("html").should("have.attr", "data-theme", "dark");
    cy.get('button[aria-label="Открыть календарь: Планируемое начало"]').should("have.css", "color", "rgb(183, 194, 174)");
    cy.get('[data-cy="project-start"]').click();
    cy.get('[role="dialog"][aria-label="Календарь: Планируемое начало"]').should("be.visible").and("have.css", "background-color", "rgb(30, 33, 31)");
    cy.get('button[aria-label="Следующий месяц"]').click();
    cy.get('[role="dialog"][aria-label="Календарь: Планируемое начало"]').should("be.visible");
  });

  it("не позволяет указать начало проекта позже завершения", () => {
    cy.intercept("GET", "/api/v1/me", { statusCode: 200, body: { id: "admin-1", login: "svetaZZZ", globalRole: "super_admin" } }).as("accessCheck");
    cy.intercept("POST", "/api/v1/projects").as("createProject");
    const today = new Date();
    const finish = new Date(today.getFullYear(), today.getMonth(), 1);
    const start = new Date(today.getFullYear(), today.getMonth(), 2);

    cy.visit("/account/projects/new");
    cy.get('[data-cy="project-name"]').type("Проект с датами");
    cy.get('[data-cy="project-finish"]').click();
    cy.get(`button[aria-label="${formatLongDate(finish)}"]`).click();
    cy.get('[data-cy="project-start"]').click();
    cy.get(`button[aria-label="${formatLongDate(start)}"]`).click();
    cy.get('[data-cy="project-finish"]').should("have.attr", "aria-invalid", "true");
    cy.get('[data-cy="project-finish-error"]').should("contain.text", "Дата начала не может быть позже даты завершения");
    cy.get('[data-cy="project-submit"]').click();
    cy.get("@createProject.all").should("have.length", 0);
  });

  it("показывает проекты, фильтрует и отражает число активных", () => {
    cy.intercept("GET", "/api/v1/projects?pageSize=50", { statusCode: 200, body: { items: [project, { ...project, id: "2", name: "Дом 25", status: "draft" }], nextCursor: null, hasMore: false } }).as("projects");
    cy.visit("/account");
    cy.wait("@projects");
    cy.get('[data-cy="project-grid"]').should("contain.text", "Квартира на Полянке").and("contain.text", "Дом 25");
    cy.get('[data-cy="account-nav-проекты"]').should("contain.text", "1");
    cy.get('[data-cy="project-search"]').type("Дом 25");
    cy.get('[data-cy="project-grid"]').should("not.contain.text", "Квартира на Полянке").and("contain.text", "Дом 25");
  });

  it("открывает страницу проекта и сохраняет изменения с версией", () => {
    cy.setCookie("arhdesign_csrf", "test-csrf");
    cy.intercept("GET", "/api/v1/projects?pageSize=1", { statusCode: 200, body: { items: [], nextCursor: null, hasMore: false } });
    cy.intercept("GET", `/api/v1/projects/${project.id}`, { statusCode: 200, body: { ...project, type: "interior_design" } }).as("project");
    cy.intercept("GET", "/api/v1/me", { statusCode: 200, body: { id: project.createdByUserId, login: "customer.one", globalRole: null } }).as("me");
    cy.intercept("GET", `/api/v1/projects/${project.id}/settings`, { statusCode: 200, body: { upcomingDays: 7 } }).as("projectSettings");
    cy.intercept("GET", `/api/v1/projects/${project.id}/upcoming?days=*`, { statusCode: 200, body: emptyUpcoming }).as("upcoming");
    cy.intercept("PATCH", `/api/v1/projects/${project.id}`, (request) => {
      expect(request.headers).to.have.property("if-match", '"1"');
      expect(request.headers).to.have.property("x-csrf-token", "test-csrf");
      expect(request.body.name).to.equal("Квартира на Полянке — обновлено");
      request.reply({ statusCode: 200, body: { ...project, type: "interior_design", name: request.body.name, version: 2 } });
    }).as("updateProject");
    cy.visit(`/account/projects/${project.id}`);
    cy.wait(["@project", "@me"]);
    cy.get('[data-cy="project-workspace"]', { timeout: 10000 }).should("contain.text", "Квартира на Полянке");
    cy.get('[data-cy="back-to-projects"]').should("contain.text", "Назад к проектам");
    cy.get('[data-cy="project-nav-summary"]').should("have.attr", "aria-current", "page");
    cy.get('[data-cy="project-nav-tasks"]').should("have.attr", "href", `/account/projects/${project.id}/tasks`).and("not.have.attr", "aria-disabled");
    cy.get('[data-cy="account-nav-проекты"]').should("not.exist");
    cy.get("aside").should("not.contain.text", "ПРОЕКТ").and("not.contain.text", "Квартира на Полянке");
    cy.get('[aria-label="Хлебные крошки"]').should("contain.text", "Квартира на Полянке");
    cy.get("#project-title").should("have.class", "sr-only");
    cy.get('[data-cy="delete-project"]').should("not.exist");
    cy.get('[data-cy="edit-project"]').should("have.attr", "aria-label", "Редактировать проект").and("have.css", "width", "36px").and("have.css", "height", "36px").and("have.css", "background-color", "rgb(255, 255, 255)").find("svg").should("exist");
    cy.get('[data-cy="edit-project"]').click();
    cy.get('[data-cy="edit-project-name"]').clear().type("Квартира на Полянке — обновлено");
    cy.get('[data-cy="save-project"]').click();
    cy.wait("@updateProject");
    cy.get('[data-cy="project-workspace"]').should("contain.text", "Квартира на Полянке — обновлено");
  });

  it("настраивает период и создаёт задачу и встречу в ближайших событиях", () => {
    cy.setCookie("arhdesign_csrf", "test-csrf");
    cy.intercept("GET", "/api/v1/projects?pageSize=1", { statusCode: 200, body: { items: [], nextCursor: null, hasMore: false } });
    cy.intercept("GET", `/api/v1/projects/${project.id}`, { statusCode: 200, body: { ...project, type: "interior_design" } });
    cy.intercept("GET", "/api/v1/me", { statusCode: 200, body: { id: project.createdByUserId, login: "customer.one", globalRole: null } });
    let upcomingDays = 7;
    cy.intercept("GET", `/api/v1/projects/${project.id}/settings`, (request) => request.reply({ statusCode: 200, body: { upcomingDays } })).as("projectSettings");
    cy.intercept("PUT", `/api/v1/projects/${project.id}/settings`, (request) => {
      expect(request.headers).to.have.property("x-csrf-token", "test-csrf");
      expect(request.body).to.deep.equal({ upcomingDays: 14 });
      upcomingDays = request.body.upcomingDays;
      request.reply({ statusCode: 200, body: { upcomingDays } });
    }).as("saveProjectSettings");
    const tomorrow = new Date();
    tomorrow.setDate(tomorrow.getDate() + 1);
    tomorrow.setHours(12, 0, 0, 0);
    const nextDay = new Date(tomorrow);
    nextDay.setDate(nextDay.getDate() + 1);
    const items = [];
    cy.intercept("GET", `/api/v1/projects/${project.id}/upcoming?days=*`, (request) => {
      const days = Number(new URL(request.url).searchParams.get("days"));
      request.reply({ statusCode: 200, body: { ...emptyUpcoming, days, items } });
    }).as("upcoming");
    cy.intercept("POST", `/api/v1/projects/${project.id}/tasks`, (request) => {
      expect(request.headers).to.have.property("x-csrf-token", "test-csrf");
      expect(request.body).to.include({ title: "Проверить чертежи" });
      const task = { createdAt: new Date().toISOString(), description: "Проверить план", effectiveAt: tomorrow.toISOString(), endsAt: null, id: "task-1", kind: "task", location: null, projectId: project.id, status: "new", title: request.body.title, version: 1 };
      items.push(task);
      request.reply({ statusCode: 201, body: task });
    }).as("createTask");
    cy.intercept("POST", `/api/v1/projects/${project.id}/meetings`, (request) => {
      expect(request.headers).to.have.property("x-csrf-token", "test-csrf");
      expect(request.body).to.include({ title: "Встреча на объекте", location: "Москва" });
      const meeting = { createdAt: new Date().toISOString(), description: null, effectiveAt: nextDay.toISOString(), endsAt: new Date(nextDay.getTime() + 60 * 60 * 1000).toISOString(), id: "meeting-1", kind: "meeting", location: request.body.location, projectId: project.id, status: null, title: request.body.title, version: 1 };
      items.push(meeting);
      request.reply({ statusCode: 201, body: meeting });
    }).as("createMeeting");

    cy.visit(`/account/projects/${project.id}`);
    cy.wait("@upcoming");
    ["Создать задачу", "Создать встречу", "Настроить период"].forEach((label) => {
      cy.get(`button[aria-label="${label}"]`).focus().invoke("attr", "aria-describedby").then((tooltipId) => {
        cy.get(`#${tooltipId}`).should("be.visible").and("contain.text", label);
      });
    });
    cy.get('[aria-label="Настроить период"]').click();
    cy.get('[data-cy="upcoming-period-form"] input[type="number"]').clear().type("14");
    cy.get('[data-cy="upcoming-period-form"]').contains("button", "Сохранить").click();
    cy.wait("@saveProjectSettings");
    cy.get('[data-cy="project-upcoming"]').should("contain.text", "ближайшие 14 дней");
    cy.get("@upcoming.all").should((calls) => {
      expect(calls.some((call) => call.request.url.includes("days=14"))).to.equal(true);
    });

    cy.get('[aria-label="Создать задачу"]').click();
    cy.get('[data-cy="create-task-form"] input').first().type("Проверить чертежи");
    cy.get('[data-cy="create-task-form"] textarea').type("Проверить план");
    cy.get('[data-cy="create-task-form"] button[type="submit"]').click();
    cy.get('[data-cy="create-task-form"] [role="alert"]').should("contain.text", "Выберите дату дедлайна");
    cy.get('[data-cy="create-task-form"] [role="combobox"]').click();
    cy.get('[data-cy="create-task-form"] [role="dialog"]').should("be.visible").and("have.css", "width", "320px");
    cy.get(`button[aria-label="${formatLongDate(new Date())}"]`).click();
    cy.get('[data-cy="create-task-form"] input[type="time"]').clear().type("00:00");
    cy.get('[data-cy="create-task-form"] button[type="submit"]').click();
    cy.get('[data-cy="create-task-form"] [role="alert"]').should("contain.text", "Выбранный дедлайн уже прошёл");
    cy.get('[data-cy="create-task-form"] [role="combobox"]').click();
    cy.get(`button[aria-label="${formatLongDate(tomorrow)}"]`).click();
    cy.get('[data-cy="create-task-form"] input[type="time"]').clear().type("12:00");
    cy.get('[data-cy="create-task-form"] button[type="submit"]').click();
    cy.wait("@createTask");
    cy.get('[data-cy="project-upcoming"]').should("contain.text", "Задача создана").and("contain.text", "Проверить чертежи");

    cy.get('[aria-label="Создать встречу"]').click();
    cy.get('[data-cy="create-meeting-form"] input').first().type("Встреча на объекте");
    cy.get('[data-cy="create-meeting-form"] input').eq(1).type("Москва");
    cy.get('[data-cy="create-meeting-form"] button[type="submit"]').click();
    cy.get('[data-cy="create-meeting-form"] [role="alert"]').should("contain.text", "Выберите дату начала встречи");
    cy.get('[data-cy="create-meeting-form"] [role="combobox"]').first().click();
    cy.get(`button[aria-label="${formatLongDate(new Date())}"]`).click();
    cy.get('[data-cy="create-meeting-form"] [role="combobox"]').eq(1).click();
    cy.get('[data-cy="create-meeting-form"] [role="dialog"]').then(($calendar) => {
      const bounds = $calendar[0].getBoundingClientRect();
      expect(bounds.left).to.be.at.least(16);
      expect(bounds.right).to.be.at.most(Cypress.config("viewportWidth") - 16);
      expect(bounds.top).to.be.at.least(16);
      expect(bounds.bottom).to.be.at.most(Cypress.config("viewportHeight") - 16);
    });
    cy.get(`button[aria-label="${formatLongDate(new Date())}"]`).click();
    cy.get('[data-cy="create-meeting-form"] input[type="time"]').first().clear().type("00:00");
    cy.get('[data-cy="create-meeting-form"] input[type="time"]').eq(1).clear().type("01:00");
    cy.get('[data-cy="create-meeting-form"] button[type="submit"]').click();
    cy.get('[data-cy="create-meeting-form"] [role="alert"]').should("contain.text", "Выбранное время начала уже прошло");
    cy.get('[data-cy="create-meeting-form"] [role="combobox"]').first().click();
    cy.get(`button[aria-label="${formatLongDate(nextDay)}"]`).click();
    cy.get('[data-cy="create-meeting-form"] [role="combobox"]').eq(1).click();
    cy.get(`button[aria-label="${formatLongDate(nextDay)}"]`).click();
    cy.get('[data-cy="create-meeting-form"] input[type="time"]').first().clear().type("12:00");
    cy.get('[data-cy="create-meeting-form"] input[type="time"]').eq(1).clear().type("13:00");
    cy.get('[data-cy="create-meeting-form"] button[type="submit"]').click();
    cy.wait("@createMeeting");
    cy.get('[data-cy="project-upcoming"]').should("contain.text", "Встреча создана").and("contain.text", "Встреча на объекте").and("contain.text", "Москва");
  });

  it("открывает задачи проекта, фильтрует, создаёт и меняет статус", () => {
    cy.setCookie("arhdesign_csrf", "test-csrf");
    cy.intercept("GET", "/api/v1/projects?pageSize=1", { statusCode: 200, body: { items: [], nextCursor: null, hasMore: false } });
    cy.intercept("GET", `/api/v1/projects/${project.id}`, { statusCode: 200, body: { ...project, type: "interior_design" } });
    cy.intercept("GET", "/api/v1/me", { statusCode: 200, body: { id: project.createdByUserId, login: "customer.one", globalRole: null } });
    const tomorrow = new Date();
    tomorrow.setDate(tomorrow.getDate() + 1);
    tomorrow.setHours(12, 0, 0, 0);
    const tasks = [{
      assignees: [{ firstName: customerMember.firstName, lastName: customerMember.lastName, login: customerMember.login, userId: customerMember.userId }],
      canChangeStatus: true,
      canEdit: true,
      completedAt: null,
      createdAt: new Date().toISOString(),
      description: "Сверить комплект с заказчиком",
      dueAt: tomorrow.toISOString(),
      id: "task-1",
      projectId: project.id,
      startedAt: null,
      status: "new",
      title: "Проверить чертежи",
      version: 1,
    }];
    cy.intercept("GET", `/api/v1/projects/${project.id}/tasks`, (request) => request.reply({ statusCode: 200, body: { canCreate: true, items: tasks } })).as("tasks");
    cy.intercept("POST", `/api/v1/projects/${project.id}/tasks`, (request) => {
      expect(request.headers).to.have.property("x-csrf-token", "test-csrf");
      expect(request.body).to.include({ title: "Подготовить ведомость" });
      expect(request.body.assigneeUserIds).to.deep.equal([customerMember.userId]);
      tasks.push({ ...tasks[0], description: null, id: "task-2", title: request.body.title });
      request.reply({ statusCode: 201, body: { createdAt: new Date().toISOString(), description: null, effectiveAt: request.body.dueAt, endsAt: null, id: "task-2", kind: "task", location: null, projectId: project.id, status: "new", title: request.body.title, version: 1 } });
    }).as("createProjectTask");
    cy.intercept("PATCH", `/api/v1/projects/${project.id}/tasks/task-1`, (request) => {
      expect(request.headers).to.have.property("if-match", "1");
      expect(request.headers).to.have.property("x-csrf-token", "test-csrf");
      expect(request.body).to.deep.equal({ status: "in_progress" });
      tasks[0] = { ...tasks[0], startedAt: new Date().toISOString(), status: "in_progress", version: 2 };
      request.reply({ statusCode: 200, body: tasks[0] });
    }).as("updateProjectTask");

    cy.visit(`/account/projects/${project.id}/tasks`);
    cy.wait("@tasks");
    cy.get('[data-cy="project-nav-tasks"]').should("have.attr", "aria-current", "page");
    cy.get('[data-cy="project-task-list"]').should("contain.text", "Проверить чертежи").and("contain.text", "Светлана Полисмакова");
    cy.get('[data-cy="task-toolbar"]').within(() => {
      cy.contains("button", "Все").should("have.attr", "aria-pressed", "true");
      cy.get('[data-cy="task-search"]').should("be.visible");
      cy.get('[data-cy="task-filter-settings"]').should("be.visible");
    });
    cy.get('[data-cy="task-filter-settings"]').click();
    cy.get('[data-cy="task-filters-form"] [role="combobox"]').eq(1).click();
    cy.contains('[role="option"]', "Светлана Полисмакова").click();
    cy.get('[data-cy="apply-task-filters"]').click();
    cy.get('[data-cy="task-filter-count"]').should("have.text", "1");
    cy.get('[data-cy="task-filter-settings"]').should("have.attr", "aria-label", "Настроить фильтры, применено: 1");
    cy.get('[data-cy="task-search"]').type("нет совпадений");
    cy.get('[data-cy="project-tasks-page"]').should("contain.text", "По выбранным фильтрам задач нет");
    cy.get('[data-cy="task-search"]').clear();
    cy.get('[data-cy="create-task-page"]').click();
    cy.get('[data-cy="create-task-page-form"] input').first().type("Подготовить ведомость");
    cy.get('[data-cy="create-task-page-form"] [role="combobox"]').first().click();
    cy.contains('[role="option"]', "Светлана Полисмакова").click();
    cy.get('[data-cy="create-task-page-form"] [role="combobox"]').eq(1).click();
    cy.get(`button[aria-label="${formatLongDate(tomorrow)}"]`).click();
    cy.get('[data-cy="create-task-page-form"] input[type="time"]').clear().type("12:00");
    cy.get('[data-cy="submit-task-page"]').click();
    cy.wait("@createProjectTask");
    cy.wait("@tasks");
    cy.get('[data-cy="project-task-list"]').should("contain.text", "Подготовить ведомость");
    cy.get('[data-cy="project-task-task-1"] [role="combobox"]').click();
    cy.contains('[role="option"]', "В работе").click();
    cy.wait("@updateProjectTask");
    cy.get('[data-cy="project-task-task-1"]').should("contain.text", "В работе").and("contain.text", "Старт:");
  });

  it("показывает задачи и встречи в календаре проекта", () => {
    cy.intercept("GET", "/api/v1/projects?pageSize=1", { statusCode: 200, body: { items: [], nextCursor: null, hasMore: false } });
    cy.intercept("GET", `/api/v1/projects/${project.id}`, { statusCode: 200, body: { ...project, type: "interior_design" } }).as("calendarProject");
    const taskAt = new Date(); taskAt.setHours(12, 0, 0, 0);
    const meetingAt = new Date(taskAt); meetingAt.setDate(meetingAt.getDate() + 1); meetingAt.setHours(15, 0, 0, 0);
    const events = [
      { createdAt: taskAt.toISOString(), description: "Комплект документации", effectiveAt: taskAt.toISOString(), endsAt: null, id: "calendar-task", kind: "task", location: null, projectId: project.id, status: "new", title: "Подготовить планы", version: 1 },
      { createdAt: taskAt.toISOString(), description: null, effectiveAt: meetingAt.toISOString(), endsAt: new Date(meetingAt.getTime() + 3600000).toISOString(), id: "calendar-meeting", kind: "meeting", location: "Объект", projectId: project.id, status: null, title: "Встреча с заказчиком", version: 1 },
    ];
    cy.intercept("GET", new RegExp(`/api/v1/projects/${project.id}/calendar\\?`), (request) => {
      expect(new URL(request.url).searchParams.get("from")).to.match(/T/);
      expect(new URL(request.url).searchParams.get("to")).to.match(/T/);
      request.reply({ statusCode: 200, body: { items: events, rangeStart: taskAt.toISOString(), rangeEnd: meetingAt.toISOString() } });
    }).as("projectCalendar");

    cy.visit(`/account/projects/${project.id}/calendar`);
    cy.wait("@projectCalendar");
    cy.get('[data-cy="project-nav-calendar"]').should("have.attr", "aria-current", "page");
    cy.get('[data-cy="project-calendar-page"]').should("not.contain.text", "План проекта");
    cy.get('[data-cy="project-calendar-page"] h1').should("not.exist");
    cy.get('[data-cy="calendar-grid"]').should("contain.text", "Подготовить планы").and("contain.text", "Встреча с заказчиком");
    cy.contains("button", "Встречи").click();
    cy.get('[data-cy="calendar-grid"]').should("not.contain.text", "Подготовить планы").and("contain.text", "Встреча с заказчиком");
    cy.get('button[aria-label^="Встреча: Встреча с заказчиком"]').click();
    cy.get('[data-cy="calendar-event-details"]').should("contain.text", "Объект");
    cy.get('button[aria-label="Закрыть форму"]').click();
    cy.get('[data-cy="calendar-display-toggle"]').should("have.attr", "aria-label", "Показать события списком").click();
    cy.get('[data-cy="calendar-list"]').should("contain.text", "Встреча с заказчиком");
    cy.get('[data-cy="calendar-period-picker"]').click();
    cy.get('[data-cy="calendar-period-form"] [role="combobox"]').eq(1).click();
    cy.contains('[role="option"]', String(new Date().getFullYear() + 1)).click();
    cy.get('[data-cy="calendar-period-form"]').contains("button", "Показать").click();
    cy.wait("@projectCalendar");
  });

  it("ищет, добавляет и удаляет участника проекта", () => {
    cy.setCookie("arhdesign_csrf", "test-csrf");
    cy.intercept("GET", "/api/v1/projects?pageSize=1", { statusCode: 200, body: { items: [], nextCursor: null, hasMore: false } });
    cy.intercept("GET", `/api/v1/projects/${project.id}`, { statusCode: 200, body: { ...project, type: "interior_design" } });
    cy.intercept("GET", "/api/v1/me", { statusCode: 200, body: { id: project.createdByUserId, login: "customer.one", globalRole: null } });
    cy.intercept("GET", `/api/v1/projects/${project.id}/settings`, { statusCode: 200, body: { upcomingDays: 7 } });
    cy.intercept("GET", `/api/v1/projects/${project.id}/upcoming?days=*`, { statusCode: 200, body: emptyUpcoming });
    const candidate = {
      email: "architect@example.com",
      firstName: "Иван",
      lastName: "Петров",
      login: "ivan.architect",
      middleName: null,
      professionalRole: "architect",
      userId: "00000000-0000-0000-0000-000000000099",
    };
    let members = [customerMember];
    cy.intercept("GET", `/api/v1/projects/${project.id}/members`, (request) => {
      request.reply({ statusCode: 200, body: { canManage: true, items: members } });
    }).as("members");
    cy.intercept("GET", `/api/v1/projects/${project.id}/member-candidates?query=*`, {
      statusCode: 200,
      body: { items: [candidate] },
    }).as("memberCandidates");
    cy.intercept("POST", `/api/v1/projects/${project.id}/members`, (request) => {
      expect(request.headers).to.have.property("x-csrf-token", "test-csrf");
      expect(request.body).to.deep.equal({ userId: candidate.userId });
      const added = { ...candidate, projectRoles: ["executor"], removable: true };
      members = [...members, added];
      request.reply({ statusCode: 201, body: added });
    }).as("addMember");
    cy.intercept("DELETE", `/api/v1/projects/${project.id}/members/${candidate.userId}`, (request) => {
      expect(request.headers).to.have.property("x-csrf-token", "test-csrf");
      members = members.filter((member) => member.userId !== candidate.userId);
      request.reply({ statusCode: 204 });
    }).as("removeMember");

    cy.visit(`/account/projects/${project.id}`);
    cy.wait("@members");
    cy.get('[data-cy="manage-project-members"]').click();
    cy.get('[data-cy="project-member-search"]').type("ivan");
    cy.wait("@memberCandidates");
    cy.contains('[role="option"]', "Иван Петров").click();
    cy.get('[data-cy="add-project-member"]').click();
    cy.wait("@addMember");
    cy.get('[data-cy="project-members"]').should("contain.text", "Иван Петров").and("contain.text", "Исполнитель");
    cy.get('button[aria-label="Удалить участника Иван Петров"]').click();
    cy.get('[data-cy="remove-project-member-dialog"]').should("contain.text", "Иван Петров");
    cy.get('[data-cy="confirm-remove-project-member"]').click();
    cy.wait("@removeMember");
    cy.get('[data-cy="project-members"]').should("not.contain.text", "Иван Петров");
  });

  it("добавляет расход из финансовой сводки и переходит в реестр расходов", () => {
    cy.setCookie("arhdesign_csrf", "test-csrf");
    cy.intercept("GET", "/api/v1/projects?pageSize=1", { statusCode: 200, body: { items: [], nextCursor: null, hasMore: false } });
    cy.intercept("GET", `/api/v1/projects/${project.id}`, { statusCode: 200, body: { ...project, type: "interior_design", autoApproveExpenses: true } }).as("financeProject");
    cy.intercept("GET", "/api/v1/me", { statusCode: 200, body: { id: project.createdByUserId, login: "customer.one", globalRole: null } });
    cy.intercept("GET", `/api/v1/projects/${project.id}/settings`, { statusCode: 200, body: { upcomingDays: 7 } });
    cy.intercept("GET", `/api/v1/projects/${project.id}/upcoming?days=*`, { statusCode: 200, body: emptyUpcoming });
    const expenses = [];
    cy.intercept("GET", `/api/v1/projects/${project.id}/finance-summary`, (request) => request.reply({ statusCode: 200, body: { availableBalanceMinor: 0, canCreateExpense: true, confirmedExpenseMinor: 0, confirmedIncomeMinor: 0, currencyCode: "RUB", pendingExpenseMinor: expenses.reduce((total, item) => total + item.amountMinor, 0) } })).as("financeSummary");
    cy.intercept("POST", `/api/v1/projects/${project.id}/expenses`, (request) => {
      expect(request.headers).to.have.property("x-csrf-token", "test-csrf");
      expect(request.headers).to.have.property("idempotency-key");
      expect(request.body).to.include({ amountMinor: 125050, category: "materials", description: "Светильники" });
      const expense = { amountMinor: 125050, category: "materials", createdAt: new Date().toISOString(), createdByUserId: project.createdByUserId, currencyCode: "RUB", description: "Светильники", id: "expense-1", plannedPaymentOn: null, projectId: project.id, status: "auto_approved", vendorName: "Свет", version: 1 };
      expenses.push(expense);
      request.reply({ statusCode: 201, body: expense });
    }).as("createExpense");
    cy.intercept("GET", `/api/v1/projects/${project.id}/expenses`, (request) => request.reply({ statusCode: 200, body: { canCreateExpense: true, items: expenses } })).as("expenses");

    cy.visit(`/account/projects/${project.id}`);
    cy.wait("@financeSummary");
    cy.get('[data-cy="add-project-expense"]').click();
    cy.get('[data-cy="submit-project-expense"]').click();
    cy.get('[data-cy="create-expense-form"] [role="alert"]').should("contain.text", "Укажите сумму");
    cy.get('[data-cy="expense-amount"]').type("1250,50");
    cy.get('[data-cy="expense-description"]').type("Светильники");
    cy.get('[data-cy="expense-vendor"]').type("Свет");
    cy.get('[data-cy="submit-project-expense"]').click();
    cy.wait("@createExpense");
    cy.get('[data-cy="project-finance-summary"]').should("contain.text", "Расход добавлен и автосогласован").and("contain.text", "1 250,50");
    cy.get('[data-cy="go-to-project-expenses"]').click();
    cy.location("pathname").should("eq", `/account/projects/${project.id}/finances`);
    cy.wait("@expenses");
    cy.get('[data-cy="project-expenses-page"]').should("contain.text", "Светильники").and("contain.text", "Автосогласован").and("contain.text", "Свет");
    cy.get('[data-cy="project-nav-finances"]').should("have.attr", "aria-current", "page");
  });

  it("показывает и подтверждает удаление только супер-администратору", () => {
    cy.setCookie("arhdesign_csrf", "test-csrf");
    cy.intercept("GET", "/api/v1/projects?pageSize=1", { statusCode: 200, body: { items: [], nextCursor: null, hasMore: false } });
    cy.intercept("GET", `/api/v1/projects/${project.id}`, { statusCode: 200, body: { ...project, type: "interior_design" } }).as("superAdminProject");
    cy.intercept("GET", "/api/v1/me", { statusCode: 200, body: { id: "admin-1", login: "svetaZZZ", globalRole: "super_admin" } }).as("superAdminMe");
    cy.intercept("GET", `/api/v1/projects/${project.id}/settings`, { statusCode: 200, body: { upcomingDays: 7 } });
    cy.intercept("GET", `/api/v1/projects/${project.id}/upcoming?days=*`, { statusCode: 200, body: emptyUpcoming });
    cy.intercept("DELETE", `/api/v1/projects/${project.id}`, (request) => {
      expect(request.headers).to.have.property("if-match", '"1"');
      request.reply({ statusCode: 204 });
    }).as("deleteProject");
    cy.intercept("GET", "/api/v1/projects?pageSize=50", { statusCode: 200, body: { items: [], nextCursor: null, hasMore: false } }).as("projects");
    cy.visit(`/account/projects/${project.id}`);
    cy.wait(["@superAdminProject", "@superAdminMe"]);
    cy.get('[data-cy="project-workspace"]', { timeout: 10000 }).should("contain.text", "Квартира на Полянке");
    cy.get('[data-cy="edit-project"]').focus().invoke("attr", "aria-describedby").then((tooltipId) => {
      cy.get(`#${tooltipId}`).should("be.visible").and("contain.text", "Редактировать проект");
    });
    cy.get('[data-cy="delete-project"]').should("have.attr", "aria-label", "Удалить проект").and("have.css", "width", "36px").and("have.css", "height", "36px").and("have.css", "background-color", "rgb(255, 255, 255)").find("svg").should("exist");
    cy.get('[data-cy="delete-project"]').focus().invoke("attr", "aria-describedby").then((tooltipId) => {
      cy.get(`#${tooltipId}`).should("be.visible").and("contain.text", "Удалить проект");
    });
    cy.get('[data-cy="delete-project"]').click();
    cy.get('[data-cy="delete-project-dialog"]').should("contain.text", "Удалить проект?");
    cy.get('[data-cy="confirm-delete-project"]').click();
    cy.wait("@deleteProject");
    cy.location("pathname").should("eq", "/account");
  });

  it("ведёт материалы проекта и сохраняет связь с расходом", () => {
    cy.setCookie("arhdesign_csrf", "test-csrf");
    cy.intercept("GET", "/api/v1/projects?pageSize=1", { statusCode: 200, body: { items: [], nextCursor: null, hasMore: false } });
    cy.intercept("GET", `/api/v1/projects/${project.id}`, { statusCode: 200, body: project });
    cy.intercept("GET", `/api/v1/projects/${project.id}/expenses`, { statusCode: 200, body: { canCreateExpense: true, items: [{ amountMinor: 250000, category: "furniture", createdAt: "2026-09-26T10:00:00Z", createdByUserId: project.createdByUserId, currencyCode: "RUB", description: "Диван", id: "00000000-0000-0000-0000-000000000077", plannedPaymentOn: null, projectId: project.id, status: "approved", vendorName: "Фабрика", version: 1 }] } });
    let materials = [];
    cy.intercept("GET", `/api/v1/projects/${project.id}/materials`, (request) => request.reply({ statusCode: 200, body: { canCreate: true, canViewFinancialInformation: true, items: materials } })).as("materials");
    cy.intercept("POST", `/api/v1/projects/${project.id}/materials`, (request) => {
      expect(request.headers).to.have.property("x-csrf-token", "test-csrf");
      expect(request.body).to.include({ contractAmountMinor: 250000, name: "Диван", supplierName: "Фабрика" });
      const item = { ...request.body, actualDeliveryOn: null, canDelete: true, canEdit: true, contactInfo: null, contractReference: null, createdAt: "2026-09-26T10:00:00Z", createdByUserId: project.createdByUserId, currencyCode: "RUB", id: "00000000-0000-0000-0000-000000000088", installationOn: null, linkedExpenseDescription: "Диван", notes: null, paidAmountMinor: 0, paymentStatus: "unpaid", plannedDeliveryOn: null, projectId: project.id, remainingAmountMinor: 250000, updatedAt: "2026-09-26T10:00:00Z", version: 1 };
      materials = [item];
      request.reply({ statusCode: 201, body: item });
    }).as("createMaterial");

    cy.visit(`/account/projects/${project.id}/materials`);
    cy.wait("@materials");
    cy.get('[data-cy="project-nav-materials"]').should("have.attr", "aria-current", "page");
    cy.get('[data-cy="project-materials-page"]').should("not.contain.text", "Реестр проекта");
    cy.get('[data-cy="add-project-material"]').focus().invoke("attr", "aria-describedby").then((tooltipId) => {
      cy.get(`#${tooltipId}`).should("be.visible").and("contain.text", "Добавить материал");
    });
    cy.get('[data-cy="add-project-material"]').click();
    cy.get('[data-cy="save-material"]').click();
    cy.get('[data-cy="material-form"] [role="alert"]').should("contain.text", "название");
    cy.get('[data-cy="material-name"]').type("Диван");
    cy.get('[data-cy="material-supplier"]').type("Фабрика");
    cy.get('[data-cy="material-amount"]').type("2500");
    cy.get('[data-cy="material-expense"]').click();
    cy.contains('[role="option"]', /Диван.*2.500,00/).click();
    cy.get('[data-cy="save-material"]').click();
    cy.wait("@createMaterial");
    cy.wait("@materials");
    cy.get('[data-cy="material-row-00000000-0000-0000-0000-000000000088"]').should("contain.text", "Фабрика").find('a[href$="/finances"]').should("contain.text", "Диван");
    cy.get('[data-cy="materials-display-toggle"]').click();
    cy.get('[data-cy="materials-table"]').should("not.exist");
    cy.get('[data-cy="material-list-item-00000000-0000-0000-0000-000000000088"]').should("contain.text", "Фабрика").and(($item) => {
      expect($item.text()).to.match(/2\s500,00/);
    });
  });

  it("открывает общий чат проекта и отправляет первое сообщение", () => {
    cy.setCookie("arhdesign_csrf", "test-csrf");
    const chatId = "00000000-0000-0000-0000-000000000070";
    cy.intercept("GET", "/api/v1/projects?pageSize=1", { statusCode: 200, body: { items: [], nextCursor: null, hasMore: false } });
    cy.intercept("GET", `/api/v1/projects/${project.id}`, { statusCode: 200, body: project }).as("chatProject");
    cy.intercept("GET", `/api/v1/projects/${project.id}/chats`, {
      statusCode: 200,
      body: { canCreate: true, items: [{ canManage: true, contextId: null, contextTitle: null, createdAt: "2026-09-26T17:00:00Z", id: chatId, kind: "project", memberCount: 1, name: "Общий чат проекта", projectId: project.id, version: 1 }] },
    }).as("projectChats");
    cy.intercept("GET", `/api/v1/projects/${project.id}/chats/${chatId}?pageSize=50`, {
      statusCode: 200,
      body: { canSend: true, chatId, currentUserId: project.createdByUserId, hasMore: false, messages: [], nextCursor: null, projectId: project.id },
    }).as("projectChat");
    cy.intercept("POST", `/api/v1/projects/${project.id}/chats/${chatId}/messages`, (request) => {
      expect(request.headers).to.have.property("x-csrf-token", "test-csrf");
      expect(request.body.body).to.equal("Согласуем планировку завтра");
      expect(request.body.clientMessageId).to.match(/^[0-9a-f-]{36}$/);
      request.reply({ statusCode: 201, body: { author: { firstName: "Светлана", lastName: "Полисмакова", login: "customer.one", userId: project.createdByUserId }, body: request.body.body, chatId, createdAt: "2026-09-26T18:00:00Z", deletedAt: null, editedAt: null, id: "00000000-0000-0000-0000-000000000071", projectId: project.id, version: 1 } });
    }).as("sendChatMessage");

    cy.visit(`/account/projects/${project.id}/chat`);
    cy.wait(["@chatProject", "@projectChats", "@projectChat"]);
    cy.get('[data-cy="project-nav-chat"]').should("have.attr", "aria-current", "page");
    cy.get('[data-cy="project-chat-history"]').should("contain.text", "Сообщений пока нет");
    cy.get('[data-cy="project-chat-message"]').type("Согласуем планировку завтра");
    cy.get('[data-cy="send-project-chat"]').click();
    cy.wait("@sendChatMessage");
    cy.get('[data-cy="chat-message-00000000-0000-0000-0000-000000000071"]').should("contain.text", "Согласуем планировку завтра").and("contain.text", "Вы");
    cy.get('[data-cy="project-chat-message"]').should("have.value", "");

    cy.viewport(390, 844);
    cy.get('[data-cy="project-chat-page"]').should("be.visible").then(($card) => {
      const bounds = $card[0].getBoundingClientRect();
      expect(bounds.left).to.be.at.least(0);
      expect(bounds.right).to.be.at.most(390);
    });
  });

  it("показывает переписку и события проекта другому участнику", () => {
    cy.setCookie("arhdesign_csrf", "test-csrf");
    const participant = {
      firstName: "Иван",
      lastName: "Петров",
      login: "ivan.executor",
      userId: "00000000-0000-0000-0000-000000000002",
    };
    const chatId = "00000000-0000-0000-0000-000000000072";
    const taskChatId = "00000000-0000-0000-0000-000000000073";
    const authors = {
      [project.createdByUserId]: { firstName: "Светлана", lastName: "Полисмакова", login: "customer.one", userId: project.createdByUserId },
      [participant.userId]: participant,
    };
    let currentUserId = project.createdByUserId;
    const messages = [];
    const contextMessages = [];
    const eventAt = new Date();
    eventAt.setDate(eventAt.getDate() + 1);
    eventAt.setHours(12, 0, 0, 0);
    const assignedTask = {
      assignees: [participant], canChangeStatus: true, canEdit: true, completedAt: null,
      createdAt: new Date().toISOString(), description: "Подготовить комплект", dueAt: eventAt.toISOString(),
      id: "shared-task", projectId: project.id, startedAt: null, status: "new", title: "Рабочие чертежи", version: 1,
    };
    const projectMeeting = {
      createdAt: new Date().toISOString(), description: "Совместная проверка", effectiveAt: eventAt.toISOString(),
      endsAt: new Date(eventAt.getTime() + 3600000).toISOString(), id: "shared-meeting", kind: "meeting",
      location: "Объект", projectId: project.id, status: null, title: "Встреча на объекте", version: 1,
    };
	const sharedExpense = {
		amountMinor: 125050, category: "materials", createdAt: new Date().toISOString(),
		createdByUserId: project.createdByUserId, currencyCode: "RUB", description: "Керамогранит для санузла",
		id: "00000000-0000-0000-0000-000000000074", plannedPaymentOn: null, projectId: project.id,
		status: "approved", vendorName: "Поставщик", version: 1,
	};
	const sharedMaterial = {
		actualDeliveryOn: null, canDelete: false, canEdit: false, contactInfo: null, contextChatId: null,
		contractAmountMinor: 125050, contractReference: null, createdAt: new Date().toISOString(),
		createdByUserId: project.createdByUserId, currencyCode: "RUB", deliveryStatus: "expected",
		id: "00000000-0000-0000-0000-000000000075", installationOn: null,
		linkedExpenseDescription: sharedExpense.description, linkedExpenseId: sharedExpense.id, name: "Керамогранит серый",
		notes: null, paidAmountMinor: 0, paymentStatus: "unpaid", plannedDeliveryOn: null, projectId: project.id,
		remainingAmountMinor: 125050, supplierName: "Поставщик", updatedAt: new Date().toISOString(), version: 1,
	};

    cy.intercept("GET", "/api/v1/projects?pageSize=1", { statusCode: 200, body: { items: [], nextCursor: null, hasMore: false } });
    cy.intercept("GET", "/api/v1/me", (request) => request.reply({ statusCode: 200, body: { id: currentUserId, login: authors[currentUserId].login, globalRole: null } }));
    cy.intercept("GET", `/api/v1/projects/${project.id}`, { statusCode: 200, body: project });
    cy.intercept("GET", `/api/v1/projects/${project.id}/chats`, (request) => request.reply({
      statusCode: 200,
      body: { canCreate: true, items: [
        { canManage: true, contextId: null, contextTitle: null, createdAt: new Date().toISOString(), id: chatId, kind: "project", memberCount: 2, name: "Общий чат проекта", projectId: project.id, version: 1 },
        { canManage: true, contextId: assignedTask.id, contextTitle: assignedTask.title, contextType: "task", createdAt: new Date().toISOString(), id: taskChatId, kind: "context", memberCount: 2, name: "Обсуждение чертежей", projectId: project.id, version: 1 },
      ] },
    })).as("sharedChats");
    cy.intercept("GET", `/api/v1/projects/${project.id}/chats/${chatId}?pageSize=50`, (request) => request.reply({
      statusCode: 200,
      body: { canSend: true, chatId, currentUserId, hasMore: false, messages, nextCursor: null, projectId: project.id },
    })).as("sharedChat");
    cy.intercept("POST", `/api/v1/projects/${project.id}/chats/${chatId}/messages`, (request) => {
      const message = {
        author: authors[currentUserId], body: request.body.body, chatId, createdAt: new Date().toISOString(),
        deletedAt: null, editedAt: null, id: `shared-message-${messages.length + 1}`, projectId: project.id, version: 1,
      };
      messages.push(message);
      request.reply({ statusCode: 201, body: message });
    }).as("sendSharedMessage");
    cy.intercept("GET", `/api/v1/projects/${project.id}/chats/${taskChatId}?pageSize=50`, (request) => request.reply({
      statusCode: 200,
      body: {
        canSend: true, chatId: taskChatId, currentUserId, hasMore: false, messages: contextMessages, nextCursor: null, projectId: project.id,
        context: { chatId: taskChatId, contextId: assignedTask.id, contextTitle: assignedTask.title, contextType: "task", name: "Обсуждение чертежей", projectId: project.id },
      },
    })).as("sharedContextChat");
    cy.intercept("POST", `/api/v1/projects/${project.id}/chats/${taskChatId}/messages`, (request) => {
      const message = {
        author: authors[currentUserId], body: request.body.body, chatId: taskChatId, createdAt: new Date().toISOString(),
        deletedAt: null, editedAt: null, id: `context-message-${contextMessages.length + 1}`, projectId: project.id, version: 1,
      };
      contextMessages.push(message);
      request.reply({ statusCode: 201, body: message });
    }).as("sendSharedContextMessage");
    cy.intercept("GET", `/api/v1/projects/${project.id}/tasks`, (request) => request.reply({ statusCode: 200, body: { canCreate: false, items: [assignedTask] } })).as("participantTasks");
    cy.intercept("GET", new RegExp(`/api/v1/projects/${project.id}/calendar\\?`), (request) => request.reply({ statusCode: 200, body: { items: [projectMeeting], rangeStart: new Date().toISOString(), rangeEnd: new Date(eventAt.getTime() + 86400000).toISOString() } })).as("participantCalendar");
	cy.intercept("GET", `/api/v1/projects/${project.id}/expenses`, { statusCode: 200, body: { canCreateExpense: false, items: [sharedExpense] } }).as("participantExpenses");
	cy.intercept("GET", `/api/v1/projects/${project.id}/materials`, { statusCode: 200, body: { canCreate: false, canViewFinancialInformation: true, items: [sharedMaterial] } }).as("participantMaterials");

    cy.visit(`/account/projects/${project.id}/chat`);
    cy.wait("@sharedChat");
    cy.get('[data-cy="project-chat-message"]').type("Иван, проверьте рабочие чертежи");
    cy.get('[data-cy="send-project-chat"]').click();
    cy.wait("@sendSharedMessage");
    cy.get('[data-cy="chat-message-shared-message-1"]').should("contain.text", "Вы");

    cy.then(() => { currentUserId = participant.userId; });
    cy.visit(`/account/projects/${project.id}/chat`);
    cy.wait("@sharedChat");
    cy.get('[data-cy="chat-message-shared-message-1"]').should("contain.text", "Светлана Полисмакова").and("not.contain.text", "Вы");
    cy.get('[data-cy="project-chat-message"]').type("Проверю комплект сегодня");
    cy.get('[data-cy="send-project-chat"]').click();
    cy.wait("@sendSharedMessage");
    cy.get('[data-cy="chat-message-shared-message-2"]').should("contain.text", "Вы").and("contain.text", "Проверю комплект сегодня");

    cy.then(() => { currentUserId = project.createdByUserId; });
    cy.visit(`/account/projects/${project.id}/chat/${taskChatId}`);
    cy.wait("@sharedContextChat");
    cy.get('[data-cy="project-chat-message"]').type("Комментарий к конкретной задаче");
    cy.get('[data-cy="send-project-chat"]').click();
    cy.wait("@sendSharedContextMessage");
    cy.then(() => { currentUserId = participant.userId; });
    cy.visit(`/account/projects/${project.id}/chat/${taskChatId}`);
    cy.wait("@sharedContextChat");
    cy.get('[data-cy="chat-message-context-message-1"]').should("contain.text", "Светлана Полисмакова").and("contain.text", "Комментарий к конкретной задаче");

    cy.visit(`/account/projects/${project.id}/tasks`);
    cy.wait("@participantTasks");
    cy.get('[data-cy="project-task-shared-task"]').should("contain.text", "Рабочие чертежи").and("contain.text", "Иван Петров");
    cy.visit(`/account/projects/${project.id}/calendar`);
    cy.wait("@participantCalendar");
    cy.get('[data-cy="calendar-grid"]').should("contain.text", "Встреча на объекте");
	cy.visit(`/account/projects/${project.id}/finances`);
	cy.wait("@participantExpenses");
	cy.get('[data-cy="project-expenses-page"]').should("contain.text", "Керамогранит для санузла");
	cy.get('[data-cy="add-project-expense"]').should("not.exist");
	cy.visit(`/account/projects/${project.id}/materials`);
	cy.wait("@participantMaterials");
	cy.get(`[data-cy="material-row-${sharedMaterial.id}"]`).should("contain.text", "Керамогранит серый").and("contain.text", "Поставщик");
	cy.get('[data-cy="add-project-material"]').should("not.exist");
	cy.get(`[data-cy="edit-material-${sharedMaterial.id}"]`).should("not.exist");
	cy.get(`[data-cy="delete-material-${sharedMaterial.id}"]`).should("not.exist");

    cy.then(() => { currentUserId = project.createdByUserId; });
    cy.visit(`/account/projects/${project.id}/chat`);
    cy.wait("@sharedChat");
    cy.get('[data-cy="chat-message-shared-message-1"]').should("contain.text", "Вы");
    cy.get('[data-cy="chat-message-shared-message-2"]').should("contain.text", "Иван Петров").and("not.contain.text", "Вы");
  });

  it("создаёт и открывает чаты из задачи, материала и расхода", () => {
    cy.setCookie("arhdesign_csrf", "test-csrf");
    const taskId = "00000000-0000-0000-0000-000000000081";
    const materialId = "00000000-0000-0000-0000-000000000082";
    const expenseId = "00000000-0000-0000-0000-000000000083";
    const taskChatId = "00000000-0000-0000-0000-000000000091";
    const materialChatId = "00000000-0000-0000-0000-000000000092";
    const expenseChatId = "00000000-0000-0000-0000-000000000093";
    const tomorrow = new Date();
    tomorrow.setDate(tomorrow.getDate() + 1);
    tomorrow.setHours(12, 0, 0, 0);

    cy.intercept("GET", "/api/v1/projects?pageSize=1", { statusCode: 200, body: { items: [], nextCursor: null, hasMore: false } });
    cy.intercept("GET", `/api/v1/projects/${project.id}`, { statusCode: 200, body: project });
    cy.intercept("GET", "/api/v1/me", { statusCode: 200, body: { id: project.createdByUserId, login: "customer.one", globalRole: null } });
    cy.intercept("GET", `/api/v1/projects/${project.id}/tasks`, {
      statusCode: 200,
      body: { canCreate: true, items: [{ assignees: [], canChangeStatus: true, canEdit: true, completedAt: null, contextChatId: null, createdAt: new Date().toISOString(), description: null, dueAt: tomorrow.toISOString(), id: taskId, projectId: project.id, startedAt: null, status: "new", title: "Рабочие чертежи", version: 1 }] },
    }).as("contextTasks");
    cy.intercept("PUT", `/api/v1/projects/${project.id}/chats/context`, (request) => {
      expect(request.headers).to.have.property("x-csrf-token", "test-csrf");
      expect(request.body).to.deep.equal({ contextId: taskId, contextType: "task", name: "Чертежи кухни" });
      request.reply({ statusCode: 200, body: { chatId: taskChatId, contextId: taskId, contextTitle: "Рабочие чертежи", contextType: "task", name: "Чертежи кухни", projectId: project.id } });
    }).as("openTaskChat");

    const contexts = {
      [taskChatId]: { chatId: taskChatId, contextId: taskId, contextTitle: "Рабочие чертежи", contextType: "task", name: "Чертежи кухни", projectId: project.id },
      [materialChatId]: { chatId: materialChatId, contextId: materialId, contextTitle: "Диван", contextType: "material", name: "Обсуждение дивана", projectId: project.id },
      [expenseChatId]: { chatId: expenseChatId, contextId: expenseId, contextTitle: "Светильники", contextType: "expense", name: "Оплата светильников", projectId: project.id },
    };
    cy.intercept("GET", `/api/v1/projects/${project.id}/chats`, {
      statusCode: 200,
      body: { canCreate: true, items: Object.values(contexts).map((context) => ({ canManage: true, contextId: context.contextId, contextTitle: context.contextTitle, contextType: context.contextType, createdAt: new Date().toISOString(), id: context.chatId, kind: "context", memberCount: 1, name: context.name, projectId: project.id, version: 1 })) },
    }).as("contextChats");
    cy.intercept("GET", /\/api\/v1\/projects\/[^/]+\/chats\/[^/?]+\?pageSize=50$/, (request) => {
      const chatId = new URL(request.url).pathname.split("/").at(-1);
      request.reply({ statusCode: 200, body: { canSend: true, chatId, context: contexts[chatId], currentUserId: project.createdByUserId, hasMore: false, messages: [], nextCursor: null, projectId: project.id } });
    }).as("contextChat");
    cy.intercept("POST", `/api/v1/projects/${project.id}/chats/${taskChatId}/messages`, (request) => {
      expect(request.body.body).to.equal("Проверим комплект завтра");
      expect(request.body.clientMessageId).to.match(/^[0-9a-f-]{36}$/);
      request.reply({ statusCode: 201, body: { author: { firstName: "Светлана", lastName: "Полисмакова", login: "customer.one", userId: project.createdByUserId }, body: request.body.body, chatId: taskChatId, createdAt: new Date().toISOString(), deletedAt: null, editedAt: null, id: "00000000-0000-0000-0000-000000000099", projectId: project.id, version: 1 } });
    }).as("sendContextMessage");

    cy.visit(`/account/projects/${project.id}/tasks`);
    cy.wait("@contextTasks");
    cy.get(`[data-cy="discuss-task-${taskId}"]`).focus().invoke("attr", "aria-describedby").then((tooltipId) => {
      cy.get(`#${tooltipId}`).should("be.visible").and("contain.text", "Обсудить");
    });
    cy.get(`[data-cy="discuss-task-${taskId}"]`).click();
    cy.get('[data-cy="context-chat-form"]').should("contain.text", "Рабочие чертежи");
    cy.get('[data-cy="context-chat-name"]').clear().type("Чертежи кухни");
    cy.get('[data-cy="create-context-chat"]').click();
    cy.wait("@openTaskChat");
    cy.location("pathname").should("eq", `/account/projects/${project.id}/chat/${taskChatId}`);
    cy.wait("@contextChat");
    cy.get('[data-cy="project-chat-page"]').should("contain.text", "Чертежи кухни").and("contain.text", "Рабочие чертежи");
    cy.get('[data-cy="project-chat-message"]').type("Проверим комплект завтра");
    cy.get('[data-cy="send-project-chat"]').click();
    cy.wait("@sendContextMessage");
    cy.get('[data-cy="chat-message-00000000-0000-0000-0000-000000000099"]').should("contain.text", "Проверим комплект завтра");

    cy.intercept("GET", `/api/v1/projects/${project.id}/expenses`, {
      statusCode: 200,
      body: { canCreateExpense: true, items: [{ amountMinor: 250000, category: "furniture", contextChatId: expenseChatId, createdAt: new Date().toISOString(), createdByUserId: project.createdByUserId, currencyCode: "RUB", description: "Светильники", id: expenseId, plannedPaymentOn: null, projectId: project.id, status: "approved", vendorName: "Свет", version: 1 }] },
    }).as("contextExpenses");
    cy.intercept("GET", `/api/v1/projects/${project.id}/materials`, {
      statusCode: 200,
      body: { canCreate: true, canViewFinancialInformation: true, items: [{ actualDeliveryOn: null, canDelete: true, canEdit: true, contactInfo: null, contextChatId: materialChatId, contractAmountMinor: 250000, contractReference: null, createdAt: new Date().toISOString(), createdByUserId: project.createdByUserId, currencyCode: "RUB", id: materialId, installationOn: null, linkedExpenseDescription: null, linkedExpenseId: null, name: "Диван", notes: null, paidAmountMinor: 0, paymentStatus: "unpaid", plannedDeliveryOn: null, projectId: project.id, remainingAmountMinor: 250000, supplierName: "Фабрика", updatedAt: new Date().toISOString(), version: 1 }] },
    }).as("contextMaterials");

    cy.visit(`/account/projects/${project.id}/materials`);
    cy.wait(["@contextMaterials", "@contextExpenses"]);
    cy.get(`[data-cy="discuss-material-${materialId}"]`).click();
    cy.location("pathname").should("eq", `/account/projects/${project.id}/chat/${materialChatId}`);
    cy.wait("@contextChat");
    cy.get('[data-cy="project-chat-page"]').should("contain.text", "Обсуждение дивана");

    cy.visit(`/account/projects/${project.id}/finances`);
    // Production performs one load; development Strict Mode may perform an
    // additional aborted load. One completed response is the stable contract.
    cy.wait("@contextExpenses");
    cy.get('[data-cy="project-expenses-page"]').should("be.visible");
    cy.get(`[data-cy="discuss-expense-${expenseId}"]`).click();
    cy.location("pathname").should("eq", `/account/projects/${project.id}/chat/${expenseChatId}`);
    cy.wait("@contextChat");
    cy.get('[data-cy="project-chat-page"]').should("contain.text", "Оплата светильников");
  });

  it("показывает понятную ошибку загрузки и повторяет запрос", () => {
    cy.intercept("GET", "/api/v1/me", { statusCode: 200, body: { id: "admin-1", login: "svetaZZZ", globalRole: "super_admin" } }).as("accessCheck");
    let shouldFail = true;
    cy.intercept("GET", "/api/v1/projects?pageSize=50", (request) => {
      if (shouldFail) request.reply({ statusCode: 503, body: { error: { message: "Сервис проектов временно недоступен" } } });
      else request.reply({ statusCode: 200, body: { items: [], nextCursor: null, hasMore: false } });
    }).as("projects");
    cy.visit("/account");
    cy.wait("@projects");
    cy.get('[role="alert"]').should("contain.text", "Сервис проектов временно недоступен");
    cy.then(() => { shouldFail = false; });
    cy.contains("button", "Повторить").click();
    cy.wait("@projects");
    cy.get('[data-cy="projects-empty"]').should("be.visible");
  });

  it("не раскрывает чужой проект при прямом переходе по URL", () => {
    const foreignProjectId = "00000000-0000-0000-0000-000000000999";
    cy.intercept("GET", `/api/v1/projects/${foreignProjectId}`, {
      statusCode: 404,
      body: { error: { code: "project_not_found", message: "Проект не найден" } },
    }).as("foreignProject");
    cy.visit(`/account/projects/${foreignProjectId}`);
    cy.wait("@foreignProject");
    cy.get('[role="alert"]').should("contain.text", "Проект не найден или у вас нет к нему доступа");
    cy.get('[data-cy="project-workspace"]').should("not.exist");
    cy.contains("a", "К проектам").should("have.attr", "href", "/account");
  });

  it("показывает серверный запрет создания проекта без обхода на клиенте", () => {
    cy.intercept("POST", "/api/v1/projects", {
      statusCode: 403,
      body: { error: { code: "project_creation_forbidden", message: "Недостаточно прав для создания проекта" } },
    }).as("deniedProjectCreation");
    cy.visit("/account/projects/new");
    cy.get('[data-cy="project-name"]').type("Закрытый проект");
    cy.get('[data-cy="project-submit"]').click();
    cy.wait("@deniedProjectCreation");
    cy.get('[data-cy="project-form-error"]').should("contain.text", "Недостаточно прав для создания проекта");
    cy.location("pathname").should("eq", "/account/projects/new");
  });

  it("корректно перестраивает кабинет на мобильном экране", () => {
    cy.intercept("GET", "/api/v1/projects?pageSize=50", { statusCode: 200, body: { items: [project], nextCursor: null, hasMore: false } });
    cy.viewport(390, 844);
    cy.visit("/account");
    cy.get('[data-cy="project-grid"]').should("be.visible");
    cy.get('aside[aria-label="Навигация личного кабинета"]').should("be.visible");
    cy.get('[data-cy="create-project-link"]').should("be.visible");
  });

  it("показывает все разделы проекта без длинной горизонтальной прокрутки на планшете", () => {
    cy.intercept("GET", `/api/v1/projects/${project.id}`, { statusCode: 200, body: project });
    cy.intercept("GET", `/api/v1/projects/${project.id}/documents`, { statusCode: 200, body: { canUpload: true, items: [] } });
    cy.viewport(767, 600);
    cy.visit(`/account/projects/${project.id}/documents`);
    cy.get('nav[aria-label="Разделы проекта"]').should("be.visible").and(($navigation) => {
      expect($navigation[0].scrollWidth).to.be.at.most($navigation[0].clientWidth + 1);
    });
    cy.get('[data-cy="project-nav-summary"]').should("be.visible");
    cy.get('[data-cy="project-nav-chat"]').should("be.visible");
    cy.viewport(1024, 600);
    cy.get('[data-cy="project-nav-documents"]').should(($activeItem) => {
      expect($activeItem[0].getBoundingClientRect().height).to.be.at.most(44);
    });
  });

  it("после выхода возвращает пользователя в начало лендинга", () => {
    cy.intercept("GET", "/api/v1/projects?pageSize=50", { statusCode: 200, body: { items: [], nextCursor: null, hasMore: false } }).as("projects");
    cy.intercept("POST", "/api/v1/auth/login", { statusCode: 200, body: { account: { firstName: "Светлана" }, expiresAt: "2026-09-25T00:00:00Z" } }).as("login");
    cy.intercept("POST", "/api/v1/auth/logout", { statusCode: 204 }).as("logout");
    cy.visit("/");
    cy.scrollTo("bottom");
    cy.window().its("scrollY").should("be.greaterThan", 0);
    cy.get('[data-cy="auth-open"]').click();
    cy.get('[data-cy="login-identifier"]').type("sveta.design");
    cy.get('[data-cy="login-password"]').type("Надёжный пароль 2026!", { log: false });
    cy.get('[data-cy="login-submit"]').click();
    cy.wait("@login");
    cy.location("pathname").should("eq", "/account");
    cy.wait("@projects");
    cy.get('aside button').contains("Выйти").click();
    cy.wait("@logout");
    cy.location("pathname").should("eq", "/");
    cy.window().its("scrollY").should("eq", 0);
  });
});

function formatLongDate(date) {
  return new Intl.DateTimeFormat("ru-RU", { day: "numeric", month: "long", weekday: "long", year: "numeric" }).format(date);
}
