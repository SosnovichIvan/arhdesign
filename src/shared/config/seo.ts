import { contact } from "./contact";

export const siteUrl = new URL(process.env.NEXT_PUBLIC_SITE_URL ?? "http://localhost:3000");

export const siteName = "Полисмакова Светлана — архитектура и интерьеры";
export const siteDescription = "Архитектура и интерьеры Светланы Полисмаковой: частные пространства с точной планировкой, естественным светом и спокойной палитрой материалов.";

export const organizationJsonLd = {
  "@context": "https://schema.org",
  "@type": "ProfessionalService",
  name: siteName,
  description: siteDescription,
  email: contact.email,
  telephone: contact.phone,
  url: siteUrl.toString(),
  sameAs: [contact.vkUrl, contact.telegramUrl],
  areaServed: "Москва",
  founder: {
    "@type": "Person",
    name: contact.name,
  },
};
