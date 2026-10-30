"use client";

import { useEffect } from "react";

export default function VerifyEmailPage() {
  useEffect(() => { window.location.replace(`/?auth=verify${window.location.hash}`); }, []);
  return <main className="grid min-h-screen place-items-center bg-background px-6 text-primary"><p role="status">Проверяем ссылку подтверждения…</p></main>;
}
