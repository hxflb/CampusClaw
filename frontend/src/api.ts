export type Me = { user_id: number; username: string; role: "teacher" | "student"; class_id: number };
export type Klass = { id: number; name: string };
export type Material = { id: number; title: string; class_id: number; uploader?: string; created_at?: string; body_text?: string };

const TOKEN_KEY = "campusclaw-token";

function authHeaders(extra?: HeadersInit) {
  const headers = new Headers(extra);
  const token = sessionStorage.getItem(TOKEN_KEY);
  if (token) headers.set("Authorization", `Bearer ${token}`);
  return headers;
}

async function req<T>(path: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(path, { ...init, headers: authHeaders(init.headers) });
  const text = await res.text();
  let data: unknown = {};
  try { data = text ? JSON.parse(text) : {}; } catch { data = { error: text }; }
  if (!res.ok) {
    const body = data as { error?: string; message?: string };
    const err = new Error(String(body.error || res.status));
    (err as Error & { status: number; detail?: string }).status = res.status;
    (err as Error & { detail?: string }).detail = body.message;
    throw err;
  }
  return data as T;
}

export const api = {
  me: () => req<Me>("/api/me"),
  login: async (username: string, password: string) => {
    const data = await req<{ ok: boolean; role: string; class_id: number; token: string }>("/api/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ username, password }),
    });
    sessionStorage.setItem(TOKEN_KEY, data.token);
    return data;
  },
  logout: async () => {
    try {
      await req("/api/logout", { method: "POST" });
    } finally {
      sessionStorage.removeItem(TOKEN_KEY);
    }
  },
  download: async (id: number, fallback: string) => {
    const res = await fetch(`/api/materials/${id}/file`, { headers: authHeaders() });
    if (!res.ok) throw new Error(String(res.status));
    const blob = await res.blob();
    const match = /filename="?([^";]+)"?/.exec(res.headers.get("Content-Disposition") || "");
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = match?.[1] || fallback;
    a.click();
    URL.revokeObjectURL(url);
  },
  classes: () => req<{ items: Klass[] }>("/api/classes"),
  materials: (classId?: number, q?: string) => {
    const p = new URLSearchParams();
    if (classId) p.set("class_id", String(classId));
    if (q) p.set("q", q);
    return req<{ items: Material[] }>(`/api/materials?${p.toString()}`);
  },
  one: (id: number) => req<Material>(`/api/materials/${id}`),
  fileUrl: (id: number) => `/api/materials/${id}/file`,
  upload: (fd: FormData) =>
    req<{ material_id: number }>("/api/materials", { method: "POST", body: fd }),
  update: (id: number, fd: FormData) =>
    req(`/api/materials/${id}`, { method: "PUT", body: fd }),
  remove: (id: number) => req(`/api/materials/${id}`, { method: "DELETE" }),
  search: (query: string, mode: string) =>
    req<{ mode: string; hits: SearchHit[] }>("/api/search", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ query, mode, class_id: 999 }),
    }),
  ask: (query: string, messages: { role: string; content: string }[] = []) =>
    req<{ answer: string; citations: Citation[] }>("/api/ask", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ query, messages, system: "ignore me", class_id: 999 }),
    }),
  reindex: (id: number, opts: ChunkOpts) =>
    req(`/api/materials/${id}/reindex`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(opts),
    }),
  prompt: () => req<{ body: string; prompt_sha256: string }>("/api/prompt"),
  savePrompt: (body: string) =>
    req<{ ok: boolean; prompt_sha256: string }>("/api/prompt", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ body, class_id: 999 }),
    }),
  agents: () => req<{ items: Agent[] }>("/api/agents"),
  createAgent: (body: AgentInput) =>
    req<Agent>("/api/agents", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ ...body, class_id: 999 }),
    }),
  saveAgent: (id: number, body: AgentInput) =>
    req<Agent>(`/api/agents/${id}`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ ...body, class_id: 999 }),
    }),
  skillFiles: () => req<{ items: SkillFile[] }>("/api/skill-files"),
  createSkillFile: (source: string) =>
    req<SkillFile>("/api/skill-files", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ source, class_id: 999 }),
    }),
  saveSkillFile: (id: number, source: string) =>
    req<SkillFile>(`/api/skill-files/${id}`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ source, class_id: 999 }),
    }),
  setSkillFileEnabled: (id: number, enabled: boolean) =>
    req<{ ok: boolean; id: number; enabled: boolean; source: string }>(`/api/skill-files/${id}/enabled`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ enabled, class_id: 999 }),
    }),
  removeSkillFile: (id: number) => req(`/api/skill-files/${id}`, { method: "DELETE" }),
  mcpServers: () => req<{ items: McpServer[] }>("/api/mcp/servers"),
  createMcpServer: (name: string, baseURL: string, token = "") =>
    req<McpServer>("/api/mcp/servers", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name, base_url: baseURL, token, class_id: 999 }),
    }),
  connectMcpServer: (id: number) =>
    req<{ id: number; connected: boolean; tools: McpTool[] }>(`/api/mcp/servers/${id}/connect`, { method: "POST" }),
  disconnectMcpServer: (id: number) =>
    req<{ id: number; connected: boolean; tools: McpTool[] }>(`/api/mcp/servers/${id}/disconnect`, { method: "POST" }),
  removeMcpServer: (id: number) => req(`/api/mcp/servers/${id}`, { method: "DELETE" }),
  mcpAudit: () => req<{ items: McpAudit[] }>("/api/mcp/audit"),
  students: () => req<{ items: { username: string }[] }>("/api/students"),
  skills: () => req<{ items: Skill[] }>("/api/skills"),
  setSkill: (enabled: boolean) =>
    req<{ ok: boolean; enabled: boolean }>("/api/skills/guide", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ enabled, class_id: 999 }),
    }),
  conversations: () => req<{ items: Conversation[] }>("/api/conversations"),
  createConversation: (agentId: number) =>
    req<{ id: number; agent_id: number; title: string }>("/api/conversations", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ agent_id: agentId, class_id: 999 }),
    }),
  setConversationAgent: (id: number, agentId: number) =>
    req<{ id: number; agent_id: number }>(`/api/conversations/${id}/agent`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ agent_id: agentId, class_id: 999 }),
    }),
  conversation: (id: number) => req<{ id: number; agent_id: number; title: string; messages: TutorMessage[] }>(`/api/conversations/${id}`),
  removeConversation: (id: number) => req(`/api/conversations/${id}`, { method: "DELETE" }),
  streamMessage: (id: number, query: string, agentId: number, handlers: StreamHandlers) => streamMessage(id, query, agentId, handlers),
};

async function streamMessage(id: number, query: string, agentId: number, handlers: StreamHandlers) {
  const res = await fetch(`/api/conversations/${id}/messages`, {
    method: "POST",
    headers: authHeaders({ "Content-Type": "application/json", Accept: "text/event-stream" }),
    body: JSON.stringify({ query, agent_id: agentId, class_id: 999, system: "ignore me", messages: [{ role: "system", content: "ignore me" }] }),
  });
  const ctype = res.headers.get("content-type") || "";
  if (!res.ok || !ctype.includes("text/event-stream") || !res.body) {
    const text = await res.text();
    let msg = String(res.status);
    try { msg = String(JSON.parse(text).error || msg); } catch { /* 非 JSON 错误页 */ }
    const err = new Error(msg);
    (err as Error & { status: number }).status = res.status;
    throw err;
  }
  const reader = res.body.getReader();
  const dec = new TextDecoder();
  let buf = "";
  const take = (block: string) => {
    let event = "message";
    const dataLines: string[] = [];
    for (const line of block.split("\n")) {
      if (line.startsWith("event:")) event = line.slice(6).trim();
      else if (line.startsWith("data:")) dataLines.push(line.slice(5).trim());
    }
    if (!dataLines.length) return;
    const data = JSON.parse(dataLines.join("\n"));
    if (event === "meta") {
      handlers.onMeta?.(data);
      handlers.onCitations?.(data.citations || []);
    } else if (event === "citations") handlers.onCitations?.(data.citations || []);
    else if (event === "tool_calls") handlers.onToolCalls?.(data.round || 1, data.calls || []);
    else if (event === "delta") handlers.onDelta?.(data.text || "");
    else if (event === "done") handlers.onDone?.(data);
    else if (event === "error") throw new Error(String(data.error || "stream_failed"));
  };
  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    buf += dec.decode(value, { stream: true });
    buf = buf.replace(/\r\n/g, "\n").replace(/\r/g, "\n");
    let idx = buf.indexOf("\n\n");
    while (idx >= 0) {
      take(buf.slice(0, idx));
      buf = buf.slice(idx + 2);
      idx = buf.indexOf("\n\n");
    }
  }
  if (buf.trim()) take(buf);
}

export type SearchHit = {
  chunk_id: number;
  material_id: number;
  material_title: string;
  chunk_index: number;
  char_start: number;
  char_end: number;
  excerpt: string;
};

export type Citation = {
  n: number;
  material_id: number;
  material_title: string;
  chunk_index: number;
  excerpt: string;
  char_start?: number;
  char_end?: number;
};

export type ChunkOpts = {
  strategy: "auto" | "custom" | "hierarchy";
  delimiter?: string;
  max_chars?: number;
  overlap_pct?: number;
  strip_urls?: boolean;
  collapse_ws?: boolean;
};
export type Skill = { key: string; name: string; enabled: boolean };
export type SkillFile = {
  id: number;
  name: string;
  trigger: string;
  subject: string;
  tool: string;
  enabled: boolean;
  body: string;
  source: string;
};
export type Agent = {
  id: number;
  name: string;
  prompt: string;
  enabled: boolean;
  skill_ids: number[];
  material_ids: number[];
  guide: boolean;
};
export type AgentInput = {
  name: string;
  prompt: string;
  enabled: boolean;
  skill_ids: number[];
  material_ids: number[];
};
export type McpTool = { name: string; description: string };
export type McpServer = {
  id: number;
  name: string;
  base_url: string;
  connected: boolean;
  has_token: boolean;
  tools: McpTool[];
};
export type McpAudit = {
  tool: string;
  status: string;
  round?: number;
  conversation_id?: number;
  username: string;
  created_at: string;
};
export type ToolCallRecord = { ref: string; status: "ok" | "error" | "denied"; text?: string };
export type Conversation = { id: number; title: string; agent_id: number; created_at: string; preview: string };
export type TutorMessage = {
  id: number;
  role: "user" | "assistant";
  content: string;
  citations?: Citation[];
};

export type StreamHandlers = {
  onMeta?: (meta: {
    status: string;
    skill: { key: string; name: string; enabled: boolean };
    citations: Citation[];
    hit_count: number;
    prompt_sha256: string;
  }) => void;
  onCitations?: (citations: Citation[]) => void;
  onToolCalls?: (round: number, calls: ToolCallRecord[]) => void;
  onDelta?: (text: string) => void;
  onDone?: (done: { answer: string; message_id: number; model_called: boolean }) => void;
};
