import type { ComponentProps, ElementType } from "react";

import { cn } from "@/shared/lib";

type TypographyProps = ComponentProps<"p"> & {
  as?: ElementType;
  tone?: "primary" | "secondary" | "inverse";
  variant?: "display" | "title" | "body" | "label";
};

const variantClasses = {
  display: "font-display text-5xl leading-[0.95] tablet:text-6xl desktop:text-7xl",
  title: "font-display text-3xl leading-none tablet:text-4xl",
  body: "text-base leading-7",
  label: "text-xs font-medium tracking-[0.16em] uppercase",
};

const toneClasses = {
  primary: "text-primary",
  secondary: "text-secondary",
  inverse: "text-inverse-text",
};

export function Typography({ as: Element = "p", className, tone = "primary", variant = "body", ...props }: TypographyProps) {
  return <Element className={cn(variantClasses[variant], toneClasses[tone], className)} {...props} />;
}
