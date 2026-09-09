"use client";

import Image from "next/image";
import Link from "next/link";
import { useEffect, useState } from "react";

import { ThemeToggle } from "@/features/themeToggle";
import { SocialLinks } from "@/shared/components/socialLinks";
import { Container } from "@/shared/ui";

const links = [
  { href: "/#projects", label: "Проекты" },
  { href: "/#services", label: "Услуги" },
  { href: "/#author", label: "Об авторе" },
  { href: "/#contact", label: "Контакт" },
];

function getActiveHash() {
  return window.location.hash || "#projects";
}

export function SiteHeader() {
  const [isMenuOpen, setIsMenuOpen] = useState(false);
  const [activeHash, setActiveHash] = useState("#projects");

  useEffect(() => {
    const syncActiveHash = () => setActiveHash(getActiveHash());
    syncActiveHash();
    window.addEventListener("hashchange", syncActiveHash);
    return () => window.removeEventListener("hashchange", syncActiveHash);
  }, []);

  return <header className="fixed inset-x-0 top-0 z-40 border-b border-border bg-page/95 backdrop-blur"><Container className="flex min-h-[72px] items-center gap-4"><Link aria-label="Полисмакова Светлана — главная" className="mr-auto inline-flex items-center gap-3 text-sm font-semibold tracking-[0.08em]" href="/"><Image alt="" aria-hidden="true" className="size-8 rounded-md" height={32} src="/icon.svg" width={32} /><span>ПОЛИСМАКОВА СВЕТЛАНА</span></Link><nav aria-label="Основная навигация" className="hidden gap-4 tablet:flex">{links.map((link) => { const isActive = activeHash === link.href.slice(1); return <Link aria-current={isActive ? "page" : undefined} className={`border-b-2 pb-1 transition-colors ${isActive ? "border-action text-primary" : "border-transparent text-secondary hover:border-action/50 hover:text-primary"}`} href={link.href} key={link.href} onClick={() => setActiveHash(link.href.slice(1))}>{link.label}</Link>; })}</nav><SocialLinks className="hidden desktop:flex" /><button aria-controls="mobile-navigation" aria-expanded={isMenuOpen} aria-label={isMenuOpen ? "Закрыть меню" : "Открыть меню"} className="inline-flex size-11 items-center justify-center text-xl tablet:hidden" onClick={() => setIsMenuOpen((open) => !open)} type="button"><span aria-hidden="true">{isMenuOpen ? "×" : "☰"}</span></button><ThemeToggle /></Container>{isMenuOpen ? <nav aria-label="Мобильная навигация" className="border-t border-border bg-page px-6 py-6 tablet:hidden" id="mobile-navigation"><Container className="flex flex-col items-start gap-5">{links.map((link) => <Link aria-current={activeHash === link.href.slice(1) ? "page" : undefined} className="text-lg font-semibold" href={link.href} key={link.href} onClick={() => { setActiveHash(link.href.slice(1)); setIsMenuOpen(false); }}>{link.label}</Link>)}<SocialLinks /></Container></nav> : null}</header>;
}
