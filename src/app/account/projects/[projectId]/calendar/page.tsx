import type { Metadata } from "next";

import { ProjectCalendarPage } from "@/features/accountProjects";

export const metadata: Metadata = { title: "Календарь проекта — личный кабинет" };

export default async function ProjectCalendarRoute({ params }: Readonly<{ params: Promise<{ projectId: string }> }>) {
  const { projectId } = await params;
  return <ProjectCalendarPage projectId={projectId} />;
}
