# API contracts

- Источник истины — `openapi/`.
- Изменение API начинается с OpenAPI и OpenSpec, затем запускается генерация Go и TypeScript типов.
- Контракт содержит request/response, ошибки, валидацию и security scheme.
- Сгенерированный код не редактируется вручную; доменные модели не дублируют API DTO без необходимости.
- Breaking API change требует отдельного OpenSpec approval и версии контракта.
