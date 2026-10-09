import { Select, type SelectOption } from "@/shared/ui";

const roles = [
  { value: "customer", label: "Заказчик" },
  { value: "designer", label: "Дизайнер" },
  { value: "foreman", label: "Прораб" },
  { value: "architect", label: "Архитектор" },
] as const satisfies readonly [SelectOption, ...SelectOption[]];

type RoleSelectProps = {
  onChange: (value: string) => void;
  value: string;
};

export function RoleSelect({ onChange, value }: RoleSelectProps) {
  return <Select dataCy="register-role" label="Роль" onValueChange={onChange} options={roles} required value={value} />;
}
