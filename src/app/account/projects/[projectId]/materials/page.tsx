import type { Metadata } from "next";

import { ProjectMaterialsPage } from "@/features/accountProjects";

export const metadata: Metadata = { title: "Материалы проекта — личный кабинет" };

export default async function AccountProjectMaterialsPage({ params }: Readonly<{ params: Promise<{ projectId: string }> }>) {
  const { projectId } = await params;
  return <ProjectMaterialsPage projectId={projectId} />;
}
