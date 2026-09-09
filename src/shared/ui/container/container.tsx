import type { ComponentProps } from "react";

import { cn } from "@/shared/lib";

type ContainerProps = ComponentProps<"div">;

export function Container({ className, ...props }: ContainerProps) {
  return <div className={cn("mx-auto w-full max-w-[1440px] px-6 tablet:px-12 desktop:px-24", className)} {...props} />;
}
