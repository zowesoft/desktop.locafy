import { useState } from "react";
import { ArrowLeft, Check, ChevronsUpDown, FolderKanban } from "lucide-react";
import { Link, useLocation } from "react-router-dom";

import { LifecycleBadge } from "@/components/lifecycle-badge";
import { buttonVariants } from "@/components/ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { useProjects } from "@/lib/projects-query";
import { cn } from "@/lib/utils";

/**
 * Top-bar control for a project workspace: a way back to the dashboard and a
 * switcher that moves between projects without losing the section in view.
 */
export function ProjectSwitcher({ projectId }: { projectId: string }) {
  const [open, setOpen] = useState(false);
  const location = useLocation();
  const projects = useProjects();

  const rows = projects.data ?? [];
  const current = rows.find((project) => project.ID === projectId);

  // `/projects/<id>/services` keeps `services` when another project is chosen.
  const section = location.pathname.split("/").slice(3).join("/");
  const hrefFor = (id: string) => (section ? `/projects/${id}/${section}` : `/projects/${id}`);

  return (
    <div className="flex shrink-0 items-center gap-1.5">
      <Link
        to="/projects"
        aria-label="All projects"
        className={cn(buttonVariants({ variant: "ghost", size: "icon" }))}
      >
        <ArrowLeft />
      </Link>

      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger
          aria-label={`Project ${current?.Name ?? "unknown"}, switch project`}
          className={cn(buttonVariants({ variant: "secondary" }), "max-w-72 gap-2 font-normal")}
        >
          <FolderKanban className="text-primary" />
          <span className="truncate font-medium">{current?.Name ?? "Unknown project"}</span>
          {current && <LifecycleBadge state={current.LifecycleState} />}
          <ChevronsUpDown className="text-muted-foreground" />
        </PopoverTrigger>

        <PopoverContent align="start" className="w-72 gap-1">
          <p className="px-2 pt-1 text-xs text-muted-foreground">Switch project</p>
          <ul className="max-h-64 overflow-y-auto">
            {rows.map((project) => {
              const isCurrent = project.ID === projectId;

              return (
                <li key={project.ID}>
                  <Link
                    to={hrefFor(project.ID)}
                    aria-current={isCurrent ? "true" : undefined}
                    onClick={() => setOpen(false)}
                    className="flex items-center gap-2 rounded-md px-2 py-1.5 hover:bg-muted"
                  >
                    <span className="min-w-0 flex-1 truncate">{project.Name}</span>
                    <LifecycleBadge state={project.LifecycleState} />
                    <Check className={cn("size-4 shrink-0", isCurrent ? "opacity-100" : "opacity-0")} />
                  </Link>
                </li>
              );
            })}
          </ul>
          <Link
            to="/projects"
            onClick={() => setOpen(false)}
            className="mt-1 border-t border-border px-2 pt-2 text-xs text-muted-foreground hover:text-foreground"
          >
            All projects
          </Link>
        </PopoverContent>
      </Popover>
    </div>
  );
}
