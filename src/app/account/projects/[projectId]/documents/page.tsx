import type { Metadata } from "next";

import { ProjectDocumentsPage } from "@/features/accountProjects";

export const metadata: Metadata = { title: "Документация проекта — личный кабинет" };

export default async function AccountProjectDocumentsPage({ params }: Readonly<{ params: Promise<{ projectId: string }> }>) {
  const { projectId } = await params;
  return <ProjectDocumentsPage projectId={projectId} />;
}
