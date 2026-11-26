import { cloneElement, useId, type ReactElement } from "react";

import { cn } from "@/shared/lib";

type TooltipProps = {
  align?: "center" | "end" | "start";
  children: ReactElement<{ "aria-describedby"?: string }>;
  placement?: "top" | "bottom";
  text: string;
};

export function Tooltip({ align = "center", children, placement = "top", text }: TooltipProps) {
  const id = useId();
  const describedBy = [children.props["aria-describedby"], id].filter(Boolean).join(" ");
  return <span className="group relative inline-flex">
    {cloneElement(children, { "aria-describedby": describedBy })}
    <span className={cn(
      "pointer-events-none hidden absolute z-50 w-max max-w-[min(12rem,calc(100vw-3rem))] whitespace-normal border border-primary bg-primary px-2.5 py-1.5 text-xs font-medium text-inverse-text opacity-0 shadow-surface transition-opacity group-hover:block group-hover:opacity-100 group-focus-within:block group-focus-within:opacity-100",
      placement === "bottom" ? "top-full mt-2" : "bottom-full mb-2",
      align === "center" && "left-1/2 -translate-x-1/2",
      align === "end" && "right-0",
      align === "start" && "left-0",
    )} id={id} role="tooltip">{text}</span>
  </span>;
}
