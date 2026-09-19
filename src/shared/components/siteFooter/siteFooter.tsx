import Link from "next/link";

import { Container } from "@/shared/ui";
import { contact, privacy } from "@/shared/config";
import { SocialLinks } from "@/shared/components/socialLinks";

export function SiteFooter() { return <footer className="bg-footer py-10 text-footer-text"><Container className="grid gap-8 tablet:grid-cols-2"><div><p className="max-w-sm text-lg leading-7">Архитектура и интерьеры, в которых удобно жить.</p><Link className="mt-5 inline-flex min-h-11 items-center text-sm underline decoration-action underline-offset-4 transition-colors hover:text-action" href={privacy.pageHref}>Политика обработки персональных данных</Link></div><div className="space-y-3 text-sm tablet:justify-self-end"><a className="block hover:text-action" href="tel:+79933353775">{contact.phone}</a><a className="block hover:text-action" href={`mailto:${contact.email}`}>{contact.email}</a><SocialLinks /></div><p className="text-xs opacity-70 tablet:col-span-2">© 2026 {contact.name}</p></Container></footer>; }
