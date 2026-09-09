import { forwardRef, type TextareaHTMLAttributes } from "react";

import { cn } from "@/shared/lib";

export const Textarea = forwardRef<HTMLTextAreaElement, TextareaHTMLAttributes<HTMLTextAreaElement>>(function Textarea(
  { className, ...props },
  ref,
) {
  return (
    <textarea
      className={cn("min-h-28 w-full resize-y rounded-none border-b border-border bg-transparent px-0 py-3 text-base text-primary placeholder:text-secondary aria-[invalid=true]:border-red-700", className)}
      ref={ref}
      {...props}
    />
  );
});
