import { contact } from "@/shared/config";
import { cn } from "@/shared/lib";

type SocialLinksProps = {
  className?: string;
};

export function SocialLinks({ className }: SocialLinksProps) {
  return (
    <div className={cn("flex items-center gap-3", className)}>
      <a aria-label="Открыть VK Полисмаковой Светланы" className="inline-flex size-11 items-center justify-center rounded-full border border-current transition-colors hover:bg-action hover:text-inverse-text" href={contact.vkUrl} rel="noopener noreferrer" target="_blank">
        <svg aria-hidden="true" fill="none" height="18" viewBox="0 0 24 24" width="18"><path d="M4 6.8h3.3c.15 5.5 2.55 7.83 4.45 8.32V6.8h3.1v4.75c1.87-.2 3.83-2.37 4.5-4.75h3.1a8.99 8.99 0 0 1-4.15 5.88 9.3 9.3 0 0 1 4.85 6.52h-3.42c-.73-2.28-2.52-4.05-4.88-4.3v4.3h-.37C8.98 19.2 5.25 15.4 4 6.8Z" fill="currentColor" /></svg>
      </a>
      <a aria-label="Открыть Telegram Полисмаковой Светланы" className="inline-flex size-11 items-center justify-center rounded-full border border-current transition-colors hover:bg-action hover:text-inverse-text" href={contact.telegramUrl} rel="noopener noreferrer" target="_blank">
        <svg aria-hidden="true" fill="none" height="18" viewBox="0 0 24 24" width="18"><path d="m20.7 4.4-3.02 14.24c-.23 1.01-.82 1.25-1.66.78l-4.59-3.38-2.21 2.13c-.24.24-.45.45-.92.45l.33-4.67 8.5-7.68c.37-.33-.08-.52-.58-.19L6.03 12.7l-4.53-1.42c-.98-.3-1- .98.2-1.45L19.42 3c.82-.3 1.54.2 1.27 1.4Z" fill="currentColor" /></svg>
      </a>
    </div>
  );
}
