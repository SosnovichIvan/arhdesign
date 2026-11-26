export function focusField(form: HTMLFormElement, selector: string) {
  requestAnimationFrame(() => {
    const field = form.querySelector<HTMLElement>(selector);
    field?.focus({ preventScroll: true });
    field?.scrollIntoView?.({ block: "nearest", behavior: "instant" });
  });
}
