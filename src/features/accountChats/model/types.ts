export type GlobalChatKind = "direct" | "group";

export type GlobalChatCandidate = {
  email: string;
  firstName: string;
  lastName: string | null;
  login: string;
  userId: string;
};

export type GlobalChatMember = GlobalChatCandidate & { joinedAt: string };

export type GlobalChatMessage = {
  author: Omit<GlobalChatCandidate, "email">;
  body: string;
  chatId: string;
  createdAt: string;
  deletedAt: string | null;
  editedAt: string | null;
  id: string;
  version: number;
};

export type GlobalChatSummary = {
  displayName: string;
  id: string;
  kind: GlobalChatKind;
  lastActivityAt: string;
  lastMessage: GlobalChatMessage | null;
  members: GlobalChatMember[];
  name: string | null;
  unreadCount: number;
  version: number;
};

export type GlobalChatPage = {
  hasMore: boolean;
  items: GlobalChatSummary[];
  nextCursor: string | null;
};

export type GlobalChatConversation = {
  canSend: boolean;
  chat: GlobalChatSummary;
  currentUserId: string;
  hasMore: boolean;
  messages: GlobalChatMessage[];
  nextCursor: string | null;
};

export type CreateGlobalChatInput = {
  kind: GlobalChatKind;
  memberUserIds: string[];
  name?: string;
};
