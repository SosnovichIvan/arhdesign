import type { Metadata } from "next";

import { ProjectChatPage } from "@/features/accountProjects";

export const metadata: Metadata = { title: "Чат проекта — личный кабинет" };

export default async function AccountProjectChatPage({ params }: Readonly<{ params: Promise<{ projectId: string }> }>) {
  const { projectId } = await params;
  return <ProjectChatPage projectId={projectId} />;
}
