import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Activity as ActivityIcon,
  ArrowLeft,
  ArrowRight,
  Boxes,
  CheckCircle2,
  Cpu,
  FolderKanban,
  FolderOpen,
  Globe,
  HardDrive,
  MemoryStick,
  Play,
  RotateCw,
  Server,
  Square,
  TerminalSquare,
  Zap,
  type LucideIcon,
} from "lucide-react";
import { Link, NavLink, Outlet, useOutletContext, useParams } from "react-router-dom";
import type { RouteObject } from "react-router-dom";

import * as ApplicationsService from "../../bindings/locafy.dev/desktop/internal/applications/service";
import type { Project } from "../../bindings/locafy.dev/desktop/internal/projects/models";
import * as ProjectsService from "../../bindings/locafy.dev/desktop/internal/projects/service";
import * as StatusService from "../../bindings/locafy.dev/desktop/internal/status/service";
import { LifecycleBadge } from "@/components/lifecycle-badge";
import { PlannedTag } from "@/components/app-shell";
import { lifecycleKey, lifecyclesKey, useProjectLifecycle } from "@/lib/lifecycle-query";
import { notificationsKey } from "@/components/top-bar-status";
import { Badge } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { useProjects } from "@/lib/projects-query";
import { cn } from "@/lib/utils";
import { ApplicationsSection } from "./applications";
import {
  ProjectServicesTab,
  ProjectDomainsTab,
  ProjectEnvironmentTab,
  ProjectLogsTab,
  ProjectTerminalTab,
  ProjectProcessesTab,
  ProjectSettingsTab,
} from "./project-tabs";

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
  const lifecycle = useProjectLifecycle(projectId ?? "");

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
              <LifecycleBadge state={lifecycle.data ?? project.LifecycleState} />
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

function gigabytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return "0 GB";
  const value = bytes / 1024 / 1024 / 1024;
  return `${value >= 100 ? value.toFixed(0) : value.toFixed(1)} GB`;
}

function ago(iso: string): string {
  const created = Date.parse(iso);
  if (Number.isNaN(created)) return "";
  const minutes = Math.max(0, Math.floor((Date.now() - created) / 60_000));
  if (minutes < 1) return "just now";
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return days < 30 ? `${days}d ago` : formatDate(iso);
}

function Meter({ label, value, percent, icon: Icon }: { label: string; value: string; percent: number; icon: LucideIcon }) {
  const used = Math.min(100, Math.max(0, Math.round(percent || 0)));
  return (
    <div className="space-y-1.5">
      <div className="flex items-center justify-between gap-3 text-xs">
        <span className="flex items-center gap-1.5 text-muted-foreground">
          <Icon className="size-3.5" aria-hidden />
          {label}
        </span>
        <span className="tabular-nums">{value}</span>
      </div>
      <div className="h-1.5 overflow-hidden rounded-full bg-muted">
        <div
          className={cn(
            "h-full rounded-full transition-all",
            used >= 85 ? "bg-destructive" : used >= 70 ? "bg-accent" : "bg-primary",
          )}
          style={{ width: `${used}%` }}
        />
      </div>
    </div>
  );
}

function StatTile({
  icon: Icon,
  tone,
  value,
  label,
  detail,
  planned,
  detailMuted,
}: {
  icon: LucideIcon;
  tone: string;
  value: string;
  label: string;
  detail: React.ReactNode;
  planned?: boolean;
  detailMuted?: boolean;
}) {
  return (
    <Card role="group" aria-label={`${label}: ${value}`}>
      <CardContent className="flex items-center gap-3">
        <span className={cn("flex size-11 shrink-0 items-center justify-center rounded-xl", tone)}>
          <Icon className="size-5" aria-hidden />
        </span>
        <div className="min-w-0">
          <p className="text-xs text-muted-foreground">{label}</p>
          <p className="text-xl font-semibold leading-tight tracking-tight">{value}</p>
          <p
            className={cn(
              "flex items-center gap-1 text-xs",
              planned || detailMuted ? "text-muted-foreground" : "text-success",
            )}
          >
            {detail}
          </p>
        </div>
      </CardContent>
    </Card>
  );
}

function HealthChip({
  icon: Icon,
  label,
  value,
  tone,
}: {
  icon: LucideIcon;
  label: string;
  value: string;
  tone: "ok" | "bad" | "muted";
}) {
  return (
    <div className="flex items-center gap-2.5 rounded-lg border border-border px-3 py-2">
      <span
        className={cn(
          "flex size-8 shrink-0 items-center justify-center rounded-lg",
          tone === "ok" && "bg-success/15 text-success",
          tone === "bad" && "bg-destructive/15 text-destructive",
          tone === "muted" && "bg-muted text-muted-foreground",
        )}
      >
        <Icon className="size-4" aria-hidden />
      </span>
      <div className="min-w-0">
        <p className="text-[0.7rem] uppercase tracking-wide text-muted-foreground">{label}</p>
        <p className="truncate text-xs font-medium">{value}</p>
      </div>
    </div>
  );
}

/*
 * The project's command centre: what the project is, what it owns, and the
 * machine state behind it. Applications are the only records that exist today,
 * so services, domains and tunnels are shown as planned rather than invented.
 */
export function ProjectOverview() {
  const project = useOutletContext<Project>();
  const queryClient = useQueryClient();

  const applications = useQuery({
    queryKey: ["applications", project.ID],
    queryFn: async () => (await ApplicationsService.List(project.ID)) ?? [],
  });
  const lifecycle = useProjectLifecycle(project.ID);
  const refreshAfterLifecycle = () => {
    void queryClient.invalidateQueries({ queryKey: ["applications", project.ID] });
    void queryClient.invalidateQueries({ queryKey: lifecycleKey(project.ID) });
    void queryClient.invalidateQueries({ queryKey: lifecyclesKey });
    void queryClient.invalidateQueries({ queryKey: notificationsKey });
  };
  const startProject = useMutation({
    mutationFn: () => ApplicationsService.StartProject(project.ID),
    onSuccess: refreshAfterLifecycle,
  });
  const stopProject = useMutation({
    mutationFn: () => ApplicationsService.StopProject(project.ID),
    onSuccess: refreshAfterLifecycle,
  });
  const restartProject = useMutation({
    mutationFn: () => ApplicationsService.RestartProject(project.ID),
    onSuccess: refreshAfterLifecycle,
  });
  const lifecyclePending =
    startProject.isPending || stopProject.isPending || restartProject.isPending;
  const lifecycleError = startProject.error ?? stopProject.error ?? restartProject.error;
  const health = useQuery({
    queryKey: ["status", "health"],
    queryFn: () => StatusService.Health(),
    refetchInterval: 15_000,
  });
  const activity = useQuery({
    queryKey: ["projects", project.ID, "activity"],
    queryFn: async () => (await ProjectsService.Activity(project.ID, 6)) ?? [],
    refetchInterval: 30_000,
  });
  const openFolder = useMutation({ mutationFn: () => ProjectsService.OpenFolder(project.ID) });

  const apps = applications.data ?? [];
  const withStart = apps.filter((app) => app.StartCommand.trim() !== "").length;
  const machine = health.data;
  const diskPercent = machine?.Disk.Percent ?? 0;
  const entries = activity.data ?? [];

  return (
    <div className="mx-auto w-full max-w-6xl space-y-4 px-6 py-6">
      <section className="relative overflow-hidden rounded-2xl border border-border bg-card">
        <div className="pointer-events-none absolute inset-0 bg-gradient-to-r from-primary/20 via-transparent to-accent/20" />
        <div className="pointer-events-none absolute -right-20 -top-24 size-64 rounded-full bg-primary/20 blur-3xl" />
        <div className="relative flex flex-wrap items-start justify-between gap-6 p-6">
          <div className="flex min-w-0 items-start gap-4">
            <span className="flex size-14 shrink-0 items-center justify-center rounded-2xl bg-primary/15 text-primary">
              <FolderKanban className="size-7" />
            </span>
            <div className="min-w-0 space-y-1.5">
              <div className="flex flex-wrap items-center gap-2">
                <h2 className="truncate text-xl font-semibold tracking-tight">{project.Name}</h2>
                <LifecycleBadge state={lifecycle.data ?? project.LifecycleState} />
                <Badge variant={project.Kind === "managed" ? "secondary" : "outline"}>
                  {project.Kind === "managed" ? "Managed" : "Linked"}
                </Badge>
              </div>
              <p className="max-w-xl text-sm text-muted-foreground">
                {project.Description ||
                  (project.Kind === "managed"
                    ? "A project Locafy created in its workspace."
                    : "An existing folder Locafy points at.")}
              </p>
              <p className="truncate font-mono text-xs text-muted-foreground" title={project.RootPath}>
                {project.RootPath}
              </p>
              <div className="flex flex-wrap items-center gap-2 pt-1.5">
                <Button
                  size="sm"
                  variant="secondary"
                  disabled={openFolder.isPending}
                  onClick={() => openFolder.mutate()}
                >
                  <FolderOpen /> Open folder
                </Button>
                <Button size="sm" variant="outline" disabled={lifecyclePending} onClick={() => startProject.mutate()}>
                  <Play /> {startProject.isPending ? "Starting…" : "Start project"}
                </Button>
                <Button size="sm" variant="outline" disabled={lifecyclePending} onClick={() => stopProject.mutate()}>
                  <Square /> {stopProject.isPending ? "Stopping…" : "Stop project"}
                </Button>
                <Button size="sm" variant="outline" disabled={lifecyclePending} onClick={() => restartProject.mutate()}>
                  <RotateCw /> {restartProject.isPending ? "Restarting…" : "Restart"}
                </Button>
                <span className="flex items-center gap-1 text-[0.7rem] uppercase tracking-wide text-muted-foreground">
                  Lifecycle
                </span>
              </div>
              {lifecycleError && (
                <p role="alert" className="text-xs text-destructive">
                  The project action failed: {String(lifecycleError)}
                </p>
              )}
              {openFolder.error && (
                <p role="alert" className="text-xs text-destructive">
                  Could not open the folder: {String(openFolder.error)}
                </p>
              )}
            </div>
          </div>

          <div className="grid w-full shrink-0 gap-3 sm:w-64">
            <Meter
              label="CPU"
              icon={Cpu}
              value={`${Math.round(machine?.CPUPercent ?? 0)}%`}
              percent={machine?.CPUPercent ?? 0}
            />
            <Meter
              label="Memory"
              icon={MemoryStick}
              value={`${gigabytes(machine?.Memory.UsedBytes ?? 0)} / ${gigabytes(machine?.Memory.TotalBytes ?? 0)}`}
              percent={machine?.Memory.Percent ?? 0}
            />
            <Meter
              label="Disk"
              icon={HardDrive}
              value={`${gigabytes(machine?.Disk.UsedBytes ?? 0)} / ${gigabytes(machine?.Disk.TotalBytes ?? 0)}`}
              percent={diskPercent}
            />
          </div>
        </div>
      </section>

      <section className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <StatTile
          icon={Boxes}
          tone="bg-primary/15 text-primary"
          value={String(apps.length)}
          label="Applications"
          detail={`${withStart} with a start command`}
          detailMuted={apps.length === 0}
        />
        <StatTile
          icon={Server}
          tone="bg-muted text-muted-foreground"
          value="0"
          label="Services"
          detail="Planned for a later phase"
          planned
        />
        <StatTile
          icon={Globe}
          tone="bg-muted text-muted-foreground"
          value="0"
          label="Domains"
          detail="Planned for a later phase"
          planned
        />
        <StatTile
          icon={Zap}
          tone="bg-muted text-muted-foreground"
          value="0"
          label="Tunnels"
          detail="Planned for a later phase"
          planned
        />
      </section>

      <div className="grid gap-4 xl:grid-cols-[minmax(0,1.55fr)_minmax(0,1fr)]">
        <div className="space-y-4">
          <Card>
            <CardHeader className="flex flex-row items-start justify-between gap-3">
              <div>
                <CardTitle className="text-base">Project status</CardTitle>
                <CardDescription>
                  What this project has, and the state of the machine behind it.
                </CardDescription>
              </div>
              <Badge variant={machine?.Healthy ? "secondary" : "outline"} className="shrink-0">
                {machine ? (machine.Healthy ? "Healthy" : "Needs attention") : "Checking…"}
              </Badge>
            </CardHeader>
            <CardContent className="space-y-3">
              <div className="grid gap-2 sm:grid-cols-2">
                <HealthChip
                  icon={Boxes}
                  label="Applications"
                  value={apps.length === 0 ? "None recorded" : `${apps.length} recorded`}
                  tone={apps.length > 0 ? "ok" : "muted"}
                />
                <HealthChip
                  icon={Server}
                  label="Services"
                  value="Planned for a later phase"
                  tone="muted"
                />
                <HealthChip
                  icon={ActivityIcon}
                  label="Machine"
                  value={machine ? machine.Summary || (machine.Healthy ? "Healthy" : "Needs attention") : "Checking…"}
                  tone={machine?.Healthy ? "ok" : "bad"}
                />
                <HealthChip
                  icon={HardDrive}
                  label="Disk"
                  value={`${gigabytes(machine?.Disk.UsedBytes ?? 0)} of ${gigabytes(machine?.Disk.TotalBytes ?? 0)} used`}
                  tone={diskPercent < 85 ? "ok" : "bad"}
                />
              </div>
              <p className="text-xs text-muted-foreground">
                {project.Kind === "managed"
                  ? "Managed — Locafy created this folder in its workspace."
                  : "Linked — an existing folder. Locafy never writes into it."}
              </p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="flex flex-row items-start justify-between gap-3">
              <div>
                <CardTitle className="text-base">Applications</CardTitle>
                <CardDescription>What runs in this project, and where.</CardDescription>
              </div>
              <Link
                to={`/projects/${project.ID}/applications`}
                className={cn(buttonVariants({ variant: "secondary", size: "sm" }), "shrink-0")}
              >
                Add application
                <ArrowRight />
              </Link>
            </CardHeader>
            <CardContent>
              {applications.isPending ? (
                <p className="text-sm text-muted-foreground">Reading applications…</p>
              ) : applications.error ? (
                <p role="alert" className="text-sm text-destructive">
                  Could not read applications: {String(applications.error)}
                </p>
              ) : apps.length === 0 ? (
                <div className="rounded-lg border border-dashed border-border px-4 py-8 text-center">
                  <p className="text-sm font-medium">No applications yet</p>
                  <p className="mx-auto mt-1 max-w-sm text-xs text-muted-foreground">
                    Add one from a framework or from the folders already in this project.
                  </p>
                </div>
              ) : (
                <div className="overflow-x-auto">
                  <table className="w-full text-left text-sm">
                    <thead className="text-xs text-muted-foreground">
                      <tr className="border-b border-border">
                        <th className="py-2 pr-3 font-medium">Name</th>
                        <th className="py-2 pr-3 font-medium">Framework</th>
                        <th className="py-2 pr-3 font-medium">Folder</th>
                        <th className="py-2 pr-3 font-medium">Start</th>
                        <th className="py-2 font-medium">Status</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-border">
                      {apps.map((app) => (
                        <tr key={app.ID}>
                          <td className="py-2.5 pr-3 font-medium">{app.Name}</td>
                          <td className="py-2.5 pr-3 text-xs text-muted-foreground">
                            {app.Framework || app.Language || "—"}
                          </td>
                          <td className="max-w-40 truncate py-2.5 pr-3 font-mono text-xs text-muted-foreground">
                            {app.Directory}
                          </td>
                          <td className="max-w-48 truncate py-2.5 pr-3 font-mono text-xs text-muted-foreground">
                            {app.StartCommand || "—"}
                          </td>
                          <td className="py-2.5 text-xs capitalize text-muted-foreground">{app.Status}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </CardContent>
          </Card>
        </div>

        <div className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Recent activity</CardTitle>
              <CardDescription>What Locafy has done with this project.</CardDescription>
            </CardHeader>
            <CardContent>
              {activity.isPending ? (
                <p className="text-sm text-muted-foreground">Reading activity…</p>
              ) : activity.error ? (
                <p role="alert" className="text-sm text-destructive">
                  Could not read activity: {String(activity.error)}
                </p>
              ) : entries.length === 0 ? (
                <p className="text-sm text-muted-foreground">Nothing recorded yet.</p>
              ) : (
                <ul className="space-y-3">
                  {entries.map((entry) => (
                    <li key={entry.ID} className="flex gap-2.5">
                      <span className="mt-1.5 size-1.5 shrink-0 rounded-full bg-primary" aria-hidden />
                      <div className="min-w-0">
                        <p className="text-xs">
                          <span className="font-medium">{entry.Action}</span>
                          {entry.Detail && (
                            <span className="text-muted-foreground"> — {entry.Detail}</span>
                          )}
                        </p>
                        <p className="text-[0.7rem] text-muted-foreground">
                          {ago(String(entry.CreatedAt))}
                        </p>
                      </div>
                    </li>
                  ))}
                </ul>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <TerminalSquare className="size-4" /> Project terminal
              </CardTitle>
              <CardDescription>An embedded shell in this project's folder.</CardDescription>
            </CardHeader>
            <CardContent className="space-y-2">
              <p className="text-xs text-muted-foreground">
                The built-in terminal arrives in a later phase. Until then, open the folder in your
                own terminal and run commands there.
              </p>
              <div className="flex items-center gap-2 text-[0.7rem] uppercase tracking-wide text-muted-foreground">
                <PlannedTag className="ml-0" />
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="text-base">Project details</CardTitle>
            </CardHeader>
            <CardContent>
              <dl className="divide-y divide-border text-sm">
                <Detail label="State">
                  <LifecycleBadge state={lifecycle.data ?? project.LifecycleState} />
                </Detail>
                <Detail label="Configuration">
                  <span className="capitalize">{project.Status}</span>
                </Detail>
                <Detail label="Created">{formatDate(project.CreatedAt)}</Detail>
                <Detail label="Updated">{formatDate(project.UpdatedAt)}</Detail>
              </dl>
              <p className="mt-3 flex items-center gap-1.5 text-xs text-muted-foreground">
                <CheckCircle2 className="size-3.5 text-success" aria-hidden />
                Removing the project from Locafy never deletes this folder.
              </p>
            </CardContent>
          </Card>
        </div>
      </div>
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
    { path: "services", element: <ProjectServicesTab /> },
    { path: "domains", element: <ProjectDomainsTab /> },
    { path: "tunnels", element: <ProjectSection name="Tunnels" planned /> },
    { path: "environment", element: <ProjectEnvironmentTab /> },
    { path: "logs", element: <ProjectLogsTab /> },
    { path: "terminal", element: <ProjectTerminalTab /> },
    { path: "processes", element: <ProjectProcessesTab /> },
    { path: "settings", element: <ProjectSettingsTab /> },
  ],
};
