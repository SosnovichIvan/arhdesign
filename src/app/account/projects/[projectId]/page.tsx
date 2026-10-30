import type { Metadata } from "next";

import { ProjectWorkspacePage } from "@/features/accountProjects";

export const metadata: Metadata = { title: "Проект — личный кабинет" };

export default async function AccountProjectPage({ params }: Readonly<{ params: Promise<{ projectId: string }> }>) {
  const { projectId } = await params;
  return <ProjectWorkspacePage projectId={projectId} />;
}
