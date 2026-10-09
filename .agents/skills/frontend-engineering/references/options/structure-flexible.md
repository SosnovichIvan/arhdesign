# Flexible logic-based structure

Используй эту структуру, когда проекту нужны ясные ownership boundaries без
фиксированного набора FSD-слоёв. Имена адаптируй к framework и существующему коду.

```text
src/
├── app/                 # startup, providers, router, global configuration
├── routes-or-pages/     # route entrypoints and scenario composition
├── features/            # product capabilities grouped by domain or use case
│   └── <feature>/
│       ├── ui/
│       ├── model/
│       ├── api/
│       └── lib/
└── shared/              # domain-neutral reusable foundations
    ├── ui/
    ├── api/
    ├── lib/
    └── config/
```

Не создавай все папки заранее. Модуль появляется только с первым реальным
файлом и владельцем.

## Правила зависимостей

- `app` и routes/pages могут компоновать features и shared.
- Feature не импортирует route/page и не знает о startup приложения.
- Между features предпочитай композицию наверху или небольшой явный public
  contract. Не создавай скрытые циклические зависимости.
- `shared` не импортирует product features и routes.
- Внешний импорт модуля идёт через его минимальный public entrypoint, если такой
  подход принят в проекте.

## Как выбрать место

- Route-specific UI/data logic остаётся рядом с route, пока не появится второй
  независимый потребитель.
- Код одной пользовательской возможности размещается вместе независимо от
  технического типа файла.
- Внутри крупной feature дели по ответственности (`ui`, `model`, `api`, `lib`),
  но для маленькой feature допускай плоскую папку.
- Переноси в shared только стабильное domain-neutral поведение; совпадение двух
  фрагментов кода само по себе не требует abstraction.
- Если модуль стал трудно обозримым, дели его по use case/subdomain, а не на
  глобальные папки всех hooks, services или types проекта.
