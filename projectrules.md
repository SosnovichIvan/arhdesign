# arhDesign — Правила проекта

## Проект

**Название:** arhDesign  
**Репозиторий:** https://github.com/SosnovichIvan/arhdesign  
**Локальный путь:** `/home/user/arhDesign`

## Figma

**Ссылка:** https://www.figma.com/files/team/1091268117334409095/project/623270120?fuid=1091268106606577791

**Team ID:** 1091268117334409095  
**Project ID:** 623270120

---

## Агент — системные инструкции

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
/home/user/.agents/skills/
├── design-agent/
│   └── SKILL.md          ← основной дизайн-скилл
├── ui-ux-pro-max/
│   └── SKILL.md          ← UI/UX шаблоны
├── shadcn-ui/
│   └── SKILL.md          ← компоненты
├── svg-precision/
│   └── SKILL.md          ← SVG
├── vercel-react-best-practices/
│   └── SKILL.md          ← React оптимизация
├── slidev/
│   └── SKILL.md          ← презентации
├── vercel-composition-patterns/
│   └── SKILL.md          ← архитектура компонентов
├── lsp-navigation/
│   └── SKILL.md          ← навигация по коду
└── ast-grep/
    └── SKILL.md          ← семантический поиск
```

### Быстрые маркеры вывода

| Маркер | Применение |
|--------|------------|
| `OpenFile: /home/user/...` | Основные файлы проекта |
| `OpenDesign: /home/user/.../*_design.html` | Canvas design файлы |
| `Preview: <port>` | Локальный preview сервер |
| `Addon: ...` | Публикация add-on |

### Рабочий процесс

1. Прочитать `projectrules.md` в начале каждой сессии
2. Определить нужные скиллы по задаче
3. Загрузить скилл(ы) через `read` tool
4. Выполнить задачу с применением правил скилла
5. Обновить документацию при существенных изменениях

---

## Текущий статус

- [x] Git репозиторий инициализирован
- [x] README.md создан
- [ ] Figma проект подключен
- [ ] Первый дизайн-макет создан
