import { FormEvent, useState } from "react";
import { Link } from "react-router-dom";
import { api, SearchHit } from "../api";

export default function SearchPage() {
  const [q, setQ] = useState("");
  const [mode, setMode] = useState("hybrid");
  const [hits, setHits] = useState<SearchHit[] | null>(null);
  const [err, setErr] = useState("");

  async function run(e: FormEvent) {
    e.preventDefault();
    setErr("");
    try {
      const r = await api.search(q, mode);
      setHits(r.hits);
    } catch (ex) {
      setHits([]);
      setErr((ex as Error).message);
    }
  }

  return (
    <section className="card panel">
      <h1><i className="fa-solid fa-magnifying-glass" /> 本班检索</h1>
      <p className="lead">班级只从登录会话读取。无命中会显示空列表，不会编造出处。</p>
      <form className="upload-plain" onSubmit={run}>
        <label>问题
          <span className="field">
            <i className="fa-solid fa-comment" />
            <input value={q} onChange={(e) => setQ(e.target.value)} required placeholder="用自然语言提问" />
          </span>
        </label>
        <div className="preset-row">
          {(["hybrid", "keyword", "vector"] as const).map((m) => (
            <button key={m} className={`btn ${mode === m ? "btn-primary" : "btn-ghost"}`} type="button" onClick={() => setMode(m)}>
              {m === "hybrid" ? "混合" : m === "keyword" ? "关键字" : "向量"}
            </button>
          ))}
        </div>
        <button className="btn btn-accent" type="submit"><i className="fa-solid fa-play" /> 检索</button>
      </form>
      {err && <p className="alert">{err}</p>}
      {hits && hits.length === 0 && <p className="empty">资料中未找到相关内容</p>}
      {hits && hits.length > 0 && (
        <ul className="list">
          {hits.map((h) => (
            <li key={h.chunk_id} className="item">
              <div className="item-main">
                <Link to={`/m/${h.material_id}`} state={{ back: "/search" }}>
                  <i className="fa-regular fa-file-lines" /> {h.material_title}
                </Link>
                <span className="meta">
                  <span className="meta-item">切片 {h.chunk_index + 1}</span>
                  <span className="meta-item">字符 {h.char_start}–{h.char_end}</span>
                </span>
                <p className="lead">{h.excerpt}</p>
              </div>
              <p className="actions">
                <Link className="btn btn-ghost" to={`/m/${h.material_id}`} state={{ back: "/search" }}>
                  <i className="fa-solid fa-book-open" /> 查看原文
                </Link>
              </p>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
