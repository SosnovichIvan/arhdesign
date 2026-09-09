"use client";

import Image from "next/image";
import Link from "next/link";

import type { Project } from "@/shared/config";

export function ProjectCard({ project }: { project: Project }) {
  const href = `/projects/${project.slug}`;

  return <article className="group overflow-hidden border border-border bg-surface"><Image alt={project.title} className="aspect-[4/3] w-full object-cover transition-transform duration-300 group-hover:scale-[1.02]" height={720} src={project.image} width={960} /><div className="space-y-3 p-5"><p className="text-xs uppercase tracking-[0.12em] text-secondary">{project.location}</p><h2 className="font-display text-3xl text-primary">{project.title}</h2><p className="text-sm leading-6 text-secondary">{project.description}</p><Link className="inline-flex min-h-11 items-center text-sm font-semibold uppercase tracking-[0.1em] text-primary underline decoration-action underline-offset-4" href={href} onClick={() => window.sessionStorage.setItem("arhdesign:project-return", JSON.stringify({ from: window.location.href, target: href }))}>Подробнее</Link></div></article>;
}
