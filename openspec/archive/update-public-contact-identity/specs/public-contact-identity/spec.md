## ADDED Requirements

### Requirement: Публичные данные автора

Figma MUST использовать имя «Полисмакова Светлана» в Header, Footer и блоках автора всех шести базовых экранов Concept A.

#### Scenario: Проверка имени

- **WHEN** пользователь открывает Desktop, Tablet или Mobile в Light или Dark
- **THEN** имя читаемо, не обрезано и не накладывается на соседние элементы.

### Requirement: Публичные контакты

Figma MUST использовать телефон `+7 993 335-37-75` и email `svetlana@polismakova.ru` во всех Footer Concept A.

#### Scenario: Проверка контактов

- **WHEN** пользователь открывает Footer любого базового экрана
- **THEN** отображаются только подтверждённые телефон и email с корректным контрастом для выбранной темы.
