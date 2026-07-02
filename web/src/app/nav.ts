import type { LucideIcon } from "lucide-react";
import {
  Activity,
  BookPlus,
  CalendarDays,
  Clock3,
  FolderInput,
  LayoutDashboard,
  Library,
  ServerCog,
  Settings
} from "lucide-react";

/**
 * Top-level navigation. Paths are stable API for bookmarks, the reverse
 * proxy config, and the deployed E2E flows — change labels, not paths.
 */
export type NavItem = {
  id: string;
  path: string;
  label: string;
  icon: LucideIcon;
  subtitle: string;
};

export const navItems: NavItem[] = [
  {
    id: "dashboard",
    path: "/dashboard",
    label: "Dashboard",
    icon: LayoutDashboard,
    subtitle: "Items needing review, health, and recent runs"
  },
  {
    id: "library",
    path: "/library",
    label: "Library",
    icon: Library,
    subtitle: "Monitored authors and books"
  },
  {
    id: "search",
    path: "/search",
    label: "Add New",
    icon: BookPlus,
    subtitle: "Search metadata providers and add books or authors"
  },
  {
    id: "wanted",
    path: "/wanted",
    label: "Wanted",
    icon: Clock3,
    subtitle: "Missing books, release decisions, and author subscriptions"
  },
  {
    id: "calendar",
    path: "/calendar",
    label: "Calendar",
    icon: CalendarDays,
    subtitle: "Upcoming and recent releases for monitored books"
  },
  {
    id: "downloads",
    path: "/downloads",
    label: "Activity",
    icon: Activity,
    subtitle: "Download queue and history"
  },
  {
    id: "imports",
    path: "/imports",
    label: "Library Import",
    icon: FolderInput,
    subtitle: "Scan roots, import files, and resolve reviews"
  },
  {
    id: "providers",
    path: "/providers",
    label: "System",
    icon: ServerCog,
    subtitle: "Setup checklist, provider health, and Readarr compatibility"
  },
  {
    id: "settings",
    path: "/settings",
    label: "Settings",
    icon: Settings,
    subtitle: "Access, media management, profiles, and connections"
  }
];

export const defaultPath = "/library";
