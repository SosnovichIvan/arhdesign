const authenticatedAccount = {
  globalRole: "super_admin",
  id: "00000000-0000-0000-0000-000000000001",
  login: "svetaZZZ",
};

beforeEach(() => {
  cy.intercept("GET", "/api/v1/me", {
    body: authenticatedAccount,
    statusCode: 200,
  });
  cy.intercept("GET", "/api/v1/notifications?pageSize=20", {
    body: { hasMore: false, items: [], nextCursor: null, unreadCount: 0 },
    statusCode: 200,
  });
  cy.intercept("GET", "/api/v1/settings", {
    body: { theme: "light" },
    statusCode: 200,
  });
  cy.intercept("PUT", "/api/v1/settings", (request) => {
    request.reply({ body: { theme: request.body.theme }, statusCode: 200 });
  });
});
