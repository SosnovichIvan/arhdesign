import type { Metadata } from "next";

import { AdminUsersPage } from "@/features/accountUsers";

export const metadata: Metadata = { title: "Пользователи — личный кабинет", robots: { index: false, follow: false } };

export default function AccountUsersPage() { return <AdminUsersPage />; }
