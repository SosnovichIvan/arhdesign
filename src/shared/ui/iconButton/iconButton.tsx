import { forwardRef, useId, type ButtonHTMLAttributes, type ReactNode } from "react";

import { cn } from "@/shared/lib";

type IconButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  children: ReactNode;
  tooltip?: string;
  tooltipAlign?: "center" | "end" | "start";
  tooltipPlacement?: "top" | "bottom";
};

export const IconButton = forwardRef<HTMLButtonElement, IconButtonProps>(function IconButton({ "aria-describedby": ariaDescribedBy, children, className, tooltip, tooltipAlign = "center", tooltipPlacement = "top", type = "button", ...props }, ref) {
  const tooltipId = useId();
  const button = (
    <button aria-describedby={tooltip ? [ariaDescribedBy, tooltipId].filter(Boolean).join(" ") : ariaDescribedBy} className={cn("inline-flex size-11 items-center justify-center rounded-full border border-border text-primary transition-colors hover:bg-surface disabled:pointer-events-none disabled:opacity-50", className)} ref={ref} type={type} {...props}>
      {children}
    </button>
  );
  if (!tooltip) return button;
  return (
    <span className="group relative inline-flex">
      {button}
      <span className={cn("pointer-events-none hidden absolute z-50 w-max max-w-[min(12rem,calc(100vw-3rem))] whitespace-normal border border-primary bg-primary px-2.5 py-1.5 text-xs font-medium text-inverse-text opacity-0 shadow-surface transition-opacity group-hover:block group-hover:opacity-100 group-focus-within:block group-focus-within:opacity-100", tooltipPlacement === "bottom" ? "top-full mt-2" : "bottom-full mb-2", tooltipAlign === "center" && "left-1/2 -translate-x-1/2", tooltipAlign === "end" && "right-0", tooltipAlign === "start" && "left-0")} id={tooltipId} role="tooltip">
        {tooltip}
      </span>
    </span>
  );
});
