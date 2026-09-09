## Interaction

`ProjectCarousel` хранит время последнего ручного действия. Автопереключение запускается раз в 5 000 ms только после того, как с последнего действия прошло не менее 10 000 ms. К ручным действиям относятся стрелки, thumbnails и клавиатурные команды. Это сохраняет управление пользователя и не требует глобального таймера.

Header использует hash-ссылки на уникальные `id` landing-секций; CSS `scroll-behavior: smooth` обеспечивает нативную плавную прокрутку с keyboard-навигацией и без JavaScript dependency. На вложенных routes ссылки должны возвращать на соответствующий hash landing.

## Visual system

Favicon создаётся как оптимизированный SVG с монограммой «ПС», согласованной с выбранным Author sign. Next.js metadata подключает его как icon. В Figma добавляется тот же знак в отдельной favicon-рамке и фиксируется в design evidence.

Footer использует semantic tokens: его Light и Dark variants должны соответствовать Figma, а не инвертировать поверхность автоматически. Проверка охватывает Desktop/Tablet/Mobile × Light/Dark.

## Verification

Playwright проверяет плавные hash-переходы, наличие всех sections и доступность Back. Unit-тесты с fake timers проверяют 5-second cadence и 10-second idle restart. Favicon и Footer проходят визуальную сверку с Figma; Figma action требует доступ к рабочему файлу.
