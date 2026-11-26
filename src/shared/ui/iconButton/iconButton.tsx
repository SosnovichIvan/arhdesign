import { forwardRef, type ButtonHTMLAttributes, type ReactNode } from "react";

import { cn } from "@/shared/lib";
import { Tooltip } from "@/shared/ui/tooltip";

type IconButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  children: ReactNode;
  tooltip?: string;
  tooltipAlign?: "center" | "end" | "start";
  tooltipPlacement?: "top" | "bottom";
};

export const IconButton = forwardRef<HTMLButtonElement, IconButtonProps>(function IconButton({ children, className, tooltip, tooltipAlign, tooltipPlacement, type = "button", ...props }, ref) {
  const button = (
    <button className={cn("inline-flex size-11 items-center justify-center rounded-full border border-border text-primary transition-colors hover:bg-surface disabled:pointer-events-none disabled:opacity-50", className)} ref={ref} type={type} {...props}>
      {children}
    </button>
  );
  return tooltip ? <Tooltip align={tooltipAlign} placement={tooltipPlacement} text={tooltip}>{button}</Tooltip> : button;
});
