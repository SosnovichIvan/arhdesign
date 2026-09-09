## 1. Design evidence

- [x] 1.1 Добавить favicon с монограммой «ПС» в Figma и проверить Footer Light/Dark во всей responsive matrix; результат: Figma asset `566:2`, local render evidence без blocking deviations.

## 2. Landing interaction and content

- [x] 2.1 Реализовать idle-aware Hero: 5 s auto-advance, 10 s restart delay после каждого ручного действия; результат: unit tests с fake timers и keyboard/button coverage.
- [x] 2.2 Реализовать favicon и Next.js metadata; результат: icon обнаруживается браузером и не ухудшает metadata audit.
- [x] 2.3 Восстановить Services, Process, Profile и Contact CTA на landing с якорными id; результат: Desktop/Tablet/Mobile × Light/Dark без overflow.
- [x] 2.4 Сделать Header navigation плавной и route-safe; результат: Playwright подтверждает корректный target и keyboard access.
- [x] 2.5 Проверить и исправить видимое действие «Назад» на каждой case-study странице; результат: при переходе из landing возвращает в историю, при прямом открытии — в каталог; accessible name «Назад».
- [x] 2.6 Привести Footer к Figma в Light/Dark; результат: visual review и no light surface in Dark.

## 3. Acceptance

- [x] 3.1 Прогнать lint, typecheck, coverage, e2e/axe и visual Figma review; результат: green reports и evidence in `docs/reviews/final-semantic-review.md`.
- [x] 3.2 Обновить context и validation; результат: `bash .ai/scripts/validate-context.sh` проходит.
