import { useCallback, useEffect, useState } from "react";
import { ExternalLinkIcon, Loader2Icon, WrenchIcon } from "lucide-react";

import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";

type Issue = {
  number: number;
  title: string;
  body: string;
  state: string;
  labels: string[];
  url: string;
  author?: string;
  created_at?: string;
};

type Candidate = {
  number: number;
  title?: string;
  url?: string;
  state?: string;
  verdict: string;
};

type TriageResult = {
  tier: string;
  is_duplicate: boolean;
  confidence: number;
  reasoning: string;
  candidates: Candidate[];
};

export type IssuePanelRequest = {
  number: number;
  autoTriage: boolean;
};

type Props = {
  request: IssuePanelRequest | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onAsk: (prompt: string) => void;
  onOpenIssue: (number: number, autoTriage: boolean) => void;
};

export function IssuePanel({ request, open, onOpenChange, onAsk, onOpenIssue }: Props) {
  const [issue, setIssue] = useState<Issue | null>(null);
  const [triage, setTriage] = useState<TriageResult | null>(null);
  const [loadingIssue, setLoadingIssue] = useState(false);
  const [loadingTriage, setLoadingTriage] = useState(false);
  const [error, setError] = useState<string>("");

  const runTriage = useCallback((n: number) => {
    setLoadingTriage(true);
    fetch(`/api/triage?number=${n}`)
      .then((r) => (r.ok ? r.json() : r.text().then((t) => Promise.reject(new Error(t)))))
      .then(setTriage)
      .catch((e) => setError((prev) => prev || String(e)))
      .finally(() => setLoadingTriage(false));
  }, []);

  useEffect(() => {
    if (!open || !request) return;
    setIssue(null);
    setTriage(null);
    setError("");
    setLoadingIssue(true);

    fetch(`/api/issue?number=${request.number}`)
      .then((r) => (r.ok ? r.json() : Promise.reject(new Error("lookup failed"))))
      .then(setIssue)
      .catch((e) => setError(String(e)))
      .finally(() => setLoadingIssue(false));

    if (request.autoTriage) runTriage(request.number);
  }, [open, request, runTriage]);

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="right" className="w-full gap-0 overflow-y-auto p-0 sm:max-w-xl">
        <SheetHeader className="border-b">
          <SheetTitle className="flex items-center gap-2">
            Issue
            {request?.number != null && (
              <span className="text-muted-foreground">#{request.number}</span>
            )}
          </SheetTitle>
          <SheetDescription>Inline view + optional duplicate check.</SheetDescription>
        </SheetHeader>

        <div className="space-y-4 p-4">
          {error && <div className="text-destructive text-sm">{error}</div>}

          <section className="space-y-2">
            {loadingIssue ? (
              <Spinner label="Loading issue…" />
            ) : issue ? (
              <div className="rounded-md border bg-muted/30 p-3">
                <div className="flex items-start justify-between gap-4">
                  <a
                    href={issue.url}
                    target="_blank"
                    rel="noreferrer"
                    className="cursor-pointer font-medium text-primary hover:underline"
                  >
                    {issue.title}
                    <ExternalLinkIcon className="mb-0.5 ml-1 inline size-3.5" />
                  </a>
                  <Badge variant={issue.state === "open" ? "default" : "secondary"}>
                    {issue.state}
                  </Badge>
                </div>
                <div className="mt-2 flex flex-wrap items-center gap-2 text-muted-foreground text-xs">
                  {issue.author && <span>@{issue.author}</span>}
                  {issue.created_at && <span>· {issue.created_at.slice(0, 10)}</span>}
                  {issue.labels?.map((l) => (
                    <Badge key={l} variant="outline" className="font-normal">
                      {l}
                    </Badge>
                  ))}
                </div>
                {issue.body && (
                  <p className="mt-3 max-h-56 overflow-auto whitespace-pre-wrap text-muted-foreground text-sm">
                    {issue.body}
                  </p>
                )}
              </div>
            ) : null}
          </section>

          <section className="space-y-2">
            <div className="flex items-center justify-between">
              <h3 className="text-muted-foreground text-xs uppercase tracking-wide">
                Duplicate check
              </h3>
              {!triage && !loadingTriage && issue && (
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => runTriage(issue.number)}
                  className="cursor-pointer gap-1.5"
                >
                  <WrenchIcon className="size-3.5" />
                  Run triage
                </Button>
              )}
            </div>
            {loadingTriage ? (
              <Spinner label="Running KNN + LLM reasoning…" />
            ) : triage ? (
              <div className="space-y-3 rounded-md border bg-muted/30 p-3">
                <div className="flex items-center gap-2">
                  <TierBadge tier={triage.tier} />
                  <span className="text-muted-foreground text-xs">
                    confidence: {(triage.confidence ?? 0).toFixed(2)}
                  </span>
                </div>
                {triage.reasoning && (
                  <p className="text-sm leading-6">{triage.reasoning}</p>
                )}
                {triage.candidates?.length > 0 && (
                  <div className="space-y-2">
                    <div className="text-muted-foreground text-xs uppercase tracking-wide">
                      Candidates
                    </div>
                    <ul className="space-y-1.5">
                      {triage.candidates.map((c) => (
                        <li key={c.number} className="flex flex-wrap items-center gap-2 text-sm">
                          <button
                            type="button"
                            className="cursor-pointer truncate text-left text-primary hover:underline"
                            onClick={() => onOpenIssue(c.number, false)}
                            title={c.title || `#${c.number}`}
                          >
                            <span className="font-mono text-muted-foreground">#{c.number}</span>
                            {c.title ? <span className="ml-2">{c.title}</span> : null}
                          </button>
                          <Badge variant="outline" className="ml-auto font-normal">
                            {c.verdict}
                          </Badge>
                          <button
                            type="button"
                            onClick={() => onOpenIssue(c.number, true)}
                            className="cursor-pointer text-muted-foreground text-xs hover:text-foreground"
                          >
                            triage →
                          </button>
                        </li>
                      ))}
                    </ul>
                  </div>
                )}
              </div>
            ) : null}
          </section>

          {triage && (
            <section className="space-y-2">
              <h3 className="text-muted-foreground text-xs uppercase tracking-wide">
                Quick actions
              </h3>
              <div className="flex flex-wrap gap-2">
                {triage.candidates.map((c) => (
                  <Button
                    key={c.number}
                    size="sm"
                    variant="secondary"
                    className="cursor-pointer"
                    onClick={() => {
                      onOpenChange(false);
                      onAsk(`Compare #${issue?.number} against #${c.number} in detail.`);
                    }}
                  >
                    Compare against #{c.number}
                  </Button>
                ))}
                {issue && (
                  <Button
                    size="sm"
                    variant="secondary"
                    className="cursor-pointer"
                    onClick={() => {
                      onOpenChange(false);
                      onAsk(`Summarise the current state of issue #${issue.number} and recommend next steps.`);
                    }}
                  >
                    Summarise #{issue.number}
                  </Button>
                )}
              </div>
            </section>
          )}
        </div>
      </SheetContent>
    </Sheet>
  );
}

function Spinner({ label }: { label: string }) {
  return (
    <div className="flex items-center gap-2 rounded-md border bg-muted/30 p-3 text-muted-foreground text-sm">
      <Loader2Icon className="size-4 animate-spin" />
      {label}
    </div>
  );
}

function TierBadge({ tier }: { tier: string }) {
  const label = tier.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());
  let variant: "default" | "secondary" | "destructive" | "outline" = "outline";
  let classes = "";
  switch (tier) {
    case "recommend_auto_close":
      variant = "destructive";
      break;
    case "human_review":
      variant = "default";
      classes = "bg-amber-500 text-black hover:bg-amber-500";
      break;
    case "not_duplicate":
      variant = "secondary";
      break;
  }
  return (
    <Badge variant={variant} className={classes}>
      {label}
    </Badge>
  );
}
