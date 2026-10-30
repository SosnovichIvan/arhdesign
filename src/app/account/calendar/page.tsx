import type { Metadata } from "next";

import { GlobalProjectsCalendarPage } from "@/features/accountCalendar";

export const metadata: Metadata = { title: "Календарь проектов — личный кабинет" };

export default function AccountCalendarPage() {
  return <GlobalProjectsCalendarPage />;
}
