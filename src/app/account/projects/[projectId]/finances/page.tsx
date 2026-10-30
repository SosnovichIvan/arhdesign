import type { Metadata } from "next";

import { ProjectFinancesPage } from "@/features/accountProjects";

export const metadata: Metadata = { title: "Финансы проекта — личный кабинет" };

export default async function AccountProjectFinancesPage({ params }: Readonly<{ params: Promise<{ projectId: string }> }>) {
  const { projectId } = await params;
  return <ProjectFinancesPage projectId={projectId} />;
}
