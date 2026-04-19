import { useCallback, useEffect, useRef, useState } from "react";
import {
  FileTextIcon,
  DownloadIcon,
  WrenchIcon,
  CheckCircle2Icon,
  CircleDashedIcon,
  XCircleIcon,
} from "lucide-react";

import {
  Conversation,
  ConversationContent,
  ConversationEmptyState,
  ConversationScrollButton,
} from "@/components/ai-elements/conversation";
import {
  Message,
  MessageContent,
  MessageResponse,
} from "@/components/ai-elements/message";
import {
  PromptInput,
  PromptInputBody,
  PromptInputFooter,
  PromptInputSubmit,
  PromptInputTextarea,
  type PromptInputMessage,
} from "@/components/ai-elements/prompt-input";
import {
  ChainOfThought,
  ChainOfThoughtContent,
  ChainOfThoughtHeader,
  ChainOfThoughtStep,
} from "@/components/ai-elements/chain-of-thought";
import {
  Suggestion,
  Suggestions,
} from "@/components/ai-elements/suggestion";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";
import { Button } from "@/components/ui/button";
import { IssuePanel, type IssuePanelRequest } from "@/IssuePanel";

type ToolEvent = {
  callId: string;
  name: string;
  args: string;
  status: "running" | "ok" | "err";
  msg?: string;
};

type QuickAction = { label: string; prompt: string };

type ChatMessage = {
  id: string;
  role: "user" | "assistant";
  text: string;
  tools: ToolEvent[];
  actions: QuickAction[];
  streaming: boolean;
};

type Frame = {
  type:
    | "token"
    | "tool_call"
    | "tool_result"
    | "done"
    | "error"
    | "user"
    | "log"
    | "quick_actions";
  data?: string;
  name?: string;
  args?: string;
  ok?: boolean;
  msg?: string;
  call_id?: string;
  actions?: QuickAction[];
};

let idSeq = 0;
const nextId = () => `m-${++idSeq}`;

const STARTERS = [
  "Find likely duplicates of issue #5161",
  "Search for WebSocket bugs",
];

const MAX_LOGS = 5000;

export default function App() {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [busy, setBusy] = useState(false);
  const [connection, setConnection] = useState<"connecting" | "open" | "closed">("connecting");
  const [repo, setRepo] = useState<string>("");
  const [logs, setLogs] = useState<string[]>([]);
  const [logsOpen, setLogsOpen] = useState(false);
  const [issuePanel, setIssuePanel] = useState<IssuePanelRequest | null>(null);
  const [issuePanelOpen, setIssuePanelOpen] = useState(false);
  // Active quick-action pills: last non-empty set survives until the next
  // interaction replaces them. Not wiped when a turn returns no actions.
  const [activeActions, setActiveActions] = useState<QuickAction[]>([]);

  const openIssuePanel = useCallback((n: number, autoTriage: boolean) => {
    setIssuePanel({ number: n, autoTriage });
    setIssuePanelOpen(true);
  }, []);
  const wsRef = useRef<WebSocket | null>(null);
  const logsViewRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    fetch("/api/meta")
      .then((r) => r.json())
      .then((d) => setRepo(d.repo ?? ""))
      .catch(() => {});
  }, []);

  useEffect(() => {
    const proto = location.protocol === "https:" ? "wss" : "ws";
    const ws = new WebSocket(`${proto}://${location.host}/ws`);
    ws.onopen = () => setConnection("open");
    ws.onclose = () => setConnection("closed");
    ws.onerror = () => setConnection("closed");
    ws.onmessage = (ev) => {
      const frame: Frame = JSON.parse(ev.data);
      if (frame.type === "log") {
        setLogs((prev) => {
          const next = prev.length >= MAX_LOGS ? prev.slice(-MAX_LOGS + 1) : prev;
          return [...next, frame.data ?? ""];
        });
        return;
      }
      if (frame.type === "quick_actions" && frame.actions && frame.actions.length > 0) {
        setActiveActions(frame.actions);
      }
      setMessages((prev) => applyFrame(prev, frame));
      if (frame.type === "done" || frame.type === "error") setBusy(false);
    };
    wsRef.current = ws;
    return () => ws.close();
  }, []);

  useEffect(() => {
    if (!logsOpen) return;
    const el = logsViewRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [logs, logsOpen]);

  const sendPrompt = useCallback(
    (text: string) => {
      const trimmed = text.trim();
      if (!trimmed || busy || !wsRef.current) return;
      const userId = nextId();
      const asstId = nextId();
      setMessages((prev) => [
        ...prev,
        { id: userId, role: "user", text: trimmed, tools: [], actions: [], streaming: false },
        { id: asstId, role: "assistant", text: "", tools: [], actions: [], streaming: true },
      ]);
      wsRef.current.send(JSON.stringify({ type: "user", data: trimmed }));
      setBusy(true);
    },
    [busy]
  );

  const handleSubmit = useCallback(
    (msg: PromptInputMessage) => sendPrompt(msg.text ?? ""),
    [sendPrompt]
  );

  const downloadLogs = useCallback(() => {
    const blob = new Blob([logs.join("\n") + "\n"], { type: "text/plain" });
    const url = URL.createObjectURL(blob);
    const ts = new Date().toISOString().replace(/[:.]/g, "-");
    const a = document.createElement("a");
    a.href = url;
    a.download = `taliesin-session-${ts}.log`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  }, [logs]);

  // Suggestion pills above the input come from the last non-empty
  // quick_actions frame. They persist across follow-up turns so the user can
  // still click them after asking an unrelated question.
  const latestActions = activeActions;

  return (
    <div className="flex h-dvh flex-col bg-background text-foreground">
      <header className="flex items-center justify-between border-b px-6 py-4">
        <div>
          <h1 className="font-semibold text-lg">Taliesin</h1>
          <p className="text-muted-foreground text-xs">
            Your friendly Wails triage bot
          </p>
        </div>
        <div className="flex items-center gap-4 text-xs">
          <Sheet open={logsOpen} onOpenChange={setLogsOpen}>
            <SheetTrigger asChild>
              <Button
                variant="ghost"
                size="sm"
                className="cursor-pointer gap-1.5 text-muted-foreground hover:text-foreground"
              >
                <FileTextIcon className="size-3.5" />
                Logs
                {logs.length > 0 && (
                  <span className="rounded-full bg-muted px-1.5 text-[10px]">{logs.length}</span>
                )}
              </Button>
            </SheetTrigger>
            <SheetContent side="right" className="w-full gap-0 p-0 sm:max-w-xl">
              <SheetHeader className="border-b">
                <SheetTitle>Server logs</SheetTitle>
                <SheetDescription>Live stream from the Go server over WebSocket.</SheetDescription>
                <div className="flex items-center gap-2 pt-2">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={downloadLogs}
                    disabled={logs.length === 0}
                    className="cursor-pointer gap-1.5"
                  >
                    <DownloadIcon className="size-3.5" />
                    Download
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => setLogs([])}
                    disabled={logs.length === 0}
                    className="cursor-pointer"
                  >
                    Clear
                  </Button>
                  <span className="ml-auto text-muted-foreground text-xs">
                    {logs.length} line{logs.length === 1 ? "" : "s"}
                  </span>
                </div>
              </SheetHeader>
              <div
                ref={logsViewRef}
                className="h-[calc(100dvh-10rem)] overflow-auto bg-muted/30 p-3 font-mono text-[11px] leading-5"
              >
                {logs.length === 0 ? (
                  <span className="text-muted-foreground">No logs yet. Server lines appear here as they happen.</span>
                ) : (
                  logs.map((line, i) => (
                    <div key={i} className="whitespace-pre-wrap break-all">
                      {line}
                    </div>
                  ))
                )}
              </div>
            </SheetContent>
          </Sheet>

          <span
            className={`size-2 rounded-full ${
              connection === "open"
                ? "bg-emerald-500"
                : connection === "closed"
                  ? "bg-red-500"
                  : "bg-muted-foreground"
            }`}
            aria-label={connection}
            title={connection}
          />
          {repo ? (
            <a
              href={`https://github.com/${repo}`}
              target="_blank"
              rel="noreferrer"
              className="cursor-pointer text-muted-foreground transition-colors hover:text-primary hover:underline"
            >
              {repo}
            </a>
          ) : (
            <span className="text-muted-foreground">{connection}</span>
          )}
        </div>
      </header>

      <Conversation className="flex-1">
        <ConversationContent className="mx-auto max-w-3xl">
          {messages.length === 0 && (
            <div className="mt-8 flex flex-col items-center gap-6">
              <ConversationEmptyState
                title="Hi, I'm Taliesin 👋"
                description="Your friendly Wails triage bot. Pick a starter or ask your own question."
              />
              <div className="flex w-full justify-center">
                <Suggestions className="mx-auto">
                  {STARTERS.map((s) => (
                    <Suggestion
                      key={s}
                      suggestion={s}
                      onClick={sendPrompt}
                      disabled={busy}
                    />
                  ))}
                </Suggestions>
              </div>
            </div>
          )}
          {messages.map((m) =>
            m.role === "user" ? (
              <Message from="user" key={m.id}>
                <MessageContent>
                  <span className="whitespace-pre-wrap">{m.text}</span>
                </MessageContent>
              </Message>
            ) : (
              <Message from="assistant" key={m.id}>
                <MessageContent className="w-full">
                  {m.tools.length > 0 && <ChainBlock message={m} />}
                  {m.text ? (
                    <MessageResponse
                      linkSafety={{ enabled: false }}
                      components={{
                        a: ({ href, children, ...rest }) => {
                          const triageMatch =
                            href?.match(/^\/triage\/(\d+)/) ??
                            (href?.startsWith("triage:")
                              ? [href, href.slice("triage:".length)]
                              : null);
                          if (triageMatch) {
                            const n = parseInt(triageMatch[1], 10);
                            if (!Number.isNaN(n)) {
                              return (
                                <button
                                  type="button"
                                  onClick={() => openIssuePanel(n, true)}
                                  className="inline-flex cursor-pointer items-center gap-1 rounded-md border border-primary/60 bg-primary/10 px-2 py-0.5 font-medium text-primary text-xs transition-colors hover:bg-primary hover:text-primary-foreground"
                                >
                                  <WrenchIcon className="size-3" />
                                  Triage
                                </button>
                              );
                            }
                          }
                          const issueMatch = href?.match(
                            /^https?:\/\/github\.com\/[^/]+\/[^/]+\/issues\/(\d+)(?:[#?].*)?$/
                          );
                          if (issueMatch) {
                            const n = parseInt(issueMatch[1], 10);
                            return (
                              <a
                                {...rest}
                                href={href}
                                onClick={(e) => {
                                  if (e.metaKey || e.ctrlKey || e.shiftKey) return;
                                  e.preventDefault();
                                  openIssuePanel(n, false);
                                }}
                                className="cursor-pointer text-primary underline-offset-4 transition-colors hover:text-accent hover:underline"
                              >
                                {children}
                              </a>
                            );
                          }
                          return (
                            <a
                              {...rest}
                              href={href}
                              target="_blank"
                              rel="noreferrer"
                              className="cursor-pointer text-primary underline-offset-4 transition-colors hover:text-accent hover:underline"
                            >
                              {children}
                            </a>
                          );
                        },
                      }}
                    >
                      {linkifyIssueRefs(m.text, repo)}
                    </MessageResponse>
                  ) : m.streaming && m.tools.length === 0 ? (
                    <span className="text-muted-foreground text-sm">…</span>
                  ) : null}
                </MessageContent>
              </Message>
            )
          )}
        </ConversationContent>
        <ConversationScrollButton />
      </Conversation>

      <IssuePanel
        request={issuePanel}
        open={issuePanelOpen}
        onOpenChange={(v) => {
          setIssuePanelOpen(v);
          if (!v) setIssuePanel(null);
        }}
        onAsk={sendPrompt}
        onOpenIssue={openIssuePanel}
      />

      <div className="bg-background px-4 pb-3 pt-1">
        <div className="mx-auto max-w-3xl space-y-2">
          {latestActions.length > 0 && (
            <div className="flex flex-wrap gap-2">
              {latestActions.map((a) => (
                <Suggestion
                  key={a.label}
                  suggestion={a.prompt}
                  onClick={() => sendPrompt(a.prompt)}
                  disabled={busy}
                >
                  {a.label}
                </Suggestion>
              ))}
            </div>
          )}
          <PromptInput onSubmit={handleSubmit} className="bg-card">
            <PromptInputBody>
              <PromptInputTextarea
                placeholder={busy ? "…Taliesin is thinking…" : "Ask Taliesin about issues…"}
                disabled={busy}
                className="min-h-10 max-h-40"
              />
            </PromptInputBody>
            <PromptInputFooter className="justify-end">
              <PromptInputSubmit status={busy ? "streaming" : undefined} />
            </PromptInputFooter>
          </PromptInput>
        </div>
      </div>
    </div>
  );
}

function applyFrame(messages: ChatMessage[], frame: Frame): ChatMessage[] {
  if (messages.length === 0) return messages;
  const idx = messages.length - 1;
  const last = messages[idx];
  if (last.role !== "assistant") return messages;
  const next: ChatMessage = { ...last, tools: [...last.tools], actions: [...last.actions] };
  switch (frame.type) {
    case "token":
      next.text += frame.data ?? "";
      break;
    case "tool_call":
      next.tools.push({
        callId: frame.call_id ?? `t-${next.tools.length}`,
        name: frame.name ?? "tool",
        args: frame.args ?? "",
        status: "running",
      });
      break;
    case "tool_result": {
      const t = next.tools.find((x) => x.callId === frame.call_id);
      if (t) {
        t.status = frame.ok ? "ok" : "err";
        if (!frame.ok) t.msg = frame.msg;
      }
      break;
    }
    case "quick_actions":
      next.actions = frame.actions ?? [];
      break;
    case "done":
      next.streaming = false;
      break;
    case "error":
      next.text += (next.text ? "\n\n" : "") + `**error**: ${frame.msg ?? "unknown"}`;
      next.streaming = false;
      break;
    default:
      break;
  }
  const out = messages.slice(0, idx);
  out.push(next);
  return out;
}

function ChainBlock({ message }: { message: ChatMessage }) {
  const [open, setOpen] = useState(message.streaming);
  // Auto-roll-up once streaming completes; still user-toggleable afterward.
  const prevStreaming = useRef(message.streaming);
  useEffect(() => {
    if (prevStreaming.current && !message.streaming) {
      setOpen(false);
    }
    if (!prevStreaming.current && message.streaming) {
      setOpen(true);
    }
    prevStreaming.current = message.streaming;
  }, [message.streaming]);

  return (
    <ChainOfThought
      open={open}
      onOpenChange={setOpen}
      className="mb-3 rounded-md border border-border/40 bg-muted/30 px-4 py-3 text-muted-foreground"
    >
      <ChainOfThoughtHeader>
        {chainHeader(message.tools, message.streaming)}
      </ChainOfThoughtHeader>
      <ChainOfThoughtContent>
        {message.tools.map((t) => (
          <ChainOfThoughtStep
            key={t.callId}
            icon={iconFor(t.status)}
            label={<span className="font-medium">{t.name}</span>}
            description={stepDescription(t)}
            status={t.status === "running" ? "active" : "complete"}
          />
        ))}
      </ChainOfThoughtContent>
    </ChainOfThought>
  );
}

function chainHeader(tools: ToolEvent[], streaming: boolean): string {
  const running = tools.some((t) => t.status === "running");
  if (streaming && running) return `Working (${tools.length} step${tools.length === 1 ? "" : "s"})`;
  return `Chain of thought · ${tools.length} step${tools.length === 1 ? "" : "s"}`;
}

function iconFor(status: ToolEvent["status"]) {
  switch (status) {
    case "running":
      return CircleDashedIcon;
    case "ok":
      return CheckCircle2Icon;
    case "err":
      return XCircleIcon;
    default:
      return WrenchIcon;
  }
}

function stepDescription(t: ToolEvent) {
  if (t.status === "err") return t.msg || "error";
  if (!t.args) return null;
  return <span className="font-mono text-xs break-all">{truncate(t.args, 200)}</span>;
}

function truncate(s: string, n: number) {
  if (!s) return "";
  return s.length > n ? s.slice(0, n) + "…" : s;
}

// linkifyIssueRefs wraps bare `#1234` occurrences in markdown links to the
// repo's issue page, skipping fenced code blocks and text already inside a
// markdown link label or URL.
function linkifyIssueRefs(md: string, repo: string): string {
  if (!repo) return md;
  const parts = md.split(/(```[\s\S]*?```|`[^`\n]*`)/g);
  const pattern = /(?<![\w\/\[\(])#(\d{1,7})\b/g;
  return parts
    .map((part) => {
      if (part.startsWith("```") || part.startsWith("`")) return part;
      return part.replace(pattern, (_m, n) => `[#${n}](https://github.com/${repo}/issues/${n})`);
    })
    .join("");
}
