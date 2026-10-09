import type { ReactNode } from "react";

import { AccountShell } from "@/widgets/accountShell";

export default function AccountLayout({ children }: Readonly<{ children: ReactNode }>) {
  return <div className="-mt-[73px]"><AccountShell>{children}</AccountShell></div>;
}
