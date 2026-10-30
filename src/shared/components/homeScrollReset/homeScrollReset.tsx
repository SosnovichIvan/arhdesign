"use client";

import { useEffect } from "react";

export function HomeScrollReset() {
  useEffect(() => {
    if (sessionStorage.getItem("arhdesign:scroll-home-top") !== "1") return;
    sessionStorage.removeItem("arhdesign:scroll-home-top");
    const previous = history.scrollRestoration;
    history.scrollRestoration = "manual";
    window.scrollTo({ behavior: "auto", left: 0, top: 0 });
    requestAnimationFrame(() => requestAnimationFrame(() => {
      document.documentElement.scrollTop = 0;
      document.body.scrollTop = 0;
      window.scrollTo({ behavior: "auto", left: 0, top: 0 });
      history.scrollRestoration = previous;
    }));
  }, []);
  return null;
}
