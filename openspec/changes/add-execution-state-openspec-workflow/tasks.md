## 1. Install and govern

- [x] 1.1 Перенести полный release `execution-state` в `.ai/skills/execution-state/` без изменения исходной schema; проверка: `SKILL.md`, `references/`, `scripts/statectl.py`, adapters и release manifest присутствуют.
- [x] 1.2 Обновить `projectrules.md` и `.ai/README.md`: каждый OpenSpec Apply начинается с явного `$execution-state` routing, а `passthrough` допустим только по решению route; проверка: документация различает OpenSpec ledger, runtime state и context.
- [x] 1.3 Добавить `.execution-state/` в `.gitignore` и запретить secrets/PII в runtime state; проверка: `git check-ignore .execution-state/state.json` и security review.

## 2. Validate

- [x] 2.1 Проверить copied skill: `statectl version`, OpenSpec route и `statectl validate` на создаваемом test state; проверка: команды проходят без model request и без изменения активного OpenSpec task.
- [x] 2.2 Обновить `.ai/context/` и выполнить `bash .ai/scripts/validate-context.sh`; проверка: context review подтверждает новую execution policy.
