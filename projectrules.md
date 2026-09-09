# arhDesign — Правила проекта

## Проект

**Название:** arhDesign  
**Репозиторий:** https://github.com/SosnovichIvan/arhdesign  
**Локальный путь:** `/Users/ivansosnovich/Documents/codex/arhdesign`

**AI workspace:** [`.ai/`](.ai/) — локальные skills, контекст и правила реализации/review.

## Figma

**Папка:** https://www.figma.com/files/team/916052876586830524/folder/644821162?fuid=916052870552793231

**Рабочий design-файл:** https://www.figma.com/design/7CasuBb4xJjm0UYTOGZElV

**Team ID:** 916052876586830524
**Folder/Project ID:** 644821162

---

## Агент — системные инструкции

### OpenSpec — обязательный рабочий контур

Любая задача в этом репозитории ведётся через OpenSpec, включая дизайн, frontend, review, контентные и инфраструктурные изменения.

1. Создать или обновить change в `openspec/changes/<change-id>/`: `proposal.md`, capability specs, `design.md` (если есть архитектурное или дизайн-решение) и `tasks.md` с проверяемыми результатами.
2. Провалидировать change средствами OpenSpec и устранить найденные несоответствия.
3. Получить явное подтверждение принятия change от пользователя.
4. Перед Apply каждой принятой OpenSpec-задачи явно активировать `$execution-state` и выполнить routing c `source=openspec`. Только `passthrough`, выбранный route для короткой связной задачи, разрешает продолжить без state-файла; для `lite`/`reset` использовать `statectl` из `.ai/skills/execution-state/`.
5. Только после принятия и routing выполнять задачи из `tasks.md`; перед отметкой задачи запускать релевантные проверки. `tasks.md` — единственный task ledger: worker не меняет checkbox напрямую, а controller принимает evidence и обновляет статус.
6. До approval определить context-impact change: какие факты `.ai/context/` могут стать неактуальны.
7. После apply обновить затронутый контекст, запустить `bash .ai/scripts/validate-context.sh` и выполнить semantic review с OpenSpec, Figma, OpenAPI или кодом.
8. Не заменять OpenSpec задачей в чате, TODO в коде или устным описанием. Срочная правка всё равно оформляется как минимальный change до начала работы.

Исключение возможно только по явному письменному указанию пользователя для конкретной задачи.

### Обязательные скиллы

Перед работой загружать соответствующий скилл:

| Задача | Скилл |
|--------|-------|
| UI/UX дизайн, Canvas, `_design.html`, mockups | `/home/user/.agents/skills/design-agent/SKILL.md` |
| Landing page, визуальные макеты | `/home/user/.agents/skills/ui-ux-pro-max/SKILL.md` |
| Компоненты shadcn/ui | `/home/user/.agents/skills/shadcn-ui/SKILL.md` |
| SVG иконки, диаграммы | `/home/user/.agents/skills/svg-precision/SKILL.md` |
| React/Next.js оптимизация | `/home/user/.agents/skills/vercel-react-best-practices/SKILL.md` |
| Slidev презентации | `/home/user/.agents/skills/slidev/SKILL.md` |
| Компонентная архитектура | `/home/user/.agents/skills/vercel-composition-patterns/SKILL.md` |
| Анализ кода, LSP навигация | `/home/user/.agents/skills/lsp-navigation/SKILL.md` |
| AST поиск паттернов | `/home/user/.agents/skills/ast-grep/SKILL.md` |

### Порядок работы с Figma

1. **Получить контекст файла:**
   ```
   figma_get_design_context(fileKey="...", maxResponseChars=8000)
   ```

2. **Найти нужный компонент/фрейм:**
   ```
   figma_find_nodes_by_name(fileKey="...", query="имя")
   ```

3. **Получить детали узла:**
   ```
   figma_get_implementation_context(fileKey="...", nodeId="...")
   ```

4. **Рендер для проверки:**
   ```
   figma_render_nodes(fileKey="...", nodeIds=["..."], format="png", scale=2)
   ```

### Структура скиллов

```
.ai/
├── README.md
├── context/                         ← состояние проекта, решения и feedback
└── skills/
    ├── arhdesign-designer/SKILL.md  ← Figma / дизайн-потоки
    ├── arhdesign-frontend/SKILL.md  ← SDD + минимальный FSD для production UI
    ├── arhdesign-backend/SKILL.md   ← Go REST API, OpenAPI и PostgreSQL
    ├── arhdesign-context/SKILL.md   ← поддержание и validation контекста
    ├── execution-state/SKILL.md     ← routing и runtime state для OpenSpec Apply
    └── arhdesign-review/SKILL.md    ← review OpenSpec/FSD/UI
```

Внешние skills из таблицы выше подключаются, когда они доступны в текущем окружении. Локальные skills из `.ai/` задают правила именно этого репозитория и дополняют, а не заменяют `projectrules.md` и OpenSpec.

### Быстрые маркеры вывода

| Маркер | Применение |
|--------|------------|
| `OpenFile: /home/user/...` | Основные файлы проекта |
| `OpenDesign: /home/user/.../*_design.html` | Canvas design файлы |
| `Preview: <port>` | Локальный preview сервер |
| `Addon: ...` | Публикация add-on |

### Рабочий процесс

1. Прочитать `projectrules.md` в начале каждой сессии
2. Прочитать `.ai/context/project-state.md` и относящийся к задаче контекст
3. Создать/обновить и провалидировать OpenSpec change; дождаться его принятия
4. Для Apply запустить `$execution-state` с `source=openspec` и следовать выбранному route; `passthrough` не отменяет OpenSpec-проверки
5. Определить нужные скиллы по задаче в `.ai/skills/` и среди доступных внешних skills
6. Загрузить локальный и/или внешний скилл(ы) через `read` tool
7. Выполнить только принятые задачи OpenSpec с применением правил скилла
8. После существенного изменения обновить документацию и `.ai/context/`
9. Новое пользовательское замечание зафиксировать в `feedback-log.md`; в skill переносить только повторяемое, проверяемое правило

---

## Текущий статус

- [x] Git репозиторий инициализирован
- [x] README.md создан
- [x] Figma проект подключен
- [ ] Первый дизайн-макет создан
