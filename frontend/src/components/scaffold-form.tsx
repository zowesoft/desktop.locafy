import { useEffect, useMemo, useRef, useState } from "react";
import { Events } from "@wailsio/runtime";

import * as ScaffoldService from "../../bindings/locafy.dev/desktop/internal/scaffold/service";
import type {
  Job,
  Preflight,
  Recipe,
} from "../../bindings/locafy.dev/desktop/internal/scaffold/models";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

type Result = { job: Job };

type Props = {
  mode: "application" | "project";
  projectID?: string;
  /** Project mode keeps the name here; application mode names the folder. */
  onDone: (result: Result) => void;
  onCancel: () => void;
};

const optionsFromDefaults = (recipe: Recipe): Record<string, string> =>
  Object.fromEntries((recipe.Options ?? []).map((option) => [option.Key, option.Default]));

/*
 * Framework scaffolding: the catalogue is shipped with Locafy, every command
 * is shown before it runs, and the record exists only after the installer's
 * own files are on disk.
 */
export function ScaffoldForm({ mode, projectID, onDone, onCancel }: Props) {
  const [recipes, setRecipes] = useState<Recipe[]>([]);
  const [recipeID, setRecipeID] = useState("laravel");
  const [options, setOptions] = useState<Record<string, string>>({});
  const [tools, setTools] = useState<Record<string, string>>({});
  const [directory, setDirectory] = useState("laravel");
  const [projectName, setProjectName] = useState("");
  const [preflight, setPreflight] = useState<Preflight | null>(null);
  const [job, setJob] = useState<Job | null>(null);
  const [lines, setLines] = useState<string[]>([]);
  const [failure, setFailure] = useState("");
  const [busy, setBusy] = useState(false);
  const [installing, setInstalling] = useState(false);
  const finished = useRef(false);

  const recipe = useMemo(
    () => recipes.find((candidate) => candidate.ID === recipeID),
    [recipes, recipeID],
  );

  useEffect(() => {
    let active = true;
    void ScaffoldService.Recipes()
      .then((result) => {
        if (!active) return;
        const loaded = result ?? [];
        setRecipes(loaded);
        const first = loaded.find((candidate) => candidate.ID === "laravel") ?? loaded[0];
        if (first) {
          setRecipeID(first.ID);
          setOptions(optionsFromDefaults(first));
          if (mode === "application") setDirectory(first.ID);
        }
      })
      .catch((error) => active && setFailure(`Could not read the framework list: ${String(error)}`));
    return () => {
      active = false;
    };
  }, [mode]);

  useEffect(() => {
    if (!recipe) return;
    let active = true;
    const timer = window.setTimeout(() => {
      void ScaffoldService.Preflight(recipe.ID, options)
        .then((result) => active && setPreflight(result))
        .catch((error) => active && setFailure(`Could not check the tools: ${String(error)}`));
    }, 150);
    return () => {
      active = false;
      window.clearTimeout(timer);
    };
  }, [recipe, options]);

  useEffect(() => {
    const offOutput = Events.On("scaffold:output", (event) => {
      const data = event.data as { jobId?: string; line?: string };
      if (!job || data.jobId !== job.ID || typeof data.line !== "string") return;
      setLines((current) => [...current.slice(-400), data.line as string]);
    });
    const offState = Events.On("scaffold:state", (event) => {
      const data = event.data as { jobId?: string; status?: string; error?: string };
      if (!job || data.jobId !== job.ID || !data.status) return;
      setJob((current) => (current ? { ...current, Status: data.status!, Error: data.error ?? "" } : current));
      if (data.status === "succeeded" && !finished.current) {
        finished.current = true;
        setBusy(false);
        onDone({ job: { ...job, Status: "succeeded" } });
      }
      if (data.status === "failed" || data.status === "cancelled") {
        setBusy(false);
        setFailure(data.error || (data.status === "cancelled" ? "Cancelled." : "The installer failed."));
      }
    });
    return () => {
      offOutput();
      offState();
    };
  }, [job, onDone]);

  const selectRecipe = (next: Recipe) => {
    setRecipeID(next.ID);
    setOptions(optionsFromDefaults(next));
    setPreflight(null);
    setFailure("");
    if (mode === "application") setDirectory(next.ID);
  };

  const canStart =
    Boolean(recipe) &&
    Boolean(preflight?.Ready) &&
    !busy &&
    (mode === "application" ? directory.trim() !== "" : projectName.trim() !== "");

  const start = async () => {
    if (!recipe || !canStart) return;
    finished.current = false;
    setFailure("");
    setLines([]);
    setBusy(true);
    try {
      const started =
        mode === "project"
          ? await ScaffoldService.StartProject(projectName.trim(), recipe.ID, options, tools)
          : await ScaffoldService.Start(projectID ?? "", recipe.ID, directory.trim(), options, tools);
      setJob(started);
    } catch (error) {
      setBusy(false);
      setFailure(String(error));
    }
  };

  const cancel = async () => {
    if (!job) return;
    try {
      await ScaffoldService.Cancel(job.ID);
    } catch (error) {
      setFailure(String(error));
    }
  };

  const discard = async () => {
    if (!job) return;
    try {
      await ScaffoldService.Discard(job.ID);
      setFailure("The partial folder was removed.");
      setJob(null);
      setLines([]);
    } catch (error) {
      setFailure(String(error));
    }
  };

  const installInstaller = async () => {
    setInstalling(true);
    setFailure("");
    try {
      await ScaffoldService.InstallLaravelInstaller();
      if (recipe) setPreflight(await ScaffoldService.Preflight(recipe.ID, options));
    } catch (error) {
      setFailure(String(error));
    } finally {
      setInstalling(false);
    }
  };

  return (
    <div className="space-y-4 text-sm">
      {mode === "project" && (
        <label className="block space-y-1.5">
          <span className="text-xs font-medium text-muted-foreground">Project name</span>
          <Input
            aria-label="Project name"
            autoFocus
            placeholder="Etugora"
            value={projectName}
            disabled={busy}
            onChange={(event) => setProjectName(event.target.value)}
          />
        </label>
      )}

      <div className="space-y-1.5">
        <span className="text-xs font-medium text-muted-foreground">Framework</span>
        <div className="grid grid-cols-2 gap-2 sm:grid-cols-3">
          {recipes.map((candidate) => (
            <button
              key={candidate.ID}
              type="button"
              aria-pressed={candidate.ID === recipeID}
              disabled={busy}
              onClick={() => selectRecipe(candidate)}
              className={
                "rounded-lg border px-3 py-2 text-left text-xs transition-colors " +
                (candidate.ID === recipeID
                  ? "border-primary bg-primary/10"
                  : "border-border hover:bg-muted/50")
              }
            >
              <span className="block text-sm font-medium">{candidate.Label}</span>
              <span className="text-muted-foreground">{candidate.Language}</span>
            </button>
          ))}
        </div>
      </div>

      {recipe && (
        <>
          <p className="text-xs text-muted-foreground">{recipe.Description}</p>

          <div className="grid gap-2 sm:grid-cols-2">
            {(recipe.Options ?? []).map((option) => (
              <label key={option.Key} className="space-y-1">
                <span className="text-xs font-medium text-muted-foreground">{option.Label}</span>
                <select
                  aria-label={option.Label}
                  className="h-8 w-full rounded-lg border border-border bg-background px-2 text-sm"
                  value={options[option.Key] ?? option.Default}
                  disabled={busy}
                  onChange={(event) =>
                    setOptions((current) => ({ ...current, [option.Key]: event.target.value }))
                  }
                >
                  {(option.Values ?? []).map((value) => (
                    <option key={value.Value} value={value.Value}>
                      {value.Label}
                    </option>
                  ))}
                </select>
              </label>
            ))}

            {mode === "application" && (
              <label className="space-y-1">
                <span className="text-xs font-medium text-muted-foreground">
                  Folder inside the project
                </span>
                <Input
                  aria-label="Application folder"
                  className="font-mono text-xs"
                  value={directory}
                  disabled={busy}
                  onChange={(event) => setDirectory(event.target.value)}
                />
              </label>
            )}
          </div>

          <div className="space-y-2 rounded-lg border border-border p-3">
            <div className="flex flex-wrap items-center gap-2">
              <span className="text-xs font-medium">Tools</span>
              <Badge variant={preflight?.Ready ? "secondary" : "outline"}>
                {preflight ? (preflight.Ready ? "Ready" : "Needs attention") : "Checking…"}
              </Badge>
              {recipe.ID === "laravel" && preflight && !preflight.Installer && (
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  className="ml-auto"
                  disabled={installing || busy}
                  onClick={installInstaller}
                >
                  {installing ? "Installing…" : "Install Laravel installer"}
                </Button>
              )}
            </div>

            {(preflight?.Tools ?? []).map((status) => (
              <div key={status.Tool} className="flex flex-wrap items-center gap-2 text-xs">
                <span className={status.Required && !status.OK ? "text-destructive" : ""}>
                  {status.Label}
                  {status.Required ? "" : " (optional)"}
                </span>
                {(status.Candidates ?? []).length > 1 ? (
                  <select
                    aria-label={`${status.Label} version`}
                    className="h-7 min-w-0 flex-1 rounded-md border border-border bg-background px-1 text-xs"
                    value={tools[status.Tool] ?? status.Selected}
                    disabled={busy}
                    onChange={(event) =>
                      setTools((current) => ({ ...current, [status.Tool]: event.target.value }))
                    }
                  >
                    {(status.Candidates ?? []).map((candidate) => (
                      <option key={candidate.Path} value={candidate.Path}>
                        {candidate.Version || "unknown version"} — {candidate.Path}
                      </option>
                    ))}
                  </select>
                ) : (
                  <span className="truncate font-mono text-muted-foreground">
                    {status.Version ? `${status.Version} — ${status.Selected}` : "not found"}
                  </span>
                )}
                {status.Required && !status.OK && status.Problem && (
                  <span className="text-destructive">{status.Problem}</span>
                )}
              </div>
            ))}

            {(preflight?.Commands ?? []).length > 0 && (
              <div className="space-y-1">
                <span className="text-xs text-muted-foreground">This runs, in order:</span>
                <pre
                  aria-label="Commands to run"
                  className="max-h-24 overflow-auto whitespace-pre-wrap break-all rounded-md bg-muted/50 p-2 font-mono text-[11px]"
                >
                  {(preflight?.Commands ?? []).join("\n")}
                </pre>
              </div>
            )}

            {(preflight?.Notes ?? []).map((note) => (
              <p key={note} className="text-xs text-muted-foreground">
                {note}
              </p>
            ))}
          </div>

          {failure && (
            <p role="alert" className="text-sm text-destructive">
              {failure}
            </p>
          )}

          {lines.length > 0 && (
            <pre
              aria-label="Installer output"
              className="max-h-40 overflow-auto whitespace-pre-wrap break-all rounded-lg bg-black/40 p-2 font-mono text-[11px] text-foreground"
            >
              {lines.slice(-80).join("\n")}
            </pre>
          )}

          <div className="flex flex-wrap justify-end gap-2">
            <Button type="button" variant="outline" disabled={busy} onClick={onCancel}>
              Cancel
            </Button>
            {job && job.Status === "running" && (
              <Button type="button" variant="outline" onClick={cancel}>
                Stop
              </Button>
            )}
            {job && (job.Status === "failed" || job.Status === "cancelled") && job.CreatedTarget && (
              <Button type="button" variant="outline" onClick={discard}>
                Remove partial folder
              </Button>
            )}
            <Button type="button" disabled={!canStart} onClick={start}>
              {busy ? "Creating…" : mode === "project" ? "Create project" : "Create application"}
            </Button>
          </div>

          <p className="text-[11px] text-muted-foreground">
            {recipe.VerifiedAt} · <a className="underline" href={recipe.Docs} target="_blank" rel="noreferrer">Official installation guide</a>
          </p>
        </>
      )}
    </div>
  );
}
