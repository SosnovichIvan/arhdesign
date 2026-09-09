"use client";

import { useRouter } from "next/navigation";

export function BackButton({ fallbackHref, label = "Назад" }: { fallbackHref: string; label?: string }) {
  const router = useRouter();

  function goBack() {
    const savedReturn = window.sessionStorage.getItem("arhdesign:project-return");
    const expectedTarget = window.location.pathname;

    if (savedReturn) {
      try {
        const { target } = JSON.parse(savedReturn) as { target?: string };
        if (target === expectedTarget) {
          window.sessionStorage.removeItem("arhdesign:project-return");
          router.back();
          return;
        }
      } catch {
        window.sessionStorage.removeItem("arhdesign:project-return");
      }
    }

    if (window.history.length === 1) {
      router.push(fallbackHref);
      return;
    }

    if (window.history.length > 1 && document.referrer.startsWith(window.location.origin)) {
      router.back();
      return;
    }
    router.push(fallbackHref);
  }

  return <button aria-label={label} className="inline-flex min-h-11 items-center gap-2 text-xs font-semibold tracking-[0.1em] text-primary uppercase hover:text-action" onClick={goBack} type="button"><span aria-hidden="true">←</span>{label}</button>;
}
