export type Project = {
  description: string;
  image: string;
  images: string[];
  location: string;
  slug: string;
  title: string;
};

const gallery = (name: string, extension: string) => Array.from(
  { length: 10 },
  (_, index) => `/images/projects/${name}/${String(index + 1).padStart(2, "0")}.${name === "contemporary-harmony" && (index === 0 || index === 3) ? "webp" : extension}`,
);
export const projects: Project[] = [
  { slug: "contemporary-harmony", title: "Современная гармония", location: "Квартира от застройщика ПИК", description: "Интерьер с точной планировкой, естественным светом и спокойной палитрой материалов.", image: "/images/projects/contemporary-harmony.webp", images: gallery("contemporary-harmony","jpg") },
  { slug: "atmosphere-of-comfort", title: "Атмосфера уюта", location: "Интерьер частного дома", description: "Тёплый жилой интерьер с тактильными материалами и мягкими сценариями света.", image: "/images/projects/atmosphere-of-comfort.jpg", images: gallery("atmosphere-of-comfort","jpg") },
  { slug: "contemporary-minimalism", title: "Современный минимализм", location: "Интерьер частного дома", description: "Лаконичное пространство, где архитектура поддерживает повседневную жизнь.", image: "/images/projects/contemporary-minimalism.jpg", images: gallery("contemporary-minimalism","jpg") },
];
