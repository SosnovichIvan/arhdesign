import { forwardRef, type InputHTMLAttributes } from "react";

import { cn } from "@/shared/lib";

export const Input = forwardRef<HTMLInputElement, InputHTMLAttributes<HTMLInputElement>>(function Input(
  { className, type = "text", ...props },
  ref,
) {
  return (
    <input
      className={cn("min-h-12 w-full rounded-none border-b border-border bg-transparent px-3 py-3 text-base text-primary transition-colors placeholder:text-secondary placeholder:opacity-100 hover:border-action focus:border-field-focus focus:outline-none focus:ring-0 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50 aria-[invalid=true]:border-red-700", className)}
      ref={ref}
      type={type}
      {...props}
    />
  );
});
