## 1. Контент и структура

- [x] 1.1 Проверить и зафиксировать 30 imageHash в требуемом порядке из трёх существующих media-групп; результат: порядок не смешивает проекты.
- [x] 1.2 Создать интерактивный набор состояний Hero-карусели для 30 кадров; результат: каждый кадр использует `FILL` и занимает 616 × 632 px.

## 2. Prototype и темы

- [x] 2.1 Настроить after-timeout переходы 3 000 ms с dissolve 300 ms и loop 30 → 01; результат: последовательность непрерывна.
- [x] 2.2 Разместить Hero-карусель в Desktop Light и Desktop Dark; результат: обе темы используют один порядок фотографий без изменения Hero-copy.

## 3. Проверка и документация

- [x] 3.1 Проверить первый, десятый, одиннадцатый, двадцатый, двадцать первый и тридцатый кадры; результат: соблюдён порядок проектов и loop.
- [x] 3.2 Обновить контекст и журнал Figma; результат: набор фото, порядок и интервал доступны для реализации.

## 4. Адаптивное расширение

- [x] 4.1 Создать responsive component set `Hero Carousel Media / Tablet` из 30 кадров 720 × 436 px; результат: after-timeout и порядок идентичны Desktop.
- [x] 4.2 Создать responsive component set `Hero Carousel Media / Mobile` из 30 кадров 342 × 360 px; результат: after-timeout и порядок идентичны Desktop.
- [x] 4.3 Разместить adaptive-инстансы в Tablet Light/Dark и Mobile Light/Dark; результат: все шесть базовых Hero используют карусель без изменения copy и темы.
- [x] 4.4 Проверить рендерами все четыре адаптивных Hero и обновить контекст; результат: нет искажений кадра и данных о статичном Hero.

## 5. Видимые controls Hero-карусели

- [x] 5.1 Добавить в 30 Desktop‑состояний preview‑filmstrip, счётчик и кнопки `←`/`→` по стилю case-study; результат: Hero явно воспринимается как карусель.
- [x] 5.2 Настроить ручные переходы previous/next в цикле 01 ↔ 30; результат: кнопки и after-timeout не конфликтуют.
- [x] 5.3 Адаптировать preview‑filmstrip и controls для Tablet/Mobile; результат: миниатюры и кнопки не закрывают ключевую часть изображения.
- [x] 5.4 Проверить Light/Dark‑контраст overlays и все варианты Hero; результат: controls читаемы в обеих темах и на фото разной яркости.

## 6. Компонентная структура

- [x] 6.1 Переименовать и оформить текущий Desktop‑набор как `Hero Carousel / Viewport=Desktop`; результат: instances Light/Dark связаны с компонентом, включающим все controls и prototype.
- [x] 6.2 Создать `Hero Carousel / Viewport=Tablet` и `Hero Carousel / Viewport=Mobile`; результат: каждый viewport имеет самостоятельный component set с 30 variants.
- [x] 6.3 Заменить все локальные Hero‑media на instances соответствующих наборов; результат: во всех шести Concept A не дублируются изображения, previews и кнопки.
- [x] 6.4 Провести naming и instance audit; результат: components и instances используют единый Figma‑нейминг, а все связи mainComponent валидны.

## 7. Единый компонент для галерей страниц «Проект»

- [x] 7.1 Выделить из Hero reusable `Carousel Controls` с property `Viewport`; результат: подложка, кнопки, счётчик и accent-stroke — единый визуальный источник для Hero и проектов.
- [x] 7.2 Создать `Project Carousel` из 30 variants (3 проекта × 10 кадров) с property `Viewport=Desktop|Tablet|Mobile`; результат: нативные размеры 786 × 540 px, 672 × 420 px и 342 × 360 px не создают отдельного визуального стиля.
- [x] 7.3 Настроить `01 / 10`, preview-filmstrip, manual previous/next и after-timeout 3 000 ms / dissolve 300 ms в loop внутри каждого проекта; результат: проекты не смешиваются между собой.
- [x] 7.4 Заменить локальные `carousel-main-image` во всех 18 экранах «Проект» на instances соответствующих component set; результат: Desktop, Tablet и Mobile × Light/Dark используют единый компонент.
- [x] 7.5 Проверить рендерами Desktop/Tablet/Mobile и провести instance audit; результат: нет локальных каруселей, controls читаемы и media не искажена.
