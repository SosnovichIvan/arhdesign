import type { ProjectStatus, ProjectUpcomingItem } from "@/features/accountProjects";

export type GlobalCalendarProject = {
  id: string;
  items: ProjectUpcomingItem[];
  name: string;
  plannedFinishOn: string | null;
  plannedStartOn: string | null;
  status: ProjectStatus;
};

export type GlobalCalendarFeed = {
  hasMoreProjects: boolean;
  projects: GlobalCalendarProject[];
  rangeEnd: string;
  rangeStart: string;
  truncatedEvents: boolean;
};
