describe("public portfolio", () => {
  it("opens the new project from the landing page", () => {
    cy.visit("/");
    cy.contains("article", "Уют на высоте").within(() => {
      cy.contains("Интерьер квартиры").should("be.visible");
      cy.contains("a", "Подробнее").click();
    });

    cy.location("pathname").should("eq", "/projects/cozy-at-height");
    cy.contains("h1", "Уют на высоте").should("be.visible");
    cy.contains("01 / 19").should("be.visible");
    cy.contains("Дизайн держится на сочетании тёплых оттенков").should("be.visible");
  });

  it("keeps the project page usable on a mobile viewport", () => {
    cy.viewport(390, 844);
    cy.visit("/projects/cozy-at-height");

    cy.contains("h1", "Уют на высоте").should("be.visible");
    cy.get('[aria-label="Галерея: Уют на высоте"]').should("be.visible");
    cy.get('[aria-label="Показать кадр 19"]').should("exist");
    cy.document().then((document) => {
      expect(document.documentElement.scrollWidth).to.be.at.most(390);
    });
  });
});
