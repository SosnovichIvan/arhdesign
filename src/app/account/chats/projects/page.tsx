import type { Metadata } from "next";

import { ProjectChatDirectory } from "@/features/accountProjects/ui/projectChatDirectory";

export const metadata: Metadata = { title: "Чаты проектов — личный кабинет", robots: { index: false, follow: false } };

export default function ProjectChatsPage() {
  return <ProjectChatDirectory />;
}
