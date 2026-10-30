import type { Metadata } from "next";

import { CreateProjectForm } from "@/features/accountProjects";

export const metadata: Metadata = { title: "Новый проект", robots: { index: false, follow: false } };

export default function NewProjectPage() {
  return <CreateProjectForm />;
}
