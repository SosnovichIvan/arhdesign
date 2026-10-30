export type AccountNotification = {
  body: string;
  createdAt: string;
  eventType: string;
  href: string;
  id: string;
  projectId: string;
  readAt: string | null;
  title: string;
};

export type AccountNotificationList = { items: AccountNotification[]; unreadCount: number };
