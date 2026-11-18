import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowRight, CalendarDays, ChevronDown, CircleExclamationPoint, FolderInput, FolderKanban, FolderOpen, FolderPlus, Link2, Pause, Pencil, Play, Plus, Trash2, type LucideIcon } from "lucide-react";
import { Link } from "react-router-dom";

import * as DialogsService from "../../bindings/locafy.dev/desktop/internal/dialogs/service";
import * as ProjectsService from "../../bindings/locafy.dev/desktop/internal/projects/service";
import type { Project } from "../../bindings/locafy.dev/desktop/internal/projects/models";
import { LifecycleBadge } from "@/components/lifecycle-badge";
import { notificationsKey } from "@/components/top-bar-status";
import { ScaffoldForm } from "@/components/scaffold-form";
import { ImportDialog } from "./projects-import";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Badge } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { projectsKey, useProjects } from "@/lib/projects-query";
import { cn } from "@/lib/utils";

const workspaceKey = ["projects", "workspace"] as const;

/** A stable decorative accent per project; status itself is always words. */
const accents = ["border-l-primary", "border-l-accent", "border-l-success", "border-l-muted-foreground"];
function accentFor(id: string): string {
  let hash = 0;
  for (const char of id) hash = (hash * 31 + char.charCodeAt(0)) | 0;
  return accents[Math.abs(hash) % accents.length];
}

function createdAge(iso: string): string {
  const created = Date.parse(iso);
  if (Number.isNaN(created)) return "";
  const minutes = Math.max(0, Math.floor((Date.now() - created) / 60_000));
  if (minutes < 1) return "just now";
  if (minutes < 60) return `${minutes} ${minutes === 1 ? "minute" : "minutes"} ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours} ${hours === 1 ? "hour" : "hours"} ago`;
  const days = Math.floor(hours / 24);
  if (days < 30) return `${days} ${days === 1 ? "day" : "days"} ago`;
  const months = Math.floor(days / 30);
  if (months < 12) return `${months} ${months === 1 ? "month" : "months"} ago`;
  const years = Math.floor(days / 365);
  return `${years} ${years === 1 ? "year" : "years"} ago`;
}

function StatTile({
  icon: Icon,
  tone,
  value,
  label,
}: {
  icon: LucideIcon;
  tone: string;
  value: number;
  label: string;
}) {
  return (
    <Card role="group" aria-label={`${label}: ${value}`}>
      <CardContent className="flex items-center gap-4">
        <Icon className={cn("size-7 shrink-0", tone)} aria-hidden />
        <div>
          <p className="text-2xl font-semibold leading-none">{value}</p>
          <p className="mt-1.5 text-sm text-muted-foreground">{label}</p>
        </div>
      </CardContent>
    </Card>
  );
}

/*
 * Which dialog is open. One at a time keeps the screen's state small: the
 * project an action targets is carried here rather than duplicated per row.
 */
type Editing =
  | { kind: "create" }
  | { kind: "scaffold" }
  | { kind: "link" }
  | { kind: "import" }
  | { kind: "rename"; project: Project }
  | { kind: "root"; project: Project }
  | { kind: "remove"; project: Project }
  | null;

export function ProjectsScreen() {
  const queryClient = useQueryClient();
  const [editing, setEditing] = useState<Editing>(null);
  const [value, setValue] = useState("");
  const [failure, setFailure] = useState<string | null>(null);
  const [filter, setFilter] = useState("");
  const [choosing, setChoosing] = useState(false);

  const projects = useProjects();

  const workspace = useQuery({
    queryKey: workspaceKey,
    queryFn: () => ProjectsService.WorkspaceDir(),
  });

  // Every project action is recorded in the activity history the bell reads,
  // so refreshing the list also refreshes its count and entries.
  const refresh = () => {
    void queryClient.invalidateQueries({ queryKey: projectsKey });
    void queryClient.invalidateQueries({ queryKey: notificationsKey });
  };

  const create = useMutation({
    mutationFn: (name: string) => ProjectsService.Create(name),
    onSuccess: refresh,
  });
  const link = useMutation({
    mutationFn: (path: string) => ProjectsService.Link(path),
    onSuccess: refresh,
  });
  const rename = useMutation({
    mutationFn: ({ id, name }: { id: string; name: string }) =>
      ProjectsService.Rename(id, name),
    onSuccess: refresh,
  });
  const changeRoot = useMutation({
    mutationFn: ({ id, path }: { id: string; path: string }) =>
      ProjectsService.ChangeRoot(id, path),
    onSuccess: refresh,
  });
  const remove = useMutation({
    mutationFn: (id: string) => ProjectsService.Remove(id),
    onSuccess: refresh,
  });

  const rows = projects.data ?? [];
  const needle = filter.trim().toLowerCase();
  const visible = needle
    ? rows.filter(
        (project) =>
          project.Name.toLowerCase().includes(needle) ||
          project.RootPath.toLowerCase().includes(needle),
      )
    : rows;
  const lifecycleCount = (state: string) =>
    rows.filter((project) => project.LifecycleState === state).length;

  const busy = create.isPending || link.isPending || rename.isPending || changeRoot.isPending || remove.isPending;

  const open = (next: Editing, initial = "") => {
    setEditing(next);
    setValue(initial);
    setFailure(null);
  };

  // The native dialog returns an empty path when it is cancelled, which leaves
  // whatever was typed in place.
  const browse = async (title: string) => {
    setChoosing(true);
    setFailure(null);
    try {
      const picked = await DialogsService.ChooseFolder(title);
      if (picked) setValue(picked);
    } catch (error) {
      setFailure(String(error));
    } finally {
      setChoosing(false);
    }
  };

  const close = () => {
    setEditing(null);
    setValue("");
    setFailure(null);
  };

  const submit = (event: React.FormEvent) => {
    event.preventDefault();
    if (!editing || busy) return;
    const trimmed = value.trim();
    const failed = (error: unknown) => setFailure(String(error));

    switch (editing.kind) {
      case "create":
        if (!trimmed) return;
        create.mutate(trimmed, { onSuccess: close, onError: failed });
        break;
      case "link":
        if (!trimmed) return;
        link.mutate(trimmed, { onSuccess: close, onError: failed });
        break;
      case "rename":
        if (!trimmed) return;
        rename.mutate(
          { id: editing.project.ID, name: trimmed },
          { onSuccess: close, onError: failed },
        );
        break;
      case "root":
        if (!trimmed) return;
        changeRoot.mutate(
          { id: editing.project.ID, path: trimmed },
          { onSuccess: close, onError: failed },
        );
        break;
      case "remove":
        remove.mutate(editing.project.ID, { onSuccess: close, onError: failed });
        break;
    }
  };

  return (
    <div className="mx-auto w-full max-w-6xl px-6 py-10">
      <header className="mb-8 flex flex-wrap items-end justify-between gap-4">
        <div className="space-y-1">
          <h1 className="text-2xl font-semibold tracking-tight">Projects</h1>
          <p className="text-sm text-muted-foreground">
            {rows.length === 0
              ? "Projects group the applications and services Locafy runs."
              : needle
                ? `${visible.length} of ${rows.length} projects`
                : `${rows.length} ${rows.length === 1 ? "project" : "projects"}`}
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => open({ kind: "link" })}>
            <Link2 /> Link folder
          </Button>
          <Button variant="outline" onClick={() => open({ kind: "import" })}>
            <FolderInput /> Import folder
          </Button>
          <Button onClick={() => open({ kind: "create" })}>
            <FolderPlus /> New project
          </Button>
        </div>
      </header>

      {rows.length > 0 && (
        <div className="mb-6 grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
          <StatTile icon={FolderKanban} tone="text-primary" value={rows.length} label="Total Projects" />
          <StatTile icon={Play} tone="text-success" value={lifecycleCount("running")} label="Running" />
          <StatTile icon={Pause} tone="text-accent" value={lifecycleCount("stopped")} label="Stopped" />
          <StatTile
            icon={CircleExclamationPoint}
            tone="text-destructive"
            value={lifecycleCount("error")}
            label="With Issues"
          />
        </div>
      )}

      {rows.length > 0 && (
        <Input
          aria-label="Filter projects"
          type="search"
          className="mb-5 max-w-sm"
          placeholder="Filter by name or folder"
          value={filter}
          onChange={(event) => setFilter(event.target.value)}
        />
      )}

      {projects.isPending ? (
        <p className="text-sm text-muted-foreground">Loading projects…</p>
      ) : projects.error ? (
        <p role="alert" className="text-sm text-destructive">
          Could not read projects: {String(projects.error)}
        </p>
      ) : rows.length === 0 ? (
        <div className="rounded-xl border border-dashed border-border px-6 py-16 text-center">
          <FolderKanban className="mx-auto size-8 text-muted-foreground" />
          <p className="mt-3 text-sm font-medium">No projects yet</p>
          <p className="mt-1 text-sm text-muted-foreground">
            Create one inside the Locafy workspace, or link a folder you already work in.
          </p>
        </div>
      ) : visible.length === 0 ? (
        <p className="text-sm text-muted-foreground">
          No project matches “{filter.trim()}”.
        </p>
      ) : (
        <ul className="grid gap-4 sm:grid-cols-2">
          {visible.map((project) => (
            <li key={project.ID}>
              <Card className={cn("h-full border-l-2", accentFor(project.ID))}>
                <CardContent className="flex h-full flex-col gap-3">
                  <div className="flex items-start gap-3">
                    <span className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-primary/15 text-primary">
                      <FolderKanban className="size-5" />
                    </span>
                    <div className="min-w-0 flex-1">
                      <Link
                        to={`/projects/${project.ID}`}
                        className="block truncate font-medium hover:underline"
                      >
                        {project.Name}
                      </Link>
                      {project.Description && (
                        <p className="mt-0.5 line-clamp-2 text-xs text-muted-foreground">
                          {project.Description}
                        </p>
                      )}
                    </div>
                    <LifecycleBadge state={project.LifecycleState} />
                  </div>

                  <div className="flex flex-wrap items-center gap-1.5">
                    <Badge variant={project.Kind === "managed" ? "secondary" : "outline"}>
                      {project.Kind === "managed" ? "Managed" : "Linked"}
                    </Badge>
                  </div>

                  <div className="flex items-center gap-2 text-xs text-muted-foreground">
                    <span title={project.RootPath} className="flex min-w-0 items-center gap-1">
                      <FolderKanban className="size-3.5 shrink-0" aria-hidden />
                      <span className="truncate font-mono">{project.RootPath}</span>
                    </span>
                    <span aria-hidden className="shrink-0">
                      |
                    </span>
                    <span className="flex shrink-0 items-center gap-1">
                      <CalendarDays className="size-3.5" aria-hidden />
                      Created {createdAge(project.CreatedAt)}
                    </span>
                  </div>

                  <div className="mt-auto flex items-center justify-between gap-2 border-t border-border pt-3">
                    <Link
                      to={`/projects/${project.ID}`}
                      aria-label={`Open ${project.Name}`}
                      className={cn(buttonVariants({ size: "sm" }))}
                    >
                      Open
                      <ArrowRight />
                    </Link>
                    <DropdownMenu>
                      <DropdownMenuTrigger
                        aria-label={`More for ${project.Name}`}
                        className={cn(buttonVariants({ variant: "ghost", size: "sm" }))}
                      >
                        More
                        <ChevronDown />
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem onClick={() => open({ kind: "rename", project }, project.Name)}>
                          <Pencil /> Rename
                        </DropdownMenuItem>
                        {project.Kind === "linked" && (
                          <DropdownMenuItem onClick={() => open({ kind: "root", project }, project.RootPath)}>
                            <Link2 /> Change folder
                          </DropdownMenuItem>
                        )}
                        <DropdownMenuItem
                          variant="destructive"
                          onClick={() => open({ kind: "remove", project })}
                        >
                          <Trash2 /> Remove
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </div>
                </CardContent>
              </Card>
            </li>
          ))}
          <li>
            <Card className="h-full border-dashed">
              <CardContent className="flex h-full flex-col items-center justify-center gap-3 px-6 py-10 text-center">
                <span className="flex size-14 items-center justify-center rounded-xl border border-dashed border-border">
                  <Plus className="size-6 text-muted-foreground" aria-hidden />
                </span>
                <p className="font-medium">Quick Start</p>
                <p className="text-sm text-muted-foreground">
                  Create a new project and get started with Locafy.
                </p>
                <Button className="mt-1" onClick={() => open({ kind: "create" })}>
                  <FolderPlus /> New Project
                </Button>
              </CardContent>
            </Card>
          </li>
        </ul>
      )}

      {workspace.data && (
        <p className="mt-4 text-xs text-muted-foreground">
          New projects are created in <span className="font-mono">{workspace.data}</span>. Remove
          only takes a project out of Locafy — its folder and files stay where they are.
        </p>
      )}

      <Dialog open={editing?.kind === "create" || editing?.kind === "link"} onOpenChange={(next) => !next && close()}>
        <DialogContent>
          <form onSubmit={submit}>
            <DialogHeader>
              <DialogTitle>
                {editing?.kind === "create" ? "New project" : "Link an existing folder"}
              </DialogTitle>
              <DialogDescription>
                {editing?.kind === "create"
                  ? "Locafy creates the folder in its workspace and keeps its settings in Locafy."
                  : "Locafy records the folder's location. Nothing inside it is moved or changed."}
              </DialogDescription>
            </DialogHeader>

            {editing?.kind === "create" && (
              <div className="mt-4 space-y-2">
                <span className="text-xs font-medium text-muted-foreground">Start from</span>
                <div className="flex flex-wrap gap-2">
                  <Button type="button" size="sm" variant="secondary" onClick={() => open({ kind: "create" })}>
                    Blank folder
                  </Button>
                  <Button type="button" size="sm" variant="outline" onClick={() => open({ kind: "link" })}>
                    Existing folder
                  </Button>
                  <Button type="button" size="sm" variant="outline" onClick={() => open({ kind: "scaffold" })}>
                    Framework
                  </Button>
                </div>
              </div>
            )}

            <div className="mt-4">
              <label className="text-sm font-medium" htmlFor="project-value">
                {editing?.kind === "create" ? "Project name" : "Folder path"}
              </label>
              <div className="mt-1.5 flex gap-2">
                <Input
                  id="project-value"
                  className="min-w-0 flex-1"
                  autoFocus
                  placeholder={editing?.kind === "create" ? "Etugora" : "C:\\work\\etugora"}
                  value={value}
                  onChange={(event) => setValue(event.target.value)}
                />
                {editing?.kind === "link" && (
                  <Button
                    type="button"
                    variant="outline"
                    disabled={choosing}
                    onClick={() => browse("Link an existing folder")}
                  >
                    <FolderOpen /> Browse…
                  </Button>
                )}
              </div>
            </div>

            {failure && (
              <p role="alert" className="mt-3 text-sm text-destructive">
                {failure}
              </p>
            )}

            <DialogFooter className="mt-5">
              <Button type="button" variant="outline" onClick={close}>
                Cancel
              </Button>
              <Button type="submit" disabled={value.trim() === "" || busy}>
                {editing?.kind === "create" ? "Create project" : "Link folder"}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <Dialog open={editing?.kind === "scaffold"} onOpenChange={(next) => !next && close()}>
        <DialogContent className="sm:max-w-xl">
          <DialogHeader>
            <DialogTitle>New project from a framework</DialogTitle>
            <DialogDescription>
              Locafy runs the framework's own installer in a new workspace folder and registers the
              project only once its files are there.
            </DialogDescription>
          </DialogHeader>
          <div className="mt-4 max-h-[60vh] overflow-y-auto pr-1">
            <ScaffoldForm
              mode="project"
              onDone={() => {
                refresh();
                close();
              }}
              onCancel={close}
            />
          </div>
        </DialogContent>
      </Dialog>

      <Dialog
        open={editing?.kind === "rename" || editing?.kind === "root"}
        onOpenChange={(next) => !next && close()}
      >
        <DialogContent>
          <form onSubmit={submit}>
            <DialogHeader>
              <DialogTitle>
                {editing?.kind === "rename" ? "Rename project" : "Change project folder"}
              </DialogTitle>
              <DialogDescription>
                {editing?.kind === "rename"
                  ? "The folder on disk keeps its current name."
                  : "Locafy points at the new folder. Files are never moved, copied or deleted."}
              </DialogDescription>
            </DialogHeader>

            <div className="mt-4">
              <label className="text-sm font-medium" htmlFor="project-value">
                {editing?.kind === "rename" ? "Project name" : "Folder path"}
              </label>
              <div className="mt-1.5 flex gap-2">
                <Input
                  id="project-value"
                  className="min-w-0 flex-1"
                  autoFocus
                  value={value}
                  onChange={(event) => setValue(event.target.value)}
                />
                {editing?.kind === "root" && (
                  <Button
                    type="button"
                    variant="outline"
                    disabled={choosing}
                    onClick={() => browse("Change project folder")}
                  >
                    <FolderOpen /> Browse…
                  </Button>
                )}
              </div>
            </div>

            {failure && (
              <p role="alert" className="mt-3 text-sm text-destructive">
                {failure}
              </p>
            )}

            <DialogFooter className="mt-5">
              <Button type="button" variant="outline" onClick={close}>
                Cancel
              </Button>
              <Button type="submit" disabled={value.trim() === "" || busy}>
                Save
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <AlertDialog
        open={editing?.kind === "remove"}
        onOpenChange={(next) => !next && close()}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              Remove {editing?.kind === "remove" ? editing.project.Name : "project"}?
            </AlertDialogTitle>
            <AlertDialogDescription>
              The project leaves Locafy. Its folder and files stay on disk — nothing is deleted.
            </AlertDialogDescription>
          </AlertDialogHeader>
          {failure && (
            <p role="alert" className="text-sm text-destructive">
              {failure}
            </p>
          )}
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction disabled={busy} onClick={submit}>
              Remove
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <ImportDialog
        open={editing?.kind === "import"}
        onClose={close}
        onImported={refresh}
      />
    </div>
  );
}
