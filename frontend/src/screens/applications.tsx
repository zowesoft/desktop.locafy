import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useOutletContext } from "react-router-dom";

import * as ApplicationsService from "../../bindings/locafy.dev/desktop/internal/applications/service";
import type {
  Application,
  CreateInput,
  Detection,
} from "../../bindings/locafy.dev/desktop/internal/applications/models";
import type { Project } from "../../bindings/locafy.dev/desktop/internal/projects/models";
import { ScaffoldForm } from "@/components/scaffold-form";
import { notificationsKey } from "@/components/top-bar-status";
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

const applicationsKey = (projectID: string) => ["applications", projectID] as const;
const detectionKey = (projectID: string) => ["applications", projectID, "detect"] as const;

/* A draft is what the operator is looking at before it becomes a record: the
 * proposal's fields, freely editable, and nothing saved. */
type Draft = {
  name: string;
  directory: string;
  language: string;
  framework: string;
  marker: string;
  startCommand: string;
  stopCommand: string;
  detected: boolean;
};

const emptyDraft: Draft = {
  name: "",
  directory: "",
  language: "",
  framework: "",
  marker: "",
  startCommand: "",
  stopCommand: "",
  detected: false,
};

function draftFromDetection(detection: Detection): Draft {
  return {
    name: detection.Name,
    directory: detection.Directory,
    language: detection.Language,
    framework: detection.Framework,
    marker: detection.Marker,
    startCommand: detection.StartCommand,
    stopCommand: "",
    detected: true,
  };
}

function inputFor(draft: Draft): CreateInput {
  return {
    Name: draft.name.trim(),
    Directory: draft.directory.trim(),
    Language: draft.language.trim(),
    Framework: draft.framework.trim(),
    Marker: draft.detected ? draft.marker : "",
    StartCommand: draft.startCommand.trim(),
    StopCommand: draft.stopCommand.trim(),
    Source: draft.detected ? "detected" : "manual",
    ScaffoldRecipe: "",
    ScaffoldOptions: "",
  };
}

/*
 * The project's applications: what runs, where, and what has to run first.
 * Detection proposes records from the folders; only a confirmed draft is saved,
 * and no command runs from this screen.
 */
export function ApplicationsSection() {
  const project = useOutletContext<Project>();
  const queryClient = useQueryClient();
  const [adding, setAdding] = useState(false);
  const [scaffolding, setScaffolding] = useState(false);
  const [editingDependencies, setEditingDependencies] = useState<Application | null>(null);

  const applications = useQuery({
    queryKey: applicationsKey(project.ID),
    queryFn: async () => (await ApplicationsService.List(project.ID)) ?? [],
  });

  const refresh = () => {
    void queryClient.invalidateQueries({ queryKey: applicationsKey(project.ID) });
    void queryClient.invalidateQueries({ queryKey: notificationsKey });
  };

  const entries = applications.data ?? [];

  return (
    <div className="mx-auto w-full max-w-4xl space-y-4 px-6 py-6">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="space-y-1">
          <h2 className="text-lg font-medium">Applications</h2>
          <p className="max-w-xl text-sm text-muted-foreground">
            What runs in this project. Locafy proposes applications it finds in the folders; nothing
            is saved or started until you confirm it.
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button variant="outline" onClick={() => setAdding(true)}>
            Add existing
          </Button>
          <Button onClick={() => setScaffolding(true)}>New application</Button>
        </div>
      </div>

      {applications.isPending ? (
        <p className="text-sm text-muted-foreground">Loading applications…</p>
      ) : applications.isError ? (
        <p role="alert" className="text-sm text-destructive">
          Could not read applications: {String(applications.error)}
        </p>
      ) : entries.length === 0 ? (
        <div className="rounded-xl border border-dashed border-border px-6 py-10 text-center">
          <p className="text-sm font-medium">No applications yet</p>
          <p className="mx-auto mt-1 max-w-md text-sm text-muted-foreground">
            Add one by hand, or let Locafy read the project's folders and propose the applications it
            recognizes.
          </p>
        </div>
      ) : (
        <ul className="space-y-3">
          {entries.map((app) => {
            const dependencies = (app.DependsOn ?? [])
              .map((id) => entries.find((candidate) => candidate.ID === id)?.Name)
              .filter((name): name is string => Boolean(name));

            return (
              <li
                key={app.ID}
                className="rounded-xl border border-border bg-card px-4 py-3 text-sm"
              >
                <div className="flex flex-wrap items-center gap-2">
                  <span className="font-medium">{app.Name}</span>
                  {app.Language && <Badge variant="secondary">{app.Language}</Badge>}
                  {app.Framework && <Badge variant="outline">{app.Framework}</Badge>}
                  <Badge variant="outline">{app.Source === "detected" ? "Detected" : "Added"}</Badge>
                  <span className="ml-auto text-xs capitalize text-muted-foreground">{app.Status}</span>
                </div>

                <dl className="mt-2 grid gap-1.5 text-xs sm:grid-cols-[7rem_1fr]">
                  <dt className="text-muted-foreground">Folder</dt>
                  <dd className="break-all font-mono">{app.Directory}</dd>
                  <dt className="text-muted-foreground">Start</dt>
                  <dd className="break-all font-mono">
                    {app.StartCommand || "— none recorded —"}
                  </dd>
                  <dt className="text-muted-foreground">Needs first</dt>
                  <dd>
                    {dependencies.length > 0 ? (
                      dependencies.join(", ")
                    ) : (
                      <span className="text-muted-foreground">Nothing</span>
                    )}
                  </dd>
                </dl>

                <div className="mt-3 flex flex-wrap gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    aria-label={`Edit dependencies of ${app.Name}`}
                    onClick={() => setEditingDependencies(app)}
                  >
                    Dependencies
                  </Button>
                </div>
              </li>
            );
          })}
        </ul>
      )}

      <Dialog open={scaffolding} onOpenChange={(next) => !next && setScaffolding(false)}>
        <DialogContent className="sm:max-w-xl">
          <DialogHeader>
            <DialogTitle>New application from a framework</DialogTitle>
            <DialogDescription>
              The framework's installer runs in a new folder inside this project, and the application
              is recorded only after its files exist.
            </DialogDescription>
          </DialogHeader>
          <div className="mt-4 max-h-[60vh] overflow-y-auto pr-1">
            <ScaffoldForm
              mode="application"
              projectID={project.ID}
              onDone={() => {
                setScaffolding(false);
                refresh();
              }}
              onCancel={() => setScaffolding(false)}
            />
          </div>
        </DialogContent>
      </Dialog>

      <AddApplicationDialog
        project={project}
        open={adding}
        onClose={() => setAdding(false)}
        onSaved={() => {
          setAdding(false);
          refresh();
        }}
      />

      {editingDependencies && (
        <DependenciesDialog
          app={editingDependencies}
          applications={entries}
          onClose={() => setEditingDependencies(null)}
          onSaved={() => {
            setEditingDependencies(null);
            refresh();
          }}
        />
      )}
    </div>
  );
}

function AddApplicationDialog({
  project,
  open,
  onClose,
  onSaved,
}: {
  project: Project;
  open: boolean;
  onClose: () => void;
  onSaved: () => void;
}) {
  const [draft, setDraft] = useState<Draft>(emptyDraft);

  const detections = useQuery({
    queryKey: detectionKey(project.ID),
    queryFn: async () => (await ApplicationsService.Detect(project.ID)) ?? [],
    enabled: open,
  });

  // Each opening starts from the first proposal, or from nothing when the files
  // suggest no application at all.
  useEffect(() => {
    if (!open) return;
    setDraft(emptyDraft);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  useEffect(() => {
    if (!open || !detections.isSuccess) return;
    const first = detections.data?.[0];
    if (first) setDraft(draftFromDetection(first));
    // Only when detection answers: editing must not be undone by a refetch.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, detections.isSuccess, detections.data]);

  const create = useMutation({
    mutationFn: (input: CreateInput) => ApplicationsService.Create(project.ID, input),
    onSuccess: onSaved,
  });

  const proposals = detections.data ?? [];
  const canSubmit = draft.name.trim() !== "" && draft.directory.trim() !== "" && !create.isPending;

  const update = (field: keyof Draft, value: string) =>
    setDraft((current) => ({ ...current, [field]: value }));

  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()}>
      <DialogContent className="sm:max-w-lg">
        <form
          onSubmit={(event) => {
            event.preventDefault();
            if (canSubmit) create.mutate(inputFor(draft));
          }}
        >
          <DialogHeader>
            <DialogTitle>Add an application</DialogTitle>
            <DialogDescription>
              Locafy read the project's folders. Choose a proposal and change anything that is
              wrong, or add one by hand. Nothing is saved until you confirm.
            </DialogDescription>
          </DialogHeader>

          <div className="mt-4 space-y-3">
            <div className="space-y-1.5">
              <span className="text-xs font-medium text-muted-foreground">Detected in the folders</span>
              {detections.isPending ? (
                <p className="text-xs text-muted-foreground">Reading the project's folders…</p>
              ) : detections.isError ? (
                <p role="alert" className="text-xs text-destructive">
                  Could not read the folders: {String(detections.error)}
                </p>
              ) : proposals.length === 0 ? (
                <p className="text-xs text-muted-foreground">
                  Nothing recognizable was found. Add the application by hand below.
                </p>
              ) : (
                <ul className="space-y-1.5">
                  {proposals.map((detection) => {
                    const chosen = draft.detected && draft.directory === detection.Directory;
                    return (
                      <li key={`${detection.Directory}:${detection.Marker}`}>
                        <button
                          type="button"
                          aria-pressed={chosen}
                          aria-label={`Use the ${detection.Name} proposal`}
                          onClick={() => setDraft(draftFromDetection(detection))}
                          className={
                            "flex w-full flex-wrap items-center gap-2 rounded-lg border px-3 py-2 text-left text-xs " +
                            (chosen
                              ? "border-primary bg-primary/10 text-foreground"
                              : "border-border hover:bg-muted/50")
                          }
                        >
                          <span className="font-medium">{detection.Name}</span>
                          <span className="font-mono text-muted-foreground">{detection.Directory}</span>
                          <span className="text-muted-foreground">{detection.Marker}</span>
                          <span className="ml-auto flex gap-1.5">
                            {detection.Language && <Badge variant="secondary">{detection.Language}</Badge>}
                            {detection.Framework && <Badge variant="outline">{detection.Framework}</Badge>}
                          </span>
                        </button>
                      </li>
                    );
                  })}
                </ul>
              )}
            </div>

            <div className="grid gap-2 sm:grid-cols-2">
              <label className="space-y-1">
                <span className="text-xs font-medium text-muted-foreground">Name</span>
                <input
                  aria-label="Application name"
                  className="h-8 w-full rounded-lg border border-border bg-background px-2 text-sm outline-none focus-visible:border-ring"
                  value={draft.name}
                  onChange={(event) => update("name", event.target.value)}
                />
              </label>
              <label className="space-y-1">
                <span className="text-xs font-medium text-muted-foreground">
                  Folder inside the project
                </span>
                <input
                  aria-label="Application folder"
                  className="h-8 w-full rounded-lg border border-border bg-background px-2 font-mono text-sm outline-none focus-visible:border-ring"
                  placeholder=". or apps/web"
                  value={draft.directory}
                  onChange={(event) =>
                    // Typing a folder by hand makes this a manual record: the
                    // marker described the folder detection proposed, not this one.
                    setDraft((current) => ({
                      ...current,
                      directory: event.target.value,
                      detected: current.detected && event.target.value === current.directory,
                    }))
                  }
                />
              </label>
              <label className="space-y-1">
                <span className="text-xs font-medium text-muted-foreground">Language</span>
                <input
                  aria-label="Language"
                  className="h-8 w-full rounded-lg border border-border bg-background px-2 text-sm outline-none focus-visible:border-ring"
                  placeholder="TypeScript"
                  value={draft.language}
                  onChange={(event) => update("language", event.target.value)}
                />
              </label>
              <label className="space-y-1">
                <span className="text-xs font-medium text-muted-foreground">Framework</span>
                <input
                  aria-label="Framework"
                  className="h-8 w-full rounded-lg border border-border bg-background px-2 text-sm outline-none focus-visible:border-ring"
                  placeholder="Next.js"
                  value={draft.framework}
                  onChange={(event) => update("framework", event.target.value)}
                />
              </label>
              <label className="space-y-1 sm:col-span-2">
                <span className="text-xs font-medium text-muted-foreground">Start command</span>
                <input
                  aria-label="Start command"
                  className="h-8 w-full rounded-lg border border-border bg-background px-2 font-mono text-sm outline-none focus-visible:border-ring"
                  placeholder="npm run dev"
                  value={draft.startCommand}
                  onChange={(event) => update("startCommand", event.target.value)}
                />
              </label>
              <label className="space-y-1 sm:col-span-2">
                <span className="text-xs font-medium text-muted-foreground">Stop command</span>
                <input
                  aria-label="Stop command"
                  className="h-8 w-full rounded-lg border border-border bg-background px-2 font-mono text-sm outline-none focus-visible:border-ring"
                  placeholder="Optional"
                  value={draft.stopCommand}
                  onChange={(event) => update("stopCommand", event.target.value)}
                />
              </label>
            </div>

            {create.error && (
              <p role="alert" className="text-sm text-destructive">
                Could not add the application: {String(create.error)}
              </p>
            )}
          </div>

          <DialogFooter className="mt-5">
            <Button type="button" variant="outline" onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" disabled={!canSubmit}>
              {create.isPending ? "Adding…" : "Add"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function DependenciesDialog({
  app,
  applications,
  onClose,
  onSaved,
}: {
  app: Application;
  applications: Application[];
  onClose: () => void;
  onSaved: () => void;
}) {
  const [selected, setSelected] = useState<ReadonlySet<string>>(
    () => new Set(app.DependsOn ?? []),
  );

  const others = applications.filter((candidate) => candidate.ID !== app.ID);

  const save = useMutation({
    // The service validates the graph before writing; a cycle comes back as an
    // error and leaves the stored edges alone.
    mutationFn: () => ApplicationsService.SetDependencies(app.ID, [...selected]),
    onSuccess: onSaved,
  });

  const toggle = (id: string) =>
    setSelected((current) => {
      const next = new Set(current);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });

  return (
    <Dialog open onOpenChange={(next) => !next && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>What {app.Name} needs first</DialogTitle>
          <DialogDescription>
            The selected applications are started before {app.Name}. Locafy refuses a choice that
            would make them wait on each other.
          </DialogDescription>
        </DialogHeader>

        <div className="mt-4 space-y-2">
          {others.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              This is the project's only application, so it has nothing to depend on.
            </p>
          ) : (
            others.map((candidate) => (
              <label key={candidate.ID} className="flex items-start gap-2 text-sm">
                <input
                  type="checkbox"
                  className="mt-1 size-4 accent-primary"
                  aria-label={`Depend on ${candidate.Name}`}
                  checked={selected.has(candidate.ID)}
                  disabled={save.isPending}
                  onChange={() => toggle(candidate.ID)}
                />
                <span className="min-w-0">
                  <span className="block">{candidate.Name}</span>
                  <span className="block truncate font-mono text-xs text-muted-foreground">
                    {candidate.Directory}
                  </span>
                </span>
              </label>
            ))
          )}

          {save.error && (
            <p role="alert" className="text-sm text-destructive">
              Could not save the dependencies: {String(save.error)}
            </p>
          )}
        </div>

        <DialogFooter className="mt-5">
          <Button type="button" variant="outline" onClick={onClose}>
            Cancel
          </Button>
          <Button type="button" disabled={save.isPending} onClick={() => save.mutate()}>
            {save.isPending ? "Saving…" : "Save dependencies"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
