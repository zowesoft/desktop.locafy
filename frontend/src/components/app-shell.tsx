import { useState } from "react";
import type { ReactNode } from "react";
import {
  Blocks,
  Box,
  Boxes,
  ChartColumn,
  CircleQuestionMark,
  Database,
  Download,
  FileText,
  FolderKanban,
  LayoutDashboard,
  Moon,
  Network,
  Search,
  Settings as SettingsIcon,
  Sun,
  Terminal,
  Zap,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Kbd } from "@/components/ui/kbd";
import { cn } from "@/lib/utils";

export type NavId =
  | "overview"
  | "projects"
  | "applications"
  | "services"
  | "runtimes"
  | "downloads"
  | "domains"
  | "analytics"
  | "tunnels"
  | "logs"
  | "terminal"
  | "extensions"
  | "help"
  | "settings";

type NavItem = {
  id: NavId;
  label: string;
  icon: typeof LayoutDashboard;
  /** Available in a later phase; shown but not selectable. */
  planned?: boolean;
};

const NAV: ReadonlyArray<NavItem> = [
  { id: "overview", label: "Overview", icon: LayoutDashboard },
  { id: "projects", label: "Projects", icon: FolderKanban },
  { id: "applications", label: "Applications", icon: Boxes },
  { id: "services", label: "Services", icon: Database },
  { id: "runtimes", label: "Runtimes", icon: Box },
  { id: "downloads", label: "Downloads", icon: Download },
  { id: "domains", label: "Domains", icon: Network },
  { id: "analytics", label: "Analytics", icon: ChartColumn },
  { id: "tunnels", label: "Tunnels", icon: Zap, planned: true },
  { id: "logs", label: "Logs", icon: FileText },
  { id: "terminal", label: "Terminal", icon: Terminal },
  { id: "extensions", label: "Extensions", icon: Blocks, planned: true },
  { id: "help", label: "Help & Support", icon: CircleQuestionMark },
  { id: "settings", label: "Settings", icon: SettingsIcon },
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
  const [selected, setSelected] = useState<NavId>("overview");
  const current = active ?? selected;
  const [isDark, setIsDark] = useState(() =>
    document.documentElement.classList.contains("dark"),
  );

  const select = (id: NavId) => {
    setSelected(id);
    onNavigate?.(id);
  };

  const toggleTheme = () => {
    const next = !isDark;
    document.documentElement.classList.toggle("dark", next);
    try {
      localStorage.setItem("locafy-theme", next ? "dark" : "light");
    } catch {
      // Storage can be unavailable in a packaged webview; the choice still holds for the session.
    }
    setIsDark(next);
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

            if (item.planned) {
              return (
                <div
                  key={item.id}
                  aria-disabled="true"
                  title={`${item.label} is planned for a later phase`}
                  className="flex h-10 w-full cursor-default items-center gap-2.5 rounded-lg px-3 text-sm text-muted-foreground/60"
                >
                  <Icon className="size-4" />
                  <span className="truncate">{item.label}</span>
                  <PlannedTag />
                </div>
              );
            }

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
                <span className="truncate">{item.label}</span>
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
            <Kbd className="absolute right-3 top-1/2 -translate-y-1/2">
              Ctrl K
            </Kbd>
          </div>
          <div className="ml-auto flex shrink-0 items-center gap-2">
            <Button
              variant="ghost"
              size="icon"
              onClick={toggleTheme}
              aria-label={isDark ? "Switch to light theme" : "Switch to dark theme"}
            >
              {isDark ? <Sun className="size-4.5" /> : <Moon className="size-4.5" />}
            </Button>
            {topbarActions}
          </div>
        </header>

        <main className="flex-1 overflow-y-auto">{children}</main>
      </div>
    </div>
  );
}

/**
 * Marks a capability that is visible but not yet available. The label carries
 * the meaning so the state is never conveyed by colour alone.
 */
export function PlannedTag({ className }: { className?: string }) {
  return (
    <Badge
      variant="outline"
      className={cn("ml-auto uppercase tracking-wide text-muted-foreground", className)}
    >
      Planned
    </Badge>
  );
}
