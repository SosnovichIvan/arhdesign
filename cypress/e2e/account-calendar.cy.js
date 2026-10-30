const projectA = {
  id: "00000000-0000-0000-0000-000000000101",
  items: [
    { createdAt: "2026-09-01T08:00:00Z", description: null, effectiveAt: "2026-10-15T10:00:00Z", endsAt: null, id: "00000000-0000-0000-0000-000000000201", kind: "task", location: null, projectId: "00000000-0000-0000-0000-000000000101", status: "new", title: "Подготовить планы", version: 1 },
    { createdAt: "2026-09-01T08:00:00Z", description: null, effectiveAt: "2026-10-18T12:00:00Z", endsAt: "2026-10-18T13:00:00Z", id: "00000000-0000-0000-0000-000000000202", kind: "meeting", location: "Объект", projectId: "00000000-0000-0000-0000-000000000101", status: null, title: "Встреча на объекте", version: 1 },
  ],
  name: "Полянка",
  plannedFinishOn: "2026-12-20",
  plannedStartOn: "2026-09-01",
  status: "active",
};

const projectB = { id: "00000000-0000-0000-0000-000000000102", items: [], name: "Дом 26", plannedFinishOn: null, plannedStartOn: null, status: "draft" };

function feed(projects) {
  return { hasMoreProjects: false, projects, rangeEnd: "2027-01-01T00:00:00Z", rangeStart: "2026-09-01T00:00:00Z", truncatedEvents: false };
}

describe("Общий календарь проектов", () => {
  beforeEach(() => {
    cy.intercept("GET", "/api/v1/me", { statusCode: 200, body: { id: "admin-1", login: "svetaZZZ", globalRole: "super_admin" } }).as("accessCheck");
    cy.intercept("GET", "/api/v1/settings", { statusCode: 200, body: { theme: "light" } });
  });

  it("показывает только доступные проекты, фильтрует и ведёт к источнику", () => {
    cy.intercept("GET", "/api/v1/calendar?*", (request) => {
      const url = new URL(request.url);
      const selected = url.searchParams.getAll("projectId");
      request.reply({ statusCode: 200, body: feed(selected.length ? [projectA] : [projectA, projectB]) });
    }).as("calendar");

    cy.visit("/account/calendar");
    cy.wait(["@accessCheck", "@calendar"]);
    cy.get('[data-cy="account-nav-календарь"]').should("have.attr", "aria-current", "page");
    cy.get('[data-cy="global-calendar-timeline"]').should("contain.text", "Полянка").and("contain.text", "Дом 26");
    cy.get('[data-cy="global-calendar-page"]').should("not.contain.text", "Секретный чужой проект");
    cy.get('[data-cy^="global-calendar-event-"]').first().should("have.attr", "href").and("include", "/tasks?taskId=");

    cy.get('[data-cy="global-calendar-filter-task"]').click();
    cy.get('[data-cy="global-calendar-timeline"]').should("contain.text", "Подготовить планы").and("not.contain.text", "Встреча на объекте");
    cy.get('[data-cy="global-calendar-display-toggle"]').click();
    cy.get('[data-cy="global-calendar-list"]').should("be.visible");

    cy.get('[data-cy="global-calendar-project-filter"]').click();
    cy.get('[data-cy="global-calendar-project-filter-form"]').contains("label", "Полянка").find('input[type="checkbox"]').check();
    cy.get('[data-cy="global-calendar-project-filter-form"]').contains("button", "Применить").click();
    cy.wait("@calendar").its("request.url").should("include", `projectId=${projectA.id}`);
    cy.get('[aria-label="Применено фильтров проектов: 1"]').should("be.visible");
    cy.get('[data-cy="global-calendar-list"]').should("contain.text", "Полянка").and("not.contain.text", "Дом 26");
  });

  it("показывает понятное пустое состояние без проектов", () => {
    cy.intercept("GET", "/api/v1/calendar?*", { statusCode: 200, body: feed([]) }).as("calendar");
    cy.visit("/account/calendar");
    cy.wait("@calendar");
    cy.get('[data-cy="global-calendar-empty"]').should("contain.text", "Проектов пока нет");
    cy.get('[data-cy="global-calendar-empty"]').contains("a", "Создать проект").should("have.attr", "href", "/account/projects/new");
  });
});
