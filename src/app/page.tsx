import Link from "next/link";
import { ContactFormProvider, ContactFormTrigger } from "@/features/contactForm";
import { ProjectCard } from "@/shared/components/projectCard";
import { ProjectCarousel } from "@/shared/components/projectCarousel";
import { SiteFooter } from "@/shared/components/siteFooter";
import { projects } from "@/shared/config";
import { Container, Typography } from "@/shared/ui";

const heroImages = projects.flatMap((project) => project.images);

export default function HomePage() {
  return <ContactFormProvider>
    <main>
      <section className="py-16 tablet:py-24 desktop:py-32">
        <Container className="desktop:grid desktop:grid-cols-2 desktop:items-center desktop:gap-16">
          <div>
            <p className="text-xs font-semibold tracking-[0.12em] text-action">АРХИТЕКТУРА И ИНТЕРЬЕРЫ · МОСКВА</p>
            <Typography as="h1" className="mt-6 max-w-4xl" variant="display">Пространства, которые остаются</Typography>
            <p className="mt-6 max-w-xl text-lg leading-8 text-secondary">Создаю архитектуру и интерьеры, в которых точность планировки соединяется с тишиной материалов и естественным светом.</p>
            <div className="mt-8 flex flex-wrap gap-4">
              <Link className="inline-flex min-h-11 items-center rounded-full bg-primary px-5 text-xs font-semibold uppercase tracking-[0.1em] text-inverse-text hover:bg-action" href="/projects">Смотреть проекты</Link>
              <ContactFormTrigger />
            </div>
          </div>
          <ProjectCarousel className="mt-12 desktop:mt-0" imageClassName="aspect-[4/5] tablet:aspect-[16/10] desktop:aspect-[4/5]" images={heroImages} title="Избранные проекты" />
        </Container>
      </section>
      <section className="bg-surface py-16 tablet:py-24" id="projects">
        <Container>
          <div className="flex items-center justify-between gap-4"><Typography as="h2" variant="title">Последние проекты</Typography><Link className="inline-flex min-h-11 items-center text-xs font-semibold tracking-[0.1em] text-primary uppercase hover:text-action" href="/projects">Перейти <span aria-hidden="true" className="ml-2">→</span></Link></div>
          <div className="mt-10 grid gap-6 tablet:grid-cols-2 desktop:grid-cols-3">{projects.map((project) => <ProjectCard key={project.slug} project={project} />)}</div>
        </Container>
      </section>
      <section className="py-16 tablet:py-24" id="services"><Container><div className="flex flex-col justify-between gap-6 tablet:flex-row"><Typography as="h2" className="tablet:text-6xl" variant="title">Услуги</Typography><p className="max-w-sm leading-7 text-secondary">От первого эскиза до пространства,<br />готового к жизни и работе.</p></div><div className="mt-10 border-t border-border">{[["01","Архитектурное проектирование","Концепция, планировочные решения, рабочая документация и авторское сопровождение."],["02","Дизайн интерьера","Цельный интерьер: от сценариев жизни и материалов до мебели и света."],["03","Авторский надзор","Контроль реализации, работа с подрядчиками и сохранение проектного замысла."]].map(([number,title,text]) => <div className="grid gap-4 border-b border-border px-4 py-6 tablet:grid-cols-[48px_360px_1fr_24px] tablet:items-center tablet:px-8" key={number}><span className="text-xs text-secondary">{number}</span><h3 className="text-xl font-medium tablet:text-2xl">{title}</h3><p className="text-sm leading-6 text-secondary">{text}</p><span aria-hidden="true" className="text-xl">↗</span></div>)}</div></Container></section>
      <section className="bg-surface py-16 tablet:py-24" id="process"><Container><div className="flex flex-col justify-between gap-6 tablet:flex-row tablet:items-center"><Typography as="h2" className="tablet:text-6xl" variant="title">Как строится работа</Typography><p className="max-w-sm leading-7 text-secondary">Прозрачный маршрут от первой встречи<br />до завершённого пространства.</p></div><ol className="mt-10 border-t border-border">{[["01","Знакомство и бриф","Фиксируем задачи, контекст, бюджет, сроки и критерии результата."],["02","Концепция и проект","Разрабатываем планировки, образ, материалы и комплект рабочей документации."],["03","Реализация","Сопровождаем стройку, согласуем решения и контролируем соответствие проекту."]].map(([number,title,text]) => <li className="grid gap-3 border-b border-border px-4 py-4 tablet:grid-cols-[48px_1fr_12px] tablet:px-4" key={number}><span className="text-xs text-secondary">{number}</span><div><h3 className="text-lg font-medium">{title}</h3><p className="mt-2 text-sm leading-6 text-secondary">{text}</p></div><span aria-hidden="true" className="size-3 rounded-full bg-action/60" /></li>)}</ol></Container></section>
      <section className="py-16 tablet:py-24" id="author"><Container className="grid items-center gap-12 desktop:grid-cols-2 desktop:gap-30"><div className="mx-auto flex h-[373px] w-full max-w-[560px] items-center justify-center"><div className="flex h-[310px] w-full max-w-[464px] items-center justify-center bg-[#1e211f] px-8 text-center text-[#fbfaf7]"><div><p className="font-display text-[140px] leading-none tracking-[-0.04em] tablet:text-[172px]">ПС</p><p className="mt-4 text-xs tracking-[0.16em] text-[#b7b8b1]">ARCHITECTURE · EST. 2005</p></div></div></div><div><Typography className="text-action" variant="label">Об авторе</Typography><Typography as="h2" className="mt-6 tablet:text-6xl" variant="title">Полисмакова Светлана</Typography><p className="mt-6 max-w-xl text-lg leading-8 text-secondary">Архитектор и дизайнер интерьеров. Создаёт спокойные, функциональные пространства, где планировка, естественный свет и материалы работают как единое целое.</p><dl className="mt-6 grid gap-2 text-sm leading-7 tablet:grid-cols-[100px_1fr]"><dt className="text-secondary">Фокус</dt><dd>Частные дома и жилые интерьеры</dd><dt className="text-secondary">География</dt><dd>Москва и проекты в регионах</dd></dl></div></Container></section>
      <section className="bg-surface py-16 tablet:py-24" id="contact"><Container><div className="bg-[#1e211f] px-6 py-10 text-[#fbfaf7] tablet:px-12 tablet:py-12"><h2 className="font-display text-4xl leading-tight tablet:text-5xl">Обсудим ваш будущий проект</h2><p className="mt-5 max-w-3xl leading-7 text-[#d7d3ca]">Расскажите о пространстве, сроках и задаче — в ответ получите ориентир по формату работы и следующий шаг.</p><div className="mt-6"><ContactFormTrigger triggerClassName="bg-transparent px-0 text-[#fbfaf7] hover:bg-transparent hover:text-action" triggerLabel="Обсудить проект →" /></div></div></Container></section>
    </main>
    <SiteFooter />
  </ContactFormProvider>;
}
