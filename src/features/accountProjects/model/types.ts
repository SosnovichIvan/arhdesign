export type ProjectStatus = "draft" | "active" | "paused" | "completed" | "pending_deletion" | "archived";

export type ProjectView = {
  adminUserIds?: string[];
  address: string | null;
  autoApproveExpenses: boolean;
  createdAt: string;
  createdByUserId: string;
  currencyCode: string;
  customerUserId: string | null;
  description: string | null;
  id: string;
  name: string;
  plannedFinishOn: string | null;
  plannedStartOn: string | null;
  status: ProjectStatus;
  type: string;
  version: number;
};

export type ProjectPage = {
  hasMore: boolean;
  items: ProjectView[];
  nextCursor: string | null;
};

export type CreateProjectPayload = {
  address?: string;
  autoApproveExpenses: boolean;
  currencyCode: string;
  customerUserId: null;
  description?: string;
  name: string;
  plannedFinishOn?: string;
  plannedStartOn?: string;
  type: string;
};

export type UpdateProjectPayload = {
  address?: string | null;
  autoApproveExpenses?: boolean;
  description?: string | null;
  name?: string;
  plannedFinishOn?: string | null;
  plannedStartOn?: string | null;
  status?: Exclude<ProjectStatus, "pending_deletion" | "archived">;
  type?: string;
};

export type CurrentAccount = {
  globalRole: "super_admin" | "technical_admin" | null;
  id: string;
  login: string;
};

export type ProfessionalRole = { code: string; name: string };
export type ProjectRole = "customer" | "project_admin" | "executor";
export type ProjectMember = {
  email: string;
  firstName: string;
  joinedAt: string;
  lastName: string | null;
  login: string;
  middleName: string | null;
  professionalRole: ProfessionalRole;
  projectRoles: ProjectRole[];
  removable: boolean;
  userId: string;
};
export type ProjectMemberList = { canManage: boolean; items: ProjectMember[] };
export type ProjectMemberCandidate = Omit<ProjectMember, "joinedAt" | "projectRoles" | "removable">;
export type ProjectMemberCandidateList = { items: ProjectMemberCandidate[] };

export type ProjectChatAuthor = {
  firstName: string;
  lastName: string | null;
  login: string;
  userId: string;
};
export type ProjectChatMessage = {
  author: ProjectChatAuthor;
  body: string;
  chatId: string;
  createdAt: string;
  deletedAt: string | null;
  editedAt: string | null;
  id: string;
  projectId: string;
  version: number;
};
export type ProjectChatContextType = "task" | "material" | "expense";
export type ProjectChatContext = {
  chatId: string;
  contextId: string;
  contextTitle: string;
  contextType: ProjectChatContextType;
  name: string;
  projectId: string;
};
export type ProjectChatPage = {
  canSend: boolean;
  chatId: string;
  currentUserId: string;
  hasMore: boolean;
  messages: ProjectChatMessage[];
  nextCursor: string | null;
  projectId: string;
  context?: ProjectChatContext | null;
};
export type ProjectChatSummary = {
  canManage: boolean;
  contextId: string | null;
  contextTitle: string | null;
  contextType?: ProjectChatContextType;
  createdAt: string;
  id: string;
  kind: "project" | "context";
  memberCount: number;
  unreadCount: number;
  name: string;
  projectId: string;
  version: number;
};
export type ProjectChatList = { canCreate: boolean; items: ProjectChatSummary[] };
export type ProjectChatMember = { firstName: string; joinedAt: string; lastName: string | null; login: string; removable: boolean; userId: string };
export type ProjectChatMemberList = { canManage: boolean; items: ProjectChatMember[] };
export type ProjectDocument = { canDelete: boolean; createdAt: string; id: string; mediaType: string; name: string; projectId: string; sizeBytes: number; uploadedByUserId: string; version: number };
export type ProjectDocumentList = { canUpload: boolean; items: ProjectDocument[] };

export type ProjectUpcomingItem = {
  createdAt: string;
  description: string | null;
  effectiveAt: string;
  endsAt: string | null;
  id: string;
  kind: "task" | "meeting";
  location: string | null;
  projectId: string;
  status: string | null;
  title: string;
  version: number;
};

export type ProjectUpcomingFeed = {
  days: number;
  items: ProjectUpcomingItem[];
  rangeEnd: string;
  rangeStart: string;
};

export type ProjectCalendarFeed = Omit<ProjectUpcomingFeed, "days">;

export type ProjectUserSettings = {
  upcomingDays: number;
};

export type ProjectTaskStatus = "new" | "in_progress" | "review" | "changes_requested" | "accepted";
export type ProjectTaskAssignee = { firstName: string; lastName: string | null; login: string; userId: string };
export type ProjectTask = {
  assignees: ProjectTaskAssignee[];
  canChangeStatus: boolean;
  canEdit: boolean;
  completedAt: string | null;
  createdAt: string;
  description: string | null;
  dueAt: string;
  id: string;
  projectId: string;
  startedAt: string | null;
  status: ProjectTaskStatus;
  title: string;
  version: number;
  contextChatId: string | null;
};
export type ProjectTaskList = { canCreate: boolean; items: ProjectTask[] };
export type CreateProjectTaskPayload = { assigneeUserIds?: string[]; description?: string; dueAt: string; title: string };
export type CreateProjectMeetingPayload = { description?: string; endsAt: string; location?: string; startsAt: string; title: string };

export type ExpenseCategory = "materials" | "furniture" | "contractor" | "delivery" | "installation" | "design" | "other";
export type ExpenseStatus = "draft" | "pending_approval" | "auto_approved" | "approved" | "rejected" | "awaiting_payment" | "paid" | "cancelled";
export type ProjectFinanceSummary = {
  availableBalanceMinor: number;
  canCreateExpense: boolean;
  confirmedExpenseMinor: number;
  confirmedIncomeMinor: number;
  currencyCode: string;
  pendingExpenseMinor: number;
};
export type ProjectExpense = {
  amountMinor: number;
  category: ExpenseCategory;
  createdAt: string;
  createdByUserId: string;
  currencyCode: string;
  description: string;
  id: string;
  plannedPaymentOn: string | null;
  projectId: string;
  status: ExpenseStatus;
  vendorName: string | null;
  version: number;
  contextChatId: string | null;
};
export type ProjectExpenseList = { canCreateExpense: boolean; items: ProjectExpense[] };
export type CreateProjectExpensePayload = {
  amountMinor: number;
  category: ExpenseCategory;
  description: string;
  plannedPaymentOn?: string | null;
  vendorName?: string | null;
};

export type MaterialCategory = "materials" | "furniture" | "delivery" | "installation" | "other";
export type MaterialPaymentStatus = "unpaid" | "partial" | "paid";
export type MaterialDeliveryStatus = "expected" | "partially_delivered" | "delivered" | "installed" | "cancelled";
export type ProjectMaterial = {
  actualDeliveryOn: string | null;
  canDelete: boolean;
  canEdit: boolean;
  category: MaterialCategory;
  contactInfo: string | null;
  contractAmountMinor: number;
  contractReference: string | null;
  createdAt: string;
  createdByUserId: string;
  currencyCode: string;
  deliveryStatus: MaterialDeliveryStatus;
  id: string;
  installationOn: string | null;
  linkedExpenseDescription: string | null;
  linkedExpenseId: string | null;
  name: string;
  notes: string | null;
  paidAmountMinor: number;
  paymentStatus: MaterialPaymentStatus;
  plannedDeliveryOn: string | null;
  projectId: string;
  remainingAmountMinor: number;
  supplierName: string;
  updatedAt: string;
  version: number;
  contextChatId: string | null;
};
export type ProjectMaterialList = { canCreate: boolean; canViewFinancialInformation: boolean; items: ProjectMaterial[] };
export type ProjectMaterialInput = {
  actualDeliveryOn?: string | null;
  category: MaterialCategory;
  contactInfo?: string | null;
  contractAmountMinor: number;
  contractReference?: string | null;
  deliveryStatus: MaterialDeliveryStatus;
  installationOn?: string | null;
  linkedExpenseId?: string | null;
  name: string;
  notes?: string | null;
  paymentStatus: MaterialPaymentStatus;
  plannedDeliveryOn?: string | null;
  supplierName: string;
};
