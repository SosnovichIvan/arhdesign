## ADDED Requirements

### Requirement: Изолированный список проектов

Система MUST возвращать authenticated пользователю только проекты, где он имеет
active membership, а super-admin MAY видеть все проекты; прямой URL MUST
проверять тот же object scope.

#### Scenario: Доступный список и чужой URL

- **WHEN** пользователь открывает список и затем запрашивает project без membership
- **THEN** список содержит только разрешённые проекты, а прямой запрос получает согласованный `403/404` без раскрытия project data.

### Requirement: Атомарное создание проекта заказчиком

Система MUST в одной транзакции создать project, назначить обычного creator
единственным active customer и project-admin и записать audit.

#### Scenario: Обычный пользователь создаёт проект

- **WHEN** authenticated обычный пользователь отправляет валидную форму проекта
- **THEN** project и обе его роли доступны сразу либо транзакция полностью откатывается.

### Requirement: Проект супер-администратора без заказчика

Система MUST разрешать только super-admin создать project без customer и MUST
позволять позднее назначить ровно одного active customer без переписывания
creator/history.

#### Scenario: Создание без заказчика

- **WHEN** super-admin создаёт project без выбранного customer
- **THEN** project сохраняется с nullable customer, без фиктивного пользователя, и остаётся управляемым super-admin.

#### Scenario: Позднее назначение заказчика

- **WHEN** super-admin назначает active пользователя customer проекта без customer
- **THEN** создаётся единственная active customer membership, история до назначения сохраняется и повтор не создаёт дубликат.

