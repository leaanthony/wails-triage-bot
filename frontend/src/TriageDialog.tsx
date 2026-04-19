import { useEffect, useState } from "react";
import { ExternalLinkIcon, Loader2Icon } from "lucide-react";

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
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
  verdict: string;
};

type TriageResult = {
  tier: string;
  is_duplicate: boolean;
  confidence: number;
  reasoning: string;
  candidates: Candidate[];
  target?: {
    number: number;
    title: string;
    url: string;
    state: string;
    labels: string[];
  };
};

type Props = {
  number: number | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
};

export function TriageDialog({ number, open, onOpenChange }: Props) {
  const [issue, setIssue] = useState<Issue | null>(null);
  const [triage, setTriage] = useState<TriageResult | null>(null);
  const [loadingIssue, setLoadingIssue] = useState(false);
  const [loadingTriage, setLoadingTriage] = useState(false);
  const [error, setError] = useState<string>("");

  useEffect(() => {
    if (!open || number == null) return;
    setIssue(null);
    setTriage(null);
    setError("");
    setLoadingIssue(true);
    setLoadingTriage(true);

    fetch(`/api/issue?number=${number}`)
      .then((r) => (r.ok ? r.json() : Promise.reject(new Error("lookup failed"))))
      .then(setIssue)
      .catch((e) => setError(String(e)))
      .finally(() => setLoadingIssue(false));

    fetch(`/api/triage?number=${number}`)
      .then((r) => (r.ok ? r.json() : r.text().then((t) => Promise.reject(new Error(t)))))
      .then(setTriage)
      .catch((e) => setError((prev) => prev || String(e)))
      .finally(() => setLoadingTriage(false));
  }, [open, number]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[85vh] overflow-y-auto sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            Triage issue
            {number != null && (
              <span className="text-muted-foreground">#{number}</span>
            )}
          </DialogTitle>
          <DialogDescription>
            KNN top-5 plus LLM reasoning over the Wails backlog.
          </DialogDescription>
        </DialogHeader>

        {error && <div className="text-destructive text-sm">{error}</div>}

        {/* Issue details */}
        <section className="space-y-2">
          <h3 className="text-muted-foreground text-xs uppercase tracking-wide">Issue</h3>
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
                <p className="mt-3 max-h-40 overflow-auto whitespace-pre-wrap text-muted-foreground text-sm">
                  {issue.body}
                </p>
              )}
            </div>
          ) : null}
        </section>

        {/* Triage */}
        <section className="space-y-2">
          <h3 className="text-muted-foreground text-xs uppercase tracking-wide">
            Duplicate check
          </h3>
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
                <div className="space-y-1 text-sm">
                  <div className="text-muted-foreground text-xs uppercase tracking-wide">
                    Candidates
                  </div>
                  <ul className="space-y-1">
                    {triage.candidates.map((c) => (
                      <li key={c.number} className="flex items-center gap-2">
                        <span className="font-mono">#{c.number}</span>
                        <Badge variant="outline" className="font-normal">
                          {c.verdict}
                        </Badge>
                      </li>
                    ))}
                  </ul>
                </div>
              )}
            </div>
          ) : null}
        </section>

        <div className="flex justify-end">
          <Button variant="ghost" onClick={() => onOpenChange(false)}>
            Close
          </Button>
        </div>
      </DialogContent>
    </Dialog>
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
  const label = tier
    .replace(/_/g, " ")
    .replace(/\b\w/g, (c) => c.toUpperCase());
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
