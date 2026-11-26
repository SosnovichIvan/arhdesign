export const projectTypes = [
  { label: "Дизайн интерьера", value: "interior_design" },
  { label: "Архитектурный проект", value: "architecture" },
  { label: "Авторский надзор", value: "supervision" },
  { label: "Комплектация", value: "procurement" },
] as const;

export function projectTypeLabel(type: string) {
  return projectTypes.find((option) => option.value === type)?.label ?? type;
}
