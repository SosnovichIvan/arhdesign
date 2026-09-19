import type { Metadata } from "next";
import Link from "next/link";

import { SiteFooter } from "@/shared/components/siteFooter";
import { consent, privacy } from "@/shared/config";
import { Container, Typography } from "@/shared/ui";

export const metadata: Metadata = {
  alternates: { canonical: "/privacy" },
  description: "Политика ИП Полисмаковой Светланы Александровны в отношении обработки персональных данных посетителей сайта designer-svetlana.ru.",
  title: "Политика обработки персональных данных",
};

const linkClassName = "text-primary underline decoration-action underline-offset-4 transition-colors hover:text-action";
const sectionClassName = "border-t border-border pt-8";
const headingClassName = "font-display text-3xl leading-tight tablet:text-4xl";
const paragraphClassName = "mt-4 text-base leading-7 text-secondary";
const listClassName = "mt-4 list-disc space-y-2 pl-6 text-base leading-7 text-secondary";

export default function PrivacyPage() {
  return <>
    <main className="py-12 tablet:py-20">
      <Container>
        <article className="mx-auto max-w-4xl">
          <Link className="inline-flex min-h-11 items-center text-xs font-semibold tracking-[0.1em] uppercase hover:text-action" href="/"><span aria-hidden="true" className="mr-2">←</span>На главную</Link>
          <Typography className="mt-8 text-action" variant="label">Версия {privacy.version}</Typography>
          <Typography as="h1" className="mt-5 max-w-3xl" variant="display">Политика обработки персональных данных</Typography>
          <p className="mt-6 max-w-3xl text-lg leading-8 text-secondary">Документ объясняет, какие данные собирает сайт designer-svetlana.ru, зачем они нужны, где хранятся, кому передаются и как посетитель может реализовать свои права.</p>
          <div className="mt-8 flex flex-wrap gap-4"><a className="inline-flex min-h-11 items-center rounded-full bg-primary px-5 text-xs font-semibold uppercase tracking-[0.1em] text-inverse-text transition-colors hover:bg-action" href={privacy.documentHref} rel="noopener noreferrer" target="_blank">Открыть политику в PDF</a><a className="inline-flex min-h-11 items-center rounded-full border border-border px-5 text-xs font-semibold uppercase tracking-[0.1em] transition-colors hover:border-action hover:text-action" href={consent.documentHref} rel="noopener noreferrer" target="_blank">Открыть согласие в PDF</a></div>

          <div className="mt-14 space-y-12">
            <section className={sectionClassName}>
              <h2 className={headingClassName}>1. Оператор и область действия</h2>
              <p className={paragraphClassName}>Оператор — Индивидуальный предприниматель ПОЛИСМАКОВА СВЕТЛАНА АЛЕКСАНДРОВНА, ИНН 682962659118, ОГРНИП 324508100255269. Почтовый адрес: 142000, Московская область, город Домодедово, Южный внутригородской район, улица Курыжова, 25, 157. Электронная почта: <a className={linkClassName} href="mailto:svetlana@polismakova.ru">svetlana@polismakova.ru</a>, телефон: <a className={linkClassName} href="tel:+79933353775">+7 993 335-37-75</a>.</p>
              <p className={paragraphClassName}>Политика применяется к данным посетителей, которые отправляют форму обратной связи, а также к техническим данным, обрабатываемым при использовании сайта.</p>
            </section>

            <section className={sectionClassName}>
              <h2 className={headingClassName}>2. Цели, данные и правовое основание</h2>
              <p className={paragraphClassName}>Правовым основанием является согласие посетителя на обработку персональных данных. Данные используются для рассмотрения обращения, связи с заявителем, уточнения параметров проекта, подготовки предложения, доставки служебного уведомления оператору, предотвращения спама и обеспечения безопасности сайта.</p>
              <ul className={listClassName}><li>имя;</li><li>номер телефона или адрес электронной почты;</li><li>тип проекта;</li><li>описание проекта, если посетитель заполнил это поле;</li><li>дата и время отправки формы, версия и контрольная сумма согласия;</li><li>адрес страницы, с которой отправлена форма;</li><li>IP-адрес, временно используемый в памяти сервера для ограничения частоты запросов;</li><li>технический cookie <code>contact_cooldown</code> и его HMAC-хеш для защиты от повторной отправки.</li></ul>
              <p className={paragraphClassName}>Сайт не собирает специальные категории, биометрические, паспортные или платежные данные и не принимает решений, порождающих юридические последствия, исключительно на основании автоматизированной обработки.</p>
            </section>

            <section className={sectionClassName}>
              <h2 className={headingClassName}>3. Способы и действия обработки</h2>
              <p className={paragraphClassName}>Обработка является автоматизированной и выполняется с передачей по сети Интернет. Оператор осуществляет сбор, запись, систематизацию, накопление, хранение, уточнение, извлечение, использование, предоставление доступа, передачу по поручению оператора, блокирование, удаление и уничтожение данных.</p>
            </section>

            <section className={sectionClassName}>
              <h2 className={headingClassName}>4. Хранение, инфраструктура и получатели</h2>
              <ul className={listClassName}><li>Основная база PostgreSQL размещена на VPS REG.RU на территории Российской Федерации. Заявки хранятся до достижения цели, отзыва согласия или не более 365 дней.</li><li>Cloudflare Workers используется как технический канал доставки уведомления и не предназначен для самостоятельного хранения текста заявки.</li><li>Telegram Bot API доставляет уведомление в авторизованный чат оператора. Сообщение с данными ставится на автоматическое удаление не позднее чем через 24 часа.</li><li>Резервные копии PostgreSQL создаются на сервере ежедневно и автоматически удаляются через 30 дней.</li><li>Access-логи Caddy хранятся с ротацией не более 30 дней. Приложение не записывает содержимое формы в журналы.</li></ul>
              <p className={paragraphClassName}>Сначала данные записываются в базу на территории Российской Федерации. Затем для доставки уведомления они передаются через Cloudflare Workers в Telegram Bot API. Такая передача может осуществляться с использованием инфраструктуры за пределами Российской Федерации и рассматривается оператором как трансграничная.</p>
              <p className={paragraphClassName}>При включении дополнительного почтового канала оператор обязан до его запуска обновить настоящую Политику и указать соответствующего получателя или поставщика.</p>
            </section>

            <section className={sectionClassName}>
              <h2 className={headingClassName}>5. Cookie и защита от злоупотреблений</h2>
              <p className={paragraphClassName}>После принятия заявки сайт устанавливает обязательный cookie <code>contact_cooldown</code> сроком на один час. Он содержит случайный технический идентификатор, недоступен JavaScript и не используется для рекламы или аналитики. В PostgreSQL хранится только HMAC-хеш идентификатора; просроченные записи удаляются автоматически.</p>
            </section>

            <section className={sectionClassName}>
              <h2 className={headingClassName}>6. Права посетителя и отзыв согласия</h2>
              <p className={paragraphClassName}>Посетитель вправе запросить сведения об обработке, уточнение, блокирование или удаление данных, отозвать согласие, а также обжаловать действия оператора в Роскомнадзор или в суд. Запрос можно направить на <a className={linkClassName} href="mailto:svetlana@polismakova.ru">svetlana@polismakova.ru</a> или по почтовому адресу оператора. В запросе следует указать имя, использованный контакт и суть требования, чтобы оператор мог найти соответствующую заявку.</p>
            </section>

            <section className={sectionClassName}>
              <h2 className={headingClassName}>7. Безопасность и уничтожение</h2>
              <p className={paragraphClassName}>Оператор применяет разграничение доступа, TLS, изоляцию базы данных во внутренней сети Docker, секреты окружения, HMAC-хеширование технического идентификатора, ограничение частоты запросов, журналирование без содержимого формы и автоматическую очистку. После наступления срока или законного требования данные удаляются из основной базы и связанных записей; остаточные копии прекращают существование по циклу ротации резервных копий и журналов.</p>
            </section>

            <section className={sectionClassName}>
              <h2 className={headingClassName}>8. Изменение Политики</h2>
              <p className={paragraphClassName}>При изменении целей, состава данных, получателей или сроков оператор публикует новую версию. К заявке привязывается редакция согласия, действовавшая в момент отправки формы. Действующая версия Политики опубликована на этой странице.</p>
            </section>
          </div>
        </article>
      </Container>
    </main>
    <SiteFooter />
  </>;
}
