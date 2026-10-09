import type { Metadata } from "next";

import { ProjectChatPage } from "@/features/accountProjects";

export const metadata: Metadata = { title: "Обсуждение проекта — личный кабинет" };

export default async function AccountProjectContextChatPage({ params }: Readonly<{ params: Promise<{ chatId: string; projectId: string }> }>) {
  const { chatId, projectId } = await params;
  return <ProjectChatPage chatId={chatId} projectId={projectId} />;
}
