import type { ButtonHTMLAttributes } from "react";

import { IconButton } from "@/shared/ui/iconButton";

type CarouselControlProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  direction: "next" | "previous";
};

export function CarouselControl({ "aria-label": ariaLabel, direction, ...props }: CarouselControlProps) {
  const symbol = direction === "next" ? "→" : "←";

  return <IconButton aria-label={ariaLabel ?? (direction === "next" ? "Следующее изображение" : "Предыдущее изображение")} {...props}>{symbol}</IconButton>;
}
