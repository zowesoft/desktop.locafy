import { ArrowLeft, FolderKanban } from "lucide-react";
import { Link, NavLink, Outlet, useOutletContext, useParams } from "react-router-dom";
import type { RouteObject } from "react-router-dom";

import type { Project } from "../../bindings/locafy.dev/desktop/internal/projects/models";
import { LifecycleBadge } from "@/components/lifecycle-badge";
import { PlannedTag } from "@/components/app-shell";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useProjects } from "@/lib/projects-query";
import { cn } from "@/lib/utils";
import { ApplicationsSection } from "./applications";

type Tab = { path: string; label: string; planned?: boolean };

/*
 * Everything scoped to one project is a tab here rather than a global screen,
 * so applications, services and domains are always seen inside the project
 * that owns them.
 */
const tabs: ReadonlyArray<Tab> = [
  { path: "", label: "Overview" },
  { path: "applications", label: "Applications" },
  { path: "services", label: "Services" },
  { path: "domains", label: "Domains" },
  { path: "tunnels", label: "Tunnels", planned: true },
  { path: "environment", label: "Environment" },
  { path: "logs", label: "Logs" },
  { path: "terminal", label: "Terminal" },
  { path: "processes", label: "Processes" },
  { path: "settings", label: "Settings" },
];

function formatDate(iso: string): string {
  const timestamp = Date.parse(iso);
  return Number.isNaN(timestamp) ? iso : new Date(timestamp).toLocaleString();
}

export function ProjectWorkspace() {
  const { projectId } = useParams();
  const projects = useProjects();
  const project = projects.data?.find((candidate) => candidate.ID === projectId);

  if (projects.isPending) {
    return <p className="px-6 py-10 text-sm text-muted-foreground">Loading project…</p>;
  }

  if (projects.error) {
    return (
      <div className="px-6 py-10">
        <p role="alert" className="text-sm text-destructive">
          Could not read projects: {String(projects.error)}
        </p>
        <BackToProjects />
      </div>
    );
  }

  if (!project) {
    return (
      <div className="mx-auto max-w-md px-6 py-20">
        <h1 className="text-xl font-semibold tracking-tight">Project not found</h1>
        <p className="mt-1.5 text-sm text-muted-foreground">
          It may have been removed from Locafy. Its folder and files are untouched.
        </p>
        <BackToProjects />
      </div>
    );
  }

  return (
    <div className="flex min-h-full flex-col">
      <header className="space-y-4 border-b border-border px-6 pt-5">
        <BackToProjects />

        <div className="flex flex-wrap items-start gap-4">
          <span className="flex size-12 shrink-0 items-center justify-center rounded-xl bg-primary/15 text-primary">
            <FolderKanban className="size-6" />
          </span>
          <div className="min-w-0 flex-1 space-y-1">
            <div className="flex flex-wrap items-center gap-2">
              <h1 className="truncate text-2xl font-semibold tracking-tight">{project.Name}</h1>
              <LifecycleBadge state={project.LifecycleState} />
              <Badge variant={project.Kind === "managed" ? "secondary" : "outline"}>
                {project.Kind === "managed" ? "Managed" : "Linked"}
              </Badge>
            </div>
            <p className="truncate font-mono text-xs text-muted-foreground" title={project.RootPath}>
              {project.RootPath}
            </p>
            {project.Description && (
              <p className="text-sm text-muted-foreground">{project.Description}</p>
            )}
          </div>
        </div>

        <nav aria-label="Project sections" className="-mb-px flex gap-1 overflow-x-auto">
          {tabs.map((tab) =>
            tab.planned ? (
              <span
                key={tab.path}
                aria-disabled="true"
                title={`${tab.label} is planned for a later phase`}
                className="flex cursor-default items-center gap-1.5 whitespace-nowrap border-b-2 border-transparent px-3 pb-2.5 text-sm text-muted-foreground/60"
              >
                {tab.label}
                <PlannedTag className="ml-0" />
              </span>
            ) : (
              <NavLink
                key={tab.path}
                to={tab.path ? `/projects/${project.ID}/${tab.path}` : `/projects/${project.ID}`}
                end={tab.path === ""}
                className={({ isActive }) =>
                  cn(
                    "whitespace-nowrap border-b-2 px-3 pb-2.5 text-sm transition-colors",
                    isActive
                      ? "border-primary text-foreground"
                      : "border-transparent text-muted-foreground hover:text-foreground",
                  )
                }
              >
                {tab.label}
              </NavLink>
            ),
          )}
        </nav>
      </header>

      <div className="flex-1">
        <Outlet context={project} />
      </div>
    </div>
  );
}

function BackToProjects() {
  return (
    <Link
      to="/projects"
      className="inline-flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground"
    >
      <ArrowLeft className="size-3.5" />
      Projects
    </Link>
  );
}

function Detail({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex flex-wrap items-baseline gap-x-4 gap-y-0.5 py-2.5">
      <dt className="w-32 shrink-0 text-muted-foreground">{label}</dt>
      <dd className="min-w-0 flex-1 break-words">{children}</dd>
    </div>
  );
}

export function ProjectOverview() {
  const project = useOutletContext<Project>();

  return (
    <div className="mx-auto w-full max-w-3xl space-y-4 px-6 py-6">
      <Card>
        <CardHeader>
          <CardTitle>Project details</CardTitle>
        </CardHeader>
        <CardContent>
          <dl className="divide-y divide-border text-sm">
            <Detail label="Folder">
              <span className="font-mono text-xs">{project.RootPath}</span>
            </Detail>
            <Detail label="Origin">
              {project.Kind === "managed"
                ? "Managed — Locafy created this folder in its workspace."
                : "Linked — an existing folder. Locafy never writes into it."}
            </Detail>
            <Detail label="State">
              <LifecycleBadge state={project.LifecycleState} />
            </Detail>
            <Detail label="Configuration">
              <span className="capitalize">{project.Status}</span>
            </Detail>
            <Detail label="Created">{formatDate(project.CreatedAt)}</Detail>
            <Detail label="Updated">{formatDate(project.UpdatedAt)}</Detail>
          </dl>
        </CardContent>
      </Card>

      <p className="text-xs text-muted-foreground">
        Applications, services and domains are added from their tabs as those features arrive, and
        starting or stopping the project arrives with project lifecycle. Removing the project from
        Locafy never deletes this folder.
      </p>
    </div>
  );
}

export function ProjectSection({ name, planned }: { name: string; planned?: boolean }) {
  return (
    <div className="mx-auto max-w-md px-6 py-20 text-center">
      <h2 className="text-lg font-medium">{name}</h2>
      <p className="mt-1 text-sm text-muted-foreground">
        {planned
          ? "Planned for a later phase. Nothing here is available yet."
          : "This section is not built yet."}
      </p>
    </div>
  );
}

export const projectWorkspaceRoute: RouteObject = {
  path: "projects/:projectId",
  element: <ProjectWorkspace />,
  children: [
    { index: true, element: <ProjectOverview /> },
    { path: "applications", element: <ApplicationsSection /> },
    ...tabs
      .filter((tab) => tab.path !== "" && tab.path !== "applications")
      .map(
        (tab): RouteObject => ({
          path: tab.path,
          element: <ProjectSection name={tab.label} planned={tab.planned} />,
        }),
      ),
  ],
};
