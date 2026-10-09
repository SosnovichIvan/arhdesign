export type AccountStatus = "pending_verification" | "active" | "disabled";

export type AdminUser = {
  id: string;
  login: string;
  email: string;
  firstName: string;
  lastName: string | null;
  middleName: string | null;
  globalRole: "super_admin" | "technical_admin" | null;
  professionalRole: { code: string; name: string };
  status: AccountStatus;
  registeredAt: string;
  lastInteractiveLoginAt: string | null;
  version: number;
};

export type AdminUserPage = { hasMore: boolean; items: AdminUser[]; nextCursor: string | null };
