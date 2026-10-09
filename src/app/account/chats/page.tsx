import type { Metadata } from "next";

import { GlobalChatsPage } from "@/features/accountChats";

export const metadata: Metadata = { title: "Чаты — личный кабинет", robots: { index: false, follow: false } };

export default function ChatsPage() { return <GlobalChatsPage />; }
