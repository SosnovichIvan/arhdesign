import { forwardRef, type InputHTMLAttributes } from "react";

import { cn } from "@/shared/lib";

export const Input = forwardRef<HTMLInputElement, InputHTMLAttributes<HTMLInputElement>>(function Input(
  { className, type = "text", ...props },
  ref,
) {
  return (
    <input
      className={cn("min-h-12 w-full rounded-none border-b border-border bg-transparent px-0 py-3 text-base text-primary placeholder:text-secondary aria-[invalid=true]:border-red-700", className)}
      ref={ref}
      type={type}
      {...props}
    />
  );
});
