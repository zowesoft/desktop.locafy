import { useLocation, useNavigate } from "react-router-dom";
import { House, Search } from "lucide-react";

import { Button } from "@/components/ui/button";

/** Shown for a hash route that matches no screen, inside the shell so the
 * navigation stays available. */
export function NotFound() {
  const location = useLocation();
  const navigate = useNavigate();

  const attempted = `#${location.pathname}${location.search}`;

  return (
    <div className="mx-auto flex max-w-md flex-col items-start gap-4 px-6 py-20">
      <span className="flex size-10 items-center justify-center rounded-xl bg-muted text-muted-foreground">
        <Search className="size-5" />
      </span>

      <div className="space-y-1.5">
        <h1 className="text-xl font-semibold tracking-tight">Page not found</h1>
        <p className="text-sm text-muted-foreground">
          Nothing is routed here. The address may be mistyped, or it may point at a screen that no
          longer exists.
        </p>
        <p className="text-xs text-muted-foreground">
          Requested: <code className="break-all font-mono text-foreground">{attempted}</code>
        </p>
      </div>

      <div className="flex flex-wrap gap-2">
        <Button onClick={() => navigate("/")}>
          <House />
          Back to overview
        </Button>
        <Button variant="outline" onClick={() => navigate(-1)}>
          Go back
        </Button>
      </div>
    </div>
  );
}
