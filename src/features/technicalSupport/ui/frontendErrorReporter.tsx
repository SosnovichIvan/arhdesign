"use client";

import { useEffect } from "react";

import { reportFrontendError } from "../api/technicalSupport";

const reported = new Set<string>();

export function FrontendErrorReporter() {
  useEffect(() => {
    function report(message: string) {
      const key = `${window.location.pathname}|${message}`;
      if (reported.has(key)) return;
      reported.add(key);
      void reportFrontendError(message, window.location.pathname).catch(() => undefined);
    }
    function onError(event: ErrorEvent) { report(event.error instanceof Error ? event.error.message : event.message); }
    function onRejection(event: PromiseRejectionEvent) { report(event.reason instanceof Error ? event.reason.message : "Необработанная ошибка приложения"); }
    window.addEventListener("error", onError);
    window.addEventListener("unhandledrejection", onRejection);
    return () => { window.removeEventListener("error", onError); window.removeEventListener("unhandledrejection", onRejection); };
  }, []);
  return null;
}
