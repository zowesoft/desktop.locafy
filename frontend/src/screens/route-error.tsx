import { isRouteErrorResponse, useNavigate, useRouteError } from "react-router-dom";
import { CircleX, House, RotateCcw } from "lucide-react";
import { Button } from "@/components/ui/button";

/**
 * Shown when a route throws, instead of the router's development notice. The
 * shell is replaced because the failure may be in the shell itself.
 */
export function RouteError() {
  const error = useRouteError();
  const navigate = useNavigate();

  const status = isRouteErrorResponse(error) ? error.status : null;
  const title = isRouteErrorResponse(error)
    ? `${error.status} ${error.statusText || "Unexpected response"}`
    : "This screen could not be displayed";

  const message = isRouteErrorResponse(error)
    ? typeof error.data === "string" && error.data.length > 0
      ? error.data
      : "The route did not return a usable response."
    : error instanceof Error
      ? error.message
      : "An unexpected error stopped this screen from rendering.";

  const stack = error instanceof Error ? error.stack : null;

  return (
    <div className="flex h-screen w-full items-center justify-center bg-background px-6 text-foreground">
      <div className="w-full max-w-md space-y-4">
        <span className="flex size-10 items-center justify-center rounded-xl bg-destructive/15 text-destructive">
          <CircleX className="size-5" />
        </span>

        <div className="space-y-1.5">
          <h1 className="text-xl font-semibold tracking-tight">{title}</h1>
          <p className="text-sm text-muted-foreground">{message}</p>
          {status !== null && (
            <p className="text-xs text-muted-foreground">Response status {status}.</p>
          )}
        </div>

        <div className="flex flex-wrap gap-2">
          <Button onClick={() => window.location.reload()}>
            <RotateCcw />
            Reload
          </Button>
          <Button variant="outline" onClick={() => navigate("/")}>
            <House />
            Back to overview
          </Button>
        </div>

        {stack && (
          <details className="rounded-lg border border-border bg-card p-3 text-xs">
            <summary className="cursor-pointer text-muted-foreground">
              Technical details
            </summary>
            <pre className="mt-2 max-h-48 overflow-auto whitespace-pre-wrap break-words font-mono text-[0.7rem] leading-relaxed text-muted-foreground">
              {stack}
            </pre>
          </details>
        )}
      </div>
    </div>
  );
}
