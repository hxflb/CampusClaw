import { FormEvent, useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { api, Agent, Citation, Conversation, Me, SkillFile, ToolCallRecord } from "../api";
import ConfirmDelete from "../ConfirmDelete";

type Turn = {
  question: string;
  answer: string;
  citations: Citation[];
  tools: ToolCallRecord[];
};

function toolStatus(status: string) {
  if (status === "ok") return "成功";
  if (status === "denied") return "被拒";
  return "出错";
}

function explain(err: unknown) {
  const msg = (err as Error).message || "请求失败";
  if (msg === "forbidden") return "没有权限修改";
  if (msg === "unauthorized") return "请先登录";
  if (msg === "empty_query") return "请先写下问题";
  if (msg === "empty_prompt") return "提示词不能为空";
  if (msg === "vector_unavailable") return "检索暂时不可用";
  if (msg === "not_found") return "找不到这个会话或智能体";
  if (msg === "no_agent") return "本班没有已启用的智能体";
  if (msg === "started") return "对话开始后不能更换智能体";
  return msg;
}

function AnswerText({ text }: { text: string }) {
  const parts = text.split(/(\[\d+\])/g);
  return (
    <>
      {parts.map((part, i) =>
        /^\[\d+\]$/.test(part) ? (
          <span key={i} className="cite-mark">{part}</span>
        ) : (
          <span key={i}>{part}</span>
        )
      )}
    </>
  );
}

function AgentMenu({
  agents,
  agentId,
  disabled,
  locked,
  onChange,
}: {
  agents: Agent[];
  agentId: number;
  disabled: boolean;
  locked: boolean;
  onChange: (id: number) => void;
}) {
  const [open, setOpen] = useState(false);
  const root = useRef<HTMLDivElement>(null);
  const current = agents.find((a) => a.id === agentId)?.name || "选择智能体";

  useEffect(() => {
    if (!open) return;
    function onPointer(e: PointerEvent) {
      if (!root.current?.contains(e.target as Node)) setOpen(false);
    }
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") setOpen(false);
    }
    document.addEventListener("pointerdown", onPointer);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("pointerdown", onPointer);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  return (
    <div className="agent-menu" ref={root}>
      <button
        type="button"
        className="agent-menu-btn"
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-label="选择智能体"
        disabled={disabled}
        title={locked ? "对话开始后不能更换智能体" : undefined}
        onClick={() => setOpen((v) => !v)}
      >
        {current}
      </button>
      {open && (
        <ul className="agent-menu-list" role="listbox" aria-label="选择智能体">
          {agents.map((a) => (
            <li key={a.id}>
              <button
                type="button"
                role="option"
                aria-selected={a.id === agentId}
                className={a.id === agentId ? "is-on" : ""}
                onClick={() => {
                  setOpen(false);
                  onChange(a.id);
                }}
              >
                {a.name}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

function skillsFor(agent: Agent | undefined, skills: SkillFile[]) {
  if (!agent) return [];
  const byId = new Map(skills.map((skill) => [skill.id, skill]));
  return (agent.skill_ids || []).flatMap((id) => {
    const skill = byId.get(id);
    return skill && skill.enabled ? [skill] : [];
  });
}

function toTurns(messages: { role: string; content: string; citations?: Citation[] }[]): Turn[] {
  const out: Turn[] = [];
  for (const m of messages) {
    if (m.role === "user") out.push({ question: m.content, answer: "", citations: [], tools: [] });
    else if (out.length) {
      const last = out[out.length - 1];
      last.answer = m.content;
      last.citations = m.citations || [];
    }
  }
  return out;
}

export default function AskPage({ me }: { me: Me }) {
  const teacher = me.role === "teacher";
  const [ready, setReady] = useState(false);
  const [agents, setAgents] = useState<Agent[]>([]);
  const [skills, setSkills] = useState<SkillFile[]>([]);
  const [agentId, setAgentId] = useState(0);
  const [convs, setConvs] = useState<Conversation[]>([]);
  const [convId, setConvId] = useState(0);
  const [turns, setTurns] = useState<Turn[]>([]);
  const [draft, setDraft] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const [note, setNote] = useState("");
  const [deleteId, setDeleteId] = useState(0);
  const convRef = useRef(0);
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const scroller = bottomRef.current?.parentElement;
    if (scroller) scroller.scrollTop = scroller.scrollHeight;
  }, [turns, busy]);

  function convLabel(c: Conversation) {
    return c.title || c.preview || "新的提问";
  }

  async function openConv(id: number) {
    const data = await api.conversation(id);
    convRef.current = id;
    setConvId(id);
    if (data.agent_id) setAgentId(data.agent_id);
    sessionStorage.setItem("tutor-conv", String(id));
    setTurns(toTurns(data.messages));
  }

  useEffect(() => {
    let cancel = false;
    (async () => {
      try {
        const [ag, files] = await Promise.all([
          api.agents(),
          api.skillFiles(),
        ]);
        if (cancel) return;
        const usable = ag.items.filter((a) => a.enabled);
        setAgents(usable);
        setSkills(files.items);
        const first = usable[0]?.id || 0;
        setAgentId(first);
        let list = (await api.conversations()).items;
        if (list.length === 0 && first) {
          await api.createConversation(first);
          list = (await api.conversations()).items;
        }
        if (cancel) return;
        setConvs(list);
        const stored = Number(sessionStorage.getItem("tutor-conv") || "");
        const pick = list.find((c) => c.id === stored)?.id || list[0]?.id;
        if (pick) {
          try {
            await openConv(pick);
          } catch {
            sessionStorage.removeItem("tutor-conv");
            if (list[0]) await openConv(list[0].id);
          }
        }
      } catch (ex) {
        if (!cancel) setErr(explain(ex));
      } finally {
        if (!cancel) setReady(true);
      }
    })();
    return () => { cancel = true; };
  }, [teacher]);

  async function refreshList(selectId?: number) {
    const list = (await api.conversations()).items;
    setConvs(list);
    if (selectId) await openConv(selectId);
  }

  async function chooseAgent(id: number) {
    const conv = convRef.current;
    if (busy || !id || id === agentId || !conv || turns.length > 0) return;
    setErr("");
    setNote("");
    try {
      await api.setConversationAgent(conv, id);
      setAgentId(id);
      setConvs((prev) => prev.map((c) => (c.id === conv ? { ...c, agent_id: id } : c)));
    } catch (ex) {
      setErr(explain(ex));
    }
  }

  async function startNew() {
    if (busy || !agentId) return;
    setErr("");
    setNote("");
    try {
      const created = await api.createConversation(agentId);
      await refreshList(created.id);
      setDraft("");
    } catch (ex) {
      setErr(explain(ex));
    }
  }

  async function removeConversation(id: number) {
    setDeleteId(0);
    if (!id || busy) return;
    setErr("");
    setNote("");
    const removingCurrent = convRef.current === id;
    try {
      await api.removeConversation(id);
      if (sessionStorage.getItem("tutor-conv") === String(id)) sessionStorage.removeItem("tutor-conv");
      let list = (await api.conversations()).items;
      if (list.length === 0 && agentId) {
        const created = await api.createConversation(agentId);
        list = (await api.conversations()).items;
        setConvs(list);
        const next = list.find((c) => c.id === created.id)?.id || list[0]?.id;
        if (next) await openConv(next);
        else {
          convRef.current = 0;
          setConvId(0);
          setTurns([]);
        }
      } else {
        setConvs(list);
        if (removingCurrent) await openConv(list[0].id);
      }
      if (removingCurrent) setDraft("");
    } catch (ex) {
      setErr(explain(ex));
    }
  }

  async function run(e?: FormEvent) {
    e?.preventDefault();
    const question = draft.trim();
    const id = convRef.current;
    if (!question || busy || !id || !agentId) return;
    setErr("");
    setNote("");
    setBusy(true);
    setDraft("");
    setTurns((prev) => [...prev, { question, answer: "", citations: [], tools: [] }]);
    try {
      await api.streamMessage(id, question, agentId, {
        onCitations: (citations) => {
          if (convRef.current !== id) return;
          setTurns((prev) => {
            if (!prev.length) return prev;
            const next = prev.slice();
            next[next.length - 1] = { ...next[next.length - 1], citations };
            return next;
          });
        },
        onToolCalls: (_round, calls) => {
          if (convRef.current !== id) return;
          setTurns((prev) => {
            if (!prev.length) return prev;
            const next = prev.slice();
            const last = next[next.length - 1];
            next[next.length - 1] = { ...last, tools: [...last.tools, ...calls] };
            return next;
          });
        },
        onDelta: (text) => {
          if (convRef.current !== id) return;
          setTurns((prev) => {
            if (!prev.length) return prev;
            const next = prev.slice();
            const last = next[next.length - 1];
            next[next.length - 1] = { ...last, answer: last.answer + text };
            return next;
          });
        },
      });
      if (convRef.current === id) {
        const data = await api.conversation(id);
        // 重拉会话时保留本次刚拿到的工具调用，否则工具行会在回答结束时闪一下就消失
        setTurns((prev) => {
          const fresh = toTurns(data.messages);
          const mine = prev[prev.length - 1]?.tools || [];
          if (fresh.length && mine.length) {
            fresh[fresh.length - 1].tools = mine;
          }
          return fresh;
        });
        const list = (await api.conversations()).items;
        setConvs(list);
      }
    } catch (ex) {
      setErr(explain(ex));
    } finally {
      setBusy(false);
    }
  }

  if (!ready) return <p className="pad">加载中…</p>;

  const pending = convs.find((c) => c.id === deleteId);
  const enabledSkills = skillsFor(agents.find((a) => a.id === agentId), skills);

  return (
    <section className="card panel studio-page">
      <h1><i className="fa-solid fa-comments" /> 解题助手</h1>
      <p className="lead">{teacher ? "在对话里选择已启用的智能体试问。智能体在智能体管理里设置。" : "选择本班已启用的智能体。技能和提示词由教师设置。"}</p>
      {agents.length === 0 && <p className="empty">本班没有已启用的智能体。</p>}
      {note && <p className="note">{note}</p>}
      {err && <p className="alert">{err}</p>}

      <div className="studio">
        <aside className="studio-side" aria-label="最近对话">
          <button className="btn btn-primary" type="button" onClick={startNew} disabled={busy}>新对话</button>
          <p className="side-label">最近对话</p>
          <ul className="side-history">
            {convs.length === 0 && <li className="hint">还没有对话。</li>}
            {convs.map((c) => (
              <li key={c.id} className={c.id === convId ? "is-current" : ""}>
                <button
                  className="hist-name"
                  type="button"
                  aria-current={c.id === convId ? "true" : undefined}
                  onClick={() => {
                    setErr("");
                    openConv(c.id).catch((ex) => setErr(explain(ex)));
                  }}
                >
                  {convLabel(c)}
                </button>
                <button
                  className="conv-del"
                  type="button"
                  aria-label={`删除 ${c.preview || "新会话"}`}
                  disabled={busy}
                  onClick={() => setDeleteId(c.id)}
                >
                  <i className="fa-solid fa-trash" />
                </button>
              </li>
            ))}
          </ul>
        </aside>

        <div className="studio-main">
          {agents.length > 0 && (
            <div className="thread-agent">
              <span>智能体</span>
              <AgentMenu agents={agents} agentId={agentId} disabled={busy || turns.length > 0} locked={turns.length > 0} onChange={chooseAgent} />
            </div>
          )}
          {enabledSkills.length > 0 && (
            <div className="skill-hint" aria-label="已启用技能">
              <span className="skill-hint-label"><i className="fa-solid fa-route" /> 已启用</span>
              <ul>
                {enabledSkills.map((skill) => (
                  <li key={skill.id}>{skill.name}</li>
                ))}
              </ul>
            </div>
          )}
          <div className="thread">
            {turns.length === 0 && (
              <p className="studio-empty">今天帮你做些什么？写下你卡在哪一步</p>
            )}
            {turns.map((t, i) => (
              <article key={`${convId}-${i}`} className="turn">
                <p className="q-line"><i className="fa-solid fa-user" /> {t.question}</p>
                {t.tools.length > 0 && (
                  <p className="tool-line" aria-label="本次调用的工具">
                    <i className="fa-solid fa-screwdriver-wrench" /> 本次用到：
                    {t.tools.map((c, k) => (
                      <span key={`${c.ref}-${k}`} className={`tool-tag is-${c.status}`} title={c.text || ""}>
                        {c.ref}（{toolStatus(c.status)}）
                      </span>
                    ))}
                  </p>
                )}
                <div className="body answer-body">
                  <div className="answer-text">
                    {t.answer ? <AnswerText text={t.answer} /> : (busy && i === turns.length - 1 ? "正在回答…" : "")}
                  </div>
                  {t.citations.length > 0 && (
                    <div className="cite-block">
                      <p className="cite-label">参考资料</p>
                      <ul className="cite-list">
                        {t.citations.map((c) => (
                          <li key={c.n}>
                            <Link to={`/m/${c.material_id}`} state={{ back: "/ask" }}>
                              <span className="cite-mark">[{c.n}]</span> {c.material_title}
                            </Link>
                          </li>
                        ))}
                      </ul>
                    </div>
                  )}
                </div>
              </article>
            ))}
            <div ref={bottomRef} />
          </div>

          <form className="composer" onSubmit={run}>
            <textarea
              aria-label="向解题助手提问"
              rows={3}
              value={draft}
              disabled={busy || !convId || !agentId}
              placeholder="今天帮你做些什么？写下你卡在哪一步"
              onChange={(e) => setDraft(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter" && !e.shiftKey) {
                  e.preventDefault();
                  run();
                }
              }}
            />
            <button className="studio-send" type="submit" aria-label={busy ? "正在回答" : "发送"} disabled={busy || !convId || !agentId || !draft.trim()}>
              ↑
            </button>
          </form>
        </div>
      </div>

      <ConfirmDelete
        open={deleteId !== 0}
        title={pending?.preview || "新会话"}
        onCancel={() => setDeleteId(0)}
        onConfirm={() => removeConversation(deleteId)}
      />
    </section>
  );
}
