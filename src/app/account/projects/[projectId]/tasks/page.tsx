import type { Metadata } from "next";

import { ProjectTasksPage } from "@/features/accountProjects";

export const metadata: Metadata = { title: "Задачи проекта — личный кабинет" };

export default async function TasksRoute({ params }: Readonly<{ params: Promise<{ projectId: string }> }>) {
  const { projectId } = await params;
  return <ProjectTasksPage projectId={projectId} />;
}
