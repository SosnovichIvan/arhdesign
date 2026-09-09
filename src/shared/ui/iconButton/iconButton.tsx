import { forwardRef, type ButtonHTMLAttributes, type ReactNode } from "react";

import { cn } from "@/shared/lib";

type IconButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  children: ReactNode;
};

export const IconButton = forwardRef<HTMLButtonElement, IconButtonProps>(function IconButton({ children, className, type = "button", ...props }, ref) {
  return (
    <button
      className={cn("inline-flex size-11 items-center justify-center rounded-full border border-border text-primary transition-colors hover:bg-surface disabled:pointer-events-none disabled:opacity-50", className)}
      ref={ref}
      type={type}
      {...props}
    >
      {children}
    </button>
  );
});
