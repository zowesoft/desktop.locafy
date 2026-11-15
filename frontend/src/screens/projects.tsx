import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { FolderKanban, FolderPlus, Link2, Pencil, Trash2 } from "lucide-react";

import * as ProjectsService from "../../bindings/locafy.dev/desktop/internal/projects/service";
import type { Project } from "../../bindings/locafy.dev/desktop/internal/projects/models";
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
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";

const projectsKey = ["projects"] as const;
const workspaceKey = ["projects", "workspace"] as const;

/*
 * Which dialog is open. One at a time keeps the screen's state small: the
 * project an action targets is carried here rather than duplicated per row.
 */
type Editing =
  | { kind: "create" }
  | { kind: "link" }
  | { kind: "rename"; project: Project }
  | { kind: "root"; project: Project }
  | { kind: "remove"; project: Project }
  | null;

export function ProjectsScreen() {
  const queryClient = useQueryClient();
  const [editing, setEditing] = useState<Editing>(null);
  const [value, setValue] = useState("");
  const [failure, setFailure] = useState<string | null>(null);

  const projects = useQuery({
    queryKey: projectsKey,
    queryFn: async () => (await ProjectsService.List()) ?? [],
  });

  const workspace = useQuery({
    queryKey: workspaceKey,
    queryFn: () => ProjectsService.WorkspaceDir(),
  });

  const refresh = () => queryClient.invalidateQueries({ queryKey: projectsKey });

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
  const busy = create.isPending || link.isPending || rename.isPending || changeRoot.isPending || remove.isPending;

  const open = (next: Editing, initial = "") => {
    setEditing(next);
    setValue(initial);
    setFailure(null);
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
    <div className="mx-auto w-full max-w-3xl px-6 py-10">
      <header className="mb-8 flex flex-wrap items-end justify-between gap-4">
        <div className="space-y-1">
          <h1 className="text-2xl font-semibold tracking-tight">Projects</h1>
          <p className="text-sm text-muted-foreground">
            {rows.length === 0
              ? "Projects group the applications and services Locafy runs."
              : `${rows.length} ${rows.length === 1 ? "project" : "projects"}`}
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => open({ kind: "link" })}>
            <Link2 /> Link folder
          </Button>
          <Button onClick={() => open({ kind: "create" })}>
            <FolderPlus /> New project
          </Button>
        </div>
      </header>

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
      ) : (
        <ul className="divide-y divide-border rounded-xl border border-border">
          {rows.map((project) => (
            <li key={project.ID} className="flex flex-wrap items-center gap-3 px-4 py-3">
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <span className="truncate font-medium">{project.Name}</span>
                  <Badge variant={project.Kind === "managed" ? "secondary" : "outline"}>
                    {project.Kind === "managed" ? "Managed" : "Linked"}
                  </Badge>
                </div>
                <p className="mt-0.5 truncate font-mono text-xs text-muted-foreground">
                  {project.RootPath}
                </p>
              </div>
              <div className="flex gap-1">
                <Button
                  variant="ghost"
                  size="icon-sm"
                  aria-label={`Rename ${project.Name}`}
                  onClick={() => open({ kind: "rename", project }, project.Name)}
                >
                  <Pencil />
                </Button>
                {project.Kind === "linked" && (
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    aria-label={`Change folder for ${project.Name}`}
                    onClick={() => open({ kind: "root", project }, project.RootPath)}
                  >
                    <Link2 />
                  </Button>
                )}
                <Button
                  variant="ghost"
                  size="icon-sm"
                  aria-label={`Remove ${project.Name}`}
                  onClick={() => open({ kind: "remove", project })}
                >
                  <Trash2 />
                </Button>
              </div>
            </li>
          ))}
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
                  ? "Locafy creates the folder in its workspace and writes a project manifest."
                  : "Locafy records the folder's location. Nothing inside it is moved or changed."}
              </DialogDescription>
            </DialogHeader>

            <div className="mt-4">
              <label className="text-sm font-medium" htmlFor="project-value">
                {editing?.kind === "create" ? "Project name" : "Folder path"}
              </label>
              <Input
                id="project-value"
                className="mt-1.5"
                autoFocus
                placeholder={editing?.kind === "create" ? "Etugora" : "C:\\work\\etugora"}
                value={value}
                onChange={(event) => setValue(event.target.value)}
              />
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
              <Input
                id="project-value"
                className="mt-1.5"
                autoFocus
                value={value}
                onChange={(event) => setValue(event.target.value)}
              />
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
    </div>
  );
}
