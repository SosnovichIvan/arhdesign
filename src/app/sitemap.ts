import type { MetadataRoute } from "next";

import { projects, siteUrl } from "@/shared/config";

export default function sitemap(): MetadataRoute.Sitemap {
  return ["/", "/projects", ...projects.map((project) => `/projects/${project.slug}`)].map((path) => ({
    changeFrequency: "monthly",
    priority: path === "/" ? 1 : 0.8,
    url: new URL(path, siteUrl).toString(),
  }));
}
