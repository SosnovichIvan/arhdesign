"use client";

import { useEffect } from "react";

import { reportFrontendError } from "@/features/technicalSupport/api/technicalSupport";
import { Button } from "@/shared/ui";

export default function AccountError({ error, reset }: Readonly<{ error: Error & { digest?: string }; reset: () => void }>) {
  useEffect(() => { void reportFrontendError(error.message, window.location.pathname, error.digest).catch(() => undefined); }, [error]);
  return <div className="mx-auto max-w-2xl border border-red-700/40 bg-surface p-8" role="alert"><p className="text-xs font-semibold uppercase tracking-[0.14em] text-action">Техническая ошибка</p><h1 className="mt-3 font-display text-4xl">Раздел временно не открылся</h1><p className="mt-4 text-sm leading-6 text-secondary">Технический администратор уже получил обезличенные сведения об ошибке. Попробуйте ещё раз.</p><Button className="mt-7" onClick={reset}>Повторить</Button></div>;
}
