import { useState } from "react";
import type { ReactNode } from "react";
import {
  Bell,
  Blocks,
  ChartColumn,
  ChevronDown,
  CircleQuestionMark,
  FolderKanban,
  LayoutDashboard,
  Network,
  Search,
  Settings as SettingsIcon,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

export type NavId =
  | "dashboard"
  | "projects"
  | "tunnels"
  | "analytics"
  | "settings"
  | "extensions"
  | "help";

const NAV: ReadonlyArray<{
  id: NavId;
  label: string;
  icon: typeof LayoutDashboard;
}> = [
  { id: "dashboard", label: "Dashboard", icon: LayoutDashboard },
  { id: "projects", label: "Projects", icon: FolderKanban },
  { id: "tunnels", label: "Tunnels", icon: Network },
  { id: "analytics", label: "Analytics", icon: ChartColumn },
  { id: "settings", label: "Settings", icon: SettingsIcon },
  { id: "extensions", label: "Extensions", icon: Blocks },
  { id: "help", label: "Help & Support", icon: CircleQuestionMark },
];

export interface AppShellProps {
  children: ReactNode;
  /** Highlights a navigation entry; omit to let the shell track its own selection. */
  active?: NavId;
  onNavigate?: (id: NavId) => void;
  /** Replaces the left side of the top bar, for a project chip or a back control. */
  context?: ReactNode;
  /** Controls shown at the right of the top bar. */
  topbarActions?: ReactNode;
  searchPlaceholder?: string;
  /** Optional content pinned below the navigation, such as a profile chip. */
  sidebarFooter?: ReactNode;
}

/**
 * Application frame shared by every screen: brand and primary navigation on
 * the left, search and context in the top bar, scrolling content on the right.
 */
export function AppShell({
  children,
  active,
  onNavigate,
  context,
  topbarActions,
  searchPlaceholder = "Search…",
  sidebarFooter,
}: AppShellProps) {
  const [selected, setSelected] = useState<NavId>("dashboard");
  const current = active ?? selected;

  const select = (id: NavId) => {
    setSelected(id);
    onNavigate?.(id);
  };

  return (
    <div className="flex h-screen w-full overflow-hidden bg-background text-foreground">
      <aside className="flex w-60 shrink-0 flex-col border-r border-sidebar-border bg-sidebar">
        <div className="flex h-16 items-center gap-2.5 px-4">
          <svg viewBox="0 0 32 32" aria-hidden className="size-8 shrink-0">
            <defs>
              <linearGradient id="locafy-mark" x1="2" y1="2" x2="30" y2="30" gradientUnits="userSpaceOnUse">
                <stop stopColor="#FF873D" />
                <stop offset="1" stopColor="#F05252" />
              </linearGradient>
            </defs>
            <path d="M16 1.5 29 9v14L16 30.5 3 23V9Z" fill="url(#locafy-mark)" />
            <path d="M16 1.5 29 9 16 16.5 3 9Z" fill="#F5F3F8" fillOpacity="0.22" />
            <path d="M16 16.5 29 9v14L16 30.5Z" fill="#111116" fillOpacity="0.18" />
          </svg>
          <span className="text-lg font-semibold tracking-tight">Locafy</span>
        </div>

        <nav aria-label="Main" className="flex flex-1 flex-col gap-0.5 overflow-y-auto px-2 py-2">
          {NAV.map((item) => {
            const Icon = item.icon;
            const isCurrent = item.id === current;
            return (
              <Button
                key={item.id}
                variant="ghost"
                aria-current={isCurrent ? "page" : undefined}
                onClick={() => select(item.id)}
                className={cn(
                  "relative h-10 w-full justify-start gap-2.5 px-3 font-normal text-muted-foreground",
                  isCurrent && "bg-sidebar-accent text-sidebar-accent-foreground",
                )}
              >
                {isCurrent && (
                  <span className="absolute left-0 top-1/2 h-4 w-0.5 -translate-y-1/2 rounded-full bg-primary" />
                )}
                <Icon className="size-4" />
                {item.label}
              </Button>
            );
          })}
        </nav>

        {sidebarFooter && (
          <div className="border-t border-sidebar-border p-2">{sidebarFooter}</div>
        )}
      </aside>

      <div className="flex min-w-0 flex-1 flex-col">
        <header className="flex h-16 shrink-0 items-center gap-3 border-b border-border px-6">
          {context}
          <div className="relative w-full max-w-md">
            <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              type="search"
              aria-label="Search"
              placeholder={searchPlaceholder}
              className="h-9 pl-9 pr-14"
            />
            <kbd className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 rounded border border-border px-1.5 py-0.5 font-mono text-[0.65rem] text-muted-foreground">
              Ctrl K
            </kbd>
          </div>
          <div className="ml-auto flex shrink-0 items-center gap-2">
            <span className="flex h-8 items-center gap-2 rounded-full border border-border bg-card px-3 text-xs font-medium">
              <span className="size-2 shrink-0 rounded-full bg-success" />
              Locafy Running
              <ChevronDown className="size-3.5 text-muted-foreground" />
            </span>
            <Button variant="ghost" size="icon" aria-label="Notifications" className="relative">
              <Bell className="size-4.5" />
              <span className="absolute right-1.5 top-1.5 size-1.5 rounded-full bg-accent ring-2 ring-background" />
            </Button>
            {topbarActions}
          </div>
        </header>

        <main className="flex-1 overflow-y-auto">{children}</main>
      </div>
    </div>
  );
}
