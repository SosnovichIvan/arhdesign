import Link from "next/link";

export default function NotFound() {
  return (
    <main className="mx-auto grid min-h-[60vh] max-w-2xl content-center gap-5 px-6 py-12">
      <p className="text-sm font-medium text-action">404</p>
      <h1 className="font-display text-4xl tablet:text-5xl">Страница не найдена</h1>
      <p className="text-secondary">Проверьте адрес или вернитесь на главную.</p>
      <Link className="inline-flex min-h-11 w-fit items-center rounded-full bg-primary px-5 text-sm text-inverse-text" href="/">На главную</Link>
    </main>
  );
}
