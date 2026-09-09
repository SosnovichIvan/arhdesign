import type { MetadataRoute } from "next";

import { siteUrl } from "@/shared/config";

export default function robots(): MetadataRoute.Robots {
  return {
    host: siteUrl.origin,
    rules: { allow: "/", userAgent: "*" },
    sitemap: new URL("/sitemap.xml", siteUrl).toString(),
  };
}
