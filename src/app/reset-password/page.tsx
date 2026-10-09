"use client";

import { useEffect } from "react";

export default function ResetPasswordPage() {
  useEffect(() => { window.location.replace(`/?auth=reset${window.location.hash}`); }, []);
  return <main className="grid min-h-screen place-items-center bg-background px-6 text-primary"><p role="status">Открываем форму восстановления доступа…</p></main>;
}
