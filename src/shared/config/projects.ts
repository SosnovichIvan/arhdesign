export type Project = {
  description: string;
  details?: string[];
  image: string;
  images: string[];
  location: string;
  slug: string;
  title: string;
};

const gallery = (name: string, extension: string, length = 10) => Array.from(
  { length },
  (_, index) => `/images/projects/${name}/${String(index + 1).padStart(2, "0")}.${name === "contemporary-harmony" && (index === 0 || index === 3) ? "webp" : extension}`,
);
export const projects: Project[] = [
  {
    slug: "cozy-at-height",
    title: "Уют на высоте",
    location: "Интерьер квартиры",
    description: "Спокойный интерьер в бежевых и молочных тонах с продуманными световыми сценариями.",
    details: [
      "Квартира, где каждое пространство построено на спокойных бежевых и молочных тонах. Гостиная с обеденной зоной объединена большими окнами, бежевыми шторами в пол и светлым паркетом. Главные акценты здесь — подвесная люстра из матовых белых шаров и LED-кольца под потолком.",
      "В спальне стена с объёмной фактурной панелью и мягкой подсветкой становится главным элементом. Прикроватные бра и мягкое постельное бельё создают атмосферу отдыха.",
      "В детской или комнате для подростка есть всё необходимое: встроенные шкафы с застеклёнными секциями, письменный стол с деревянной столешницей и светлая подсветка на потолке.",
      "Дизайн держится на сочетании тёплых оттенков, текстур и продуманного света, поэтому квартира выглядит уютно в любое время суток.",
    ],
    image: "/images/projects/cozy-at-height/02.webp",
    images: gallery("cozy-at-height", "webp", 19),
  },
  { slug: "contemporary-harmony", title: "Современная гармония", location: "Квартира от застройщика ПИК", description: "Интерьер с точной планировкой, естественным светом и спокойной палитрой материалов.", image: "/images/projects/contemporary-harmony.webp", images: gallery("contemporary-harmony","jpg") },
  { slug: "atmosphere-of-comfort", title: "Атмосфера уюта", location: "Интерьер частного дома", description: "Тёплый жилой интерьер с тактильными материалами и мягкими сценариями света.", image: "/images/projects/atmosphere-of-comfort.jpg", images: gallery("atmosphere-of-comfort","jpg") },
  { slug: "contemporary-minimalism", title: "Современный минимализм", location: "Интерьер частного дома", description: "Лаконичное пространство, где архитектура поддерживает повседневную жизнь.", image: "/images/projects/contemporary-minimalism.jpg", images: gallery("contemporary-minimalism","jpg") },
];
