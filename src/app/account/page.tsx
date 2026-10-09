import type { Metadata } from "next";

import { AccountProjectsPage } from "@/features/accountProjects";

export const metadata: Metadata = { title: "Проекты — личный кабинет", robots: { index: false, follow: false } };

export default function AccountPage() {
  return <AccountProjectsPage />;
}
