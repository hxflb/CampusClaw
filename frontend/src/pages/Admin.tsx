import { FormEvent, useEffect, useRef, useState } from "react";
import { Agent, AgentInput, Material, McpAudit, McpServer, SkillFile, api } from "../api";

const EMPTY: AgentInput = { name: "", prompt: "", enabled: true, skill_ids: [], material_ids: [] };
const SKILL_TEMPLATE = `名称：数学解题引导
适用条件：方程,去括号,分数,通分,应用题
资料：数学
工具：knowledge_search
启用：是

学生正在自己做数学作业，往往已经写了一步。先看懂他卡在哪一步，只指出下一步该检查什么。不要算出最终得数，不要把整道题做完。依据只用数学自学笔记。
`;

type Tab = "agents" | "skills" | "mcp";

function StatusTag({ on }: { on: boolean }) {
  return <span className={on ? "state-tag is-on" : "state-tag is-off"}>{on ? "已启用" : "已关闭"}</span>;
}

function explain(err: unknown) {
  const msg = (err as Error).message || "请求失败";
  if (msg === "forbidden") return "没有权限";
  if (msg === "name_taken") return "本班已有同名项目";
  if (msg === "bad_skill") return "技能文件需要名称、适用条件、资料、工具和启用，空一行后写说明。工具写 knowledge_search，或已连接服务器上的“工具名@服务器名”。启用只能写是或否。";
  if (msg === "mcp_tool_denied") return "这个 MCP 工具还没在本班已连接的服务器里";
  if (msg === "mcp_unavailable") return (err as Error & { detail?: string }).detail || "连接失败。检索和对话仍可使用。";
  if (msg === "bad_request") return "名称、提示词和启用状态都要填写";
  if (msg === "not_found") return "技能或材料不属于本班";
  return msg;
}

export default function AdminPage() {
  const [tab, setTab] = useState<Tab>("agents");
  const [servers, setServers] = useState<McpServer[]>([]);
  const [audit, setAudit] = useState<McpAudit[]>([]);
  const [mcpName, setMcpName] = useState("");
  const [mcpURL, setMcpURL] = useState("http://mcp-demo:8090");
  const [mcpToken, setMcpToken] = useState("");
  const [materials, setMaterials] = useState<Material[]>([]);
  const [skills, setSkills] = useState<SkillFile[]>([]);
  const [agents, setAgents] = useState<Agent[]>([]);
  const [form, setForm] = useState<AgentInput>(EMPTY);
  const [editing, setEditing] = useState(0);
  const [agentOpen, setAgentOpen] = useState(false);
  const [skillId, setSkillId] = useState(0);
  const [skillOpen, setSkillOpen] = useState(false);
  const [skillText, setSkillText] = useState(SKILL_TEMPLATE);
  const [note, setNote] = useState("");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const fileRef = useRef<HTMLInputElement>(null);

  async function load() {
    const [mats, ag, sk, mcp, rows] = await Promise.all([
      api.materials(), api.agents(), api.skillFiles(), api.mcpServers(), api.mcpAudit(),
    ]);
    setMaterials(mats.items);
    setAgents(ag.items);
    setSkills(sk.items);
    setServers(mcp.items);
    setAudit(rows.items);
  }

  useEffect(() => {
    load().catch((ex) => setErr(explain(ex)));
  }, []);

  async function act(job: () => Promise<void>) {
    setBusy(true);
    setErr("");
    setNote("");
    try {
      await job();
      await load();
    } catch (ex) {
      setErr(explain(ex));
    } finally {
      setBusy(false);
    }
  }

  function openAgent(agent?: Agent) {
    setEditing(agent?.id || 0);
    setForm(agent ? {
      name: agent.name,
      prompt: agent.prompt,
      enabled: agent.enabled,
      skill_ids: agent.skill_ids || [],
      material_ids: agent.material_ids || [],
    } : EMPTY);
    setAgentOpen(true);
    setNote("");
    setErr("");
  }

  function closeAgent() {
    setAgentOpen(false);
    setEditing(0);
    setForm(EMPTY);
  }

  function toggleId(list: number[], id: number, on: boolean) {
    return on ? [...list, id] : list.filter((n) => n !== id);
  }

  function pickAll(ids: number[], on: boolean) {
    return on ? ids : [];
  }

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setErr("");
    setNote("");
    try {
      const creating = editing === 0;
      if (creating) await api.createAgent(form);
      else await api.saveAgent(editing, form);
      await load();
      closeAgent();
      setNote(creating ? "智能体已创建" : "智能体已保存");
    } catch (ex) {
      setErr(explain(ex));
    } finally {
      setBusy(false);
    }
  }

  function openSkill(skill?: SkillFile) {
    setSkillId(skill?.id || 0);
    setSkillText(skill?.source || SKILL_TEMPLATE);
    setSkillOpen(true);
    setNote("");
    setErr("");
  }

  function closeSkill() {
    setSkillOpen(false);
    setSkillId(0);
    setSkillText(SKILL_TEMPLATE);
  }

  async function saveSkill(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setErr("");
    setNote("");
    try {
      const creating = skillId === 0;
      if (creating) await api.createSkillFile(skillText);
      else await api.saveSkillFile(skillId, skillText);
      await load();
      closeSkill();
      setNote(creating ? "技能文件已添加" : "技能文件已保存");
    } catch (ex) {
      setErr(explain(ex));
    } finally {
      setBusy(false);
    }
  }

  async function toggleAgent(agent: Agent) {
    setErr("");
    setNote("");
    try {
      await api.saveAgent(agent.id, {
        name: agent.name,
        prompt: agent.prompt,
        enabled: !agent.enabled,
        skill_ids: agent.skill_ids || [],
        material_ids: agent.material_ids || [],
      });
      await load();
    } catch (ex) {
      setErr(explain(ex));
    }
  }

  async function toggleSkill(skill: SkillFile) {
    setErr("");
    setNote("");
    try {
      await api.setSkillFileEnabled(skill.id, !skill.enabled);
      await load();
    } catch (ex) {
      setErr(explain(ex));
    }
  }

  async function removeSkill(skill: SkillFile) {
    setErr("");
    setNote("");
    try {
      await api.removeSkillFile(skill.id);
      if (skillId === skill.id) closeSkill();
      await load();
    } catch (ex) {
      setErr(explain(ex));
    }
  }

  function uploadSkill(file: File) {
    const name = file.name.toLowerCase();
    if (!name.endsWith(".md") && !name.endsWith(".txt")) {
      setErr("请选择 .md 或 .txt 文件");
      return;
    }
    file.text().then((text) => {
      setSkillText(text);
      setSkillOpen(true);
      setErr("");
    }).catch(() => setErr("文件读取失败"));
  }

  return (
    <section className="card panel admin-page">
      <h1><i className="fa-solid fa-robot" /> 智能体管理</h1>
      <div className="admin-tabs" role="tablist">
        <button type="button" className={tab === "agents" ? "active" : ""} onClick={() => { setTab("agents"); closeAgent(); closeSkill(); }}>智能体</button>
        <button type="button" className={tab === "skills" ? "active" : ""} onClick={() => { setTab("skills"); closeAgent(); closeSkill(); }}>技能</button>
        <button type="button" className={tab === "mcp" ? "active" : ""} onClick={() => { setTab("mcp"); closeAgent(); closeSkill(); }}>MCP</button>
      </div>
      {note && <p className="note">{note}</p>}
      {err && <p className="alert">{err}</p>}

      {tab === "skills" && !skillOpen && (
        <div className="skill-admin">
          {skills.length === 0 && <p className="empty">还没有技能。</p>}
          {skills.map((skill) => (
            <article key={skill.id} className="skill-card">
              <div className="skill-pick">
                <div className="skill-head">
                  <strong>{skill.name}</strong>
                  <StatusTag on={skill.enabled} />
                </div>
                <span>适用：{skill.trigger || "未填写"}</span>
              </div>
              <div className="skill-actions">
                <button className="btn btn-ghost" type="button" onClick={() => openSkill(skill)}>修改</button>
                <button className="btn btn-ghost" type="button" onClick={() => toggleSkill(skill)}>{skill.enabled ? "关闭" : "启用"}</button>
                <button className="btn btn-ghost" type="button" onClick={() => removeSkill(skill)}>删除</button>
              </div>
            </article>
          ))}
          <div className="skill-toolbar">
            <button className="btn btn-primary" type="button" onClick={() => openSkill()}>新建</button>
          </div>
        </div>
      )}

      {tab === "skills" && skillOpen && (
        <form className="skill-admin" onSubmit={saveSkill}>
          <p className="lead">工具写 knowledge_search，或写成 工具名@服务器名。服务器要先在 MCP 页连接成功。</p>
          <label htmlFor="skill-source">{skillId ? "修改技能文件" : "新的技能文件"}
            <textarea id="skill-source" rows={12} value={skillText} onChange={(e) => setSkillText(e.target.value)} required />
          </label>
          <div className="skill-toolbar">
            <button className="btn btn-ghost" type="button" onClick={() => fileRef.current?.click()}>上传 .md / .txt</button>
            <input
              ref={fileRef}
              type="file"
              accept=".md,.txt,text/plain,text/markdown"
              hidden
              onChange={(e) => {
                const file = e.target.files?.[0];
                e.target.value = "";
                if (file) uploadSkill(file);
              }}
            />
          </div>
          <button className="btn btn-primary btn-block" type="submit" disabled={busy}>
            {busy ? "保存中…" : "保存技能文件"}
          </button>
          <button className="btn btn-ghost btn-block" type="button" onClick={closeSkill}>返回列表</button>
        </form>
      )}

      {tab === "agents" && !agentOpen && (
        <div className="skill-admin">
          {agents.length === 0 && <p className="empty">还没有智能体。</p>}
          {agents.map((agent) => (
            <article key={agent.id} className="skill-card">
              <div className="skill-pick">
                <div className="skill-head">
                  <strong>{agent.name}</strong>
                  <StatusTag on={agent.enabled} />
                </div>
              </div>
              <div className="skill-actions">
                <button className="btn btn-ghost" type="button" onClick={() => openAgent(agent)}>修改</button>
                <button className="btn btn-ghost" type="button" onClick={() => toggleAgent(agent)}>{agent.enabled ? "关闭" : "启用"}</button>
              </div>
            </article>
          ))}
          <div className="skill-toolbar">
            <button className="btn btn-primary" type="button" onClick={() => openAgent()}>新建</button>
          </div>
        </div>
      )}

      {tab === "agents" && agentOpen && (
        <form className="agent-form" onSubmit={submit}>
          <label htmlFor="agent-name">名称
            <input id="agent-name" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required />
          </label>
          <label htmlFor="agent-prompt">角色提示词
            <textarea id="agent-prompt" rows={6} value={form.prompt} onChange={(e) => setForm({ ...form, prompt: e.target.value })} required />
          </label>
          <fieldset>
            <legend>技能</legend>
            <p className="lead">只采用勾选的技能。未勾选的技能这个智能体不能使用。</p>
            {skills.length === 0 && <p className="lead">还没有技能文件。</p>}
            {skills.length > 0 && (
              <div className="check-group">
                <label className="check check-all">
                  <input
                    type="checkbox"
                    checked={skills.every((skill) => form.skill_ids.includes(skill.id))}
                    ref={(el) => {
                      if (!el) return;
                      const picked = skills.filter((skill) => form.skill_ids.includes(skill.id)).length;
                      el.indeterminate = picked > 0 && picked < skills.length;
                    }}
                    onChange={(e) => setForm({ ...form, skill_ids: pickAll(skills.map((skill) => skill.id), e.target.checked) })}
                  />
                  全选
                </label>
                <div className="check-items">
                  {skills.map((skill) => (
                    <label key={skill.id} className="check">
                      <input
                        type="checkbox"
                        checked={form.skill_ids.includes(skill.id)}
                        onChange={(e) => setForm({ ...form, skill_ids: toggleId(form.skill_ids, skill.id, e.target.checked) })}
                      />
                      {skill.name}
                    </label>
                  ))}
                </div>
              </div>
            )}
          </fieldset>
          <fieldset>
            <legend>材料</legend>
            <p className="lead">只检索勾选的材料。未勾选的材料这个智能体不能查看。</p>
            {materials.length === 0 && <p className="lead">本班还没有材料。</p>}
            {materials.length > 0 && (
              <div className="check-group">
                <label className="check check-all">
                  <input
                    type="checkbox"
                    checked={materials.every((m) => form.material_ids.includes(m.id))}
                    ref={(el) => {
                      if (!el) return;
                      const picked = materials.filter((m) => form.material_ids.includes(m.id)).length;
                      el.indeterminate = picked > 0 && picked < materials.length;
                    }}
                    onChange={(e) => setForm({ ...form, material_ids: pickAll(materials.map((m) => m.id), e.target.checked) })}
                  />
                  全选
                </label>
                <div className="check-items">
                  {materials.map((m) => (
                    <label key={m.id} className="check">
                      <input
                        type="checkbox"
                        checked={form.material_ids.includes(m.id)}
                        onChange={(e) => setForm({ ...form, material_ids: toggleId(form.material_ids, m.id, e.target.checked) })}
                      />
                      {m.title}
                    </label>
                  ))}
                </div>
              </div>
            )}
          </fieldset>
          <button className="btn btn-primary btn-block" type="submit" disabled={busy}>
            {busy ? "保存中…" : editing ? "保存智能体" : "创建智能体"}
          </button>
          <button className="btn btn-ghost btn-block" type="button" onClick={closeAgent}>返回列表</button>
        </form>
      )}

      {tab === "mcp" && (
        <div className="skill-admin">
          <p className="lead">登记本班 MCP 服务器的名称和地址。令牌只写不读：提交后不再回显，接口只回「是否已配令牌」。</p>
          <form className="agent-form" onSubmit={async (e) => {
            e.preventDefault();
            setBusy(true);
            setErr("");
            setNote("");
            try {
              await api.createMcpServer(mcpName.trim(), mcpURL.trim(), mcpToken.trim());
              setMcpName("");
              setMcpToken("");
              setNote("服务器已登记");
              await load();
            } catch (ex) {
              setErr(explain(ex));
            } finally {
              setBusy(false);
            }
          }}>
            <label htmlFor="mcp-name">名称
              <input id="mcp-name" value={mcpName} onChange={(e) => setMcpName(e.target.value)} placeholder="campus-tools" required />
            </label>
            <label htmlFor="mcp-url">Base URL
              <input id="mcp-url" value={mcpURL} onChange={(e) => setMcpURL(e.target.value)} required />
            </label>
            <label htmlFor="mcp-token">令牌（可留空，回退到服务端环境变量）
              <input id="mcp-token" type="password" value={mcpToken} onChange={(e) => setMcpToken(e.target.value)} autoComplete="new-password" />
            </label>
            <button className="btn btn-primary" type="submit" disabled={busy}>登记</button>
          </form>
          {servers.length === 0 && <p className="empty">还没有 MCP 服务器。</p>}
          {servers.map((server) => (
            <article key={server.id} className="skill-card">
              <div className="skill-pick">
                <div className="skill-head">
                  <strong>{server.name}</strong>
                  <span className={server.connected ? "state-tag is-on" : "state-tag is-off"}>{server.connected ? "已连接" : "未连接"}</span>
                  <span className={server.has_token ? "state-tag is-on" : "state-tag is-off"}>{server.has_token ? "已配令牌" : "未配令牌"}</span>
                </div>
                <span>{server.base_url}</span>
                {server.tools.length === 0 && <span>尚未发现工具</span>}
                {server.tools.length > 0 && (
                  <ul className="tool-list">
                    {server.tools.map((tool) => (
                      <li key={tool.name}>
                        <code>{tool.name}</code>
                        <span className="tool-desc">{tool.description || "（没有说明）"}</span>
                      </li>
                    ))}
                  </ul>
                )}
              </div>
              <div className="skill-actions">
                <button className="btn btn-ghost" type="button" disabled={busy} onClick={() => act(async () => {
                  await api.connectMcpServer(server.id);
                  setNote("已连接");
                })}>连接</button>
                <button className="btn btn-ghost" type="button" disabled={busy} onClick={() => act(async () => {
                  await api.disconnectMcpServer(server.id);
                  setNote("已断开");
                })}>断开</button>
                <button className="btn btn-ghost" type="button" disabled={busy} onClick={() => act(async () => {
                  await api.removeMcpServer(server.id);
                  setNote("已删除");
                })}>删除</button>
              </div>
            </article>
          ))}
          <h2>调用审计</h2>
          {audit.length === 0 && <p className="empty">还没有调用记录。</p>}
          {audit.length > 0 && (
            <ul className="list">
              {audit.map((row, i) => (
                <li key={`${row.created_at}-${row.tool}-${i}`} className="item">
                  <div className="item-main">
                    <strong>{row.tool}</strong>
                    <span className="meta">
                      <span className="meta-item">{row.status}</span>
                      <span className="meta-item">{row.username}</span>
                      {typeof row.round === "number" && row.round > 0 && (
                        <span className="meta-item">第 {row.round} 轮</span>
                      )}
                      {row.conversation_id && <span className="meta-item">会话 {row.conversation_id}</span>}
                      <span className="meta-item">{row.created_at}</span>
                    </span>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </section>
  );
}
