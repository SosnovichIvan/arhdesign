"use client";

import { useSyncExternalStore } from "react";

const query = "(max-width: 63.999rem)";

function readWidth() {
  return window.matchMedia?.(query).matches ?? false;
}

function subscribe(notify: () => void) {
  const preference = window.matchMedia?.(query);
  preference?.addEventListener?.("change", notify);
  return () => preference?.removeEventListener?.("change", notify);
}

export function useNarrowWorkspace() {
  return useSyncExternalStore(subscribe, readWidth, () => false);
}
