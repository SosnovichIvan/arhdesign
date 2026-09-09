import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { projects } from "@/shared/config";
import { BackButton } from "@/shared/components/backButton";
import { SiteFooter } from "@/shared/components/siteFooter";
import { ProjectCarousel } from "@/shared/components/projectCarousel";
import { Container, Typography } from "@/shared/ui";

export function generateStaticParams() { return projects.map(({ slug }) => ({ slug })); }
export async function generateMetadata({ params }: { params: Promise<{ slug: string }> }): Promise<Metadata> {
  const { slug } = await params;
  const project = projects.find((item) => item.slug === slug);
  if (!project) return {};
  return {
    alternates: { canonical: `/projects/${project.slug}` },
    description: project.description,
    openGraph: { images: [{ alt: project.title, url: project.image }], title: project.title },
    title: project.title,
  };
}
export default async function ProjectPage({ params }: { params: Promise<{ slug: string }> }) { const { slug } = await params; const project = projects.find((item) => item.slug === slug); if (!project) notFound(); return <><main className="py-10 tablet:py-16"><Container><div className="flex items-center justify-between gap-4"><Typography as="h1" variant="title">{project.title}</Typography><BackButton fallbackHref="/projects" label="Назад" /></div><p className="mt-3 text-secondary">{project.location}</p><div className="mt-8"><ProjectCarousel images={project.images} title={project.title}/></div><p className="mt-8 max-w-2xl text-lg leading-8 text-secondary">{project.description}</p></Container></main><SiteFooter /></>; }
