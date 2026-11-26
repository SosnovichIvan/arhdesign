import type { Metadata } from "next";
import Script from "next/script";
import type { ReactNode } from "react";

import "./globals.css";
import { AccountAuthProvider } from "@/features/accountAuth";
import { ThemePreferenceProvider } from "@/features/themeToggle";
import { SiteHeader } from "@/shared/components/siteHeader";
import { organizationJsonLd, siteDescription, siteName, siteUrl } from "@/shared/config";

export const metadata: Metadata = {
  metadataBase: siteUrl,
  title: {
    default: siteName,
    template: `%s | ${siteName}`,
  },
  description: siteDescription,
  alternates: { canonical: "/" },
  openGraph: {
    type: "website",
    locale: "ru_RU",
    siteName,
    title: siteName,
    description: siteDescription,
    url: "/",
    images: [{ alt: "Интерьер из портфолио Светланы Полисмаковой", height: 630, url: "/images/projects/contemporary-harmony.webp", width: 1200 }],
  },
  robots: { follow: true, index: true },
};

type RootLayoutProps = Readonly<{ children: ReactNode }>;

export default function RootLayout({ children }: RootLayoutProps) {
  return (
    <html lang="ru" data-scroll-behavior="smooth" data-theme="light" suppressHydrationWarning>
      <body>
        <Script id="arhdesign-guest-theme" strategy="beforeInteractive">
          {'try { if (localStorage.getItem("arhdesign:theme") === "dark") document.documentElement.dataset.theme = "dark" } catch {}'}
        </Script>
        <ThemePreferenceProvider>
          <AccountAuthProvider>
            <SiteHeader />
            <div className="pt-[73px]">{children}</div>
          </AccountAuthProvider>
        </ThemePreferenceProvider>
        <script dangerouslySetInnerHTML={{ __html: JSON.stringify(organizationJsonLd) }} type="application/ld+json" />
      </body>
    </html>
  );
}
