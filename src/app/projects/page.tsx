import type { Metadata } from "next";

import { BackButton } from "@/shared/components/backButton";
import { ProjectCard } from "@/shared/components/projectCard";
import { SiteFooter } from "@/shared/components/siteFooter";
import { projects } from "@/shared/config";
import { Container, Typography } from "@/shared/ui";

export const metadata: Metadata = {
  alternates: { canonical: "/projects" },
  description: "Реализованные интерьеры Светланы Полисмаковой: квартиры и частные дома.",
  title: "Проекты",
};

export default function ProjectsPage() { return <><main className="py-16 tablet:py-24"><Container><div className="flex items-center justify-between gap-4"><Typography as="h1" variant="display">Проекты</Typography><BackButton fallbackHref="/#projects" label="Назад" /></div><div className="mt-12 grid gap-6 tablet:grid-cols-2 desktop:grid-cols-3">{projects.map((project) => <ProjectCard key={project.slug} project={project} />)}</div></Container></main><SiteFooter /></>; }
