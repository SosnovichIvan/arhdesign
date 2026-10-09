import type { Metadata } from "next";
import { redirect } from "next/navigation";

export const metadata: Metadata = {
  title: "Регистрация",
  description: "Регистрация в личном кабинете arhDesign.",
  robots: { index: false, follow: false },
};

export default function RegisterPage() {
  redirect("/?auth=register");
}
