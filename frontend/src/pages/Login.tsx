import { FormEvent, useState } from "react";
import { useNavigate } from "react-router-dom";
import { api, Me } from "../api";

const PRESETS = [
  { username: "teacher_a", password: "demo123", label: "教师 A", icon: "fa-chalkboard-user" },
  { username: "student_a1", password: "demo123", label: "学生 A1", icon: "fa-user" },
  { username: "student_b1", password: "demo123", label: "学生 B1", icon: "fa-user" },
] as const;

export default function Login({ onLogin }: { onLogin: (m: Me) => void }) {
  const nav = useNavigate();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [err, setErr] = useState("");

  async function submit(e: FormEvent) {
    e.preventDefault();
    setErr("");
    try {
      await api.login(username, password);
      const me = await api.me();
      onLogin(me);
      nav("/");
    } catch {
      setErr("用户名或密码错误");
    }
  }

  return (
    <section className="card login-card">
      <div className="card-icon"><i className="fa-solid fa-lock" /></div>
      <h1>进入本班知识库</h1>
      <p className="lead">教师管理材料入库，学生只读本班列表。</p>
      {err && <p className="alert"><i className="fa-solid fa-circle-exclamation" /> {err}</p>}
      <form onSubmit={submit}>
        <label>用户名
          <span className="field">
            <i className="fa-solid fa-user" />
            <input value={username} onChange={(e) => setUsername(e.target.value)} required autoComplete="username" />
          </span>
        </label>
        <label>密码
          <span className="field">
            <i className="fa-solid fa-key" />
            <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} required autoComplete="current-password" />
          </span>
        </label>
        <button className="btn btn-primary btn-block" type="submit">登录</button>
      </form>
      <div className="hint">
        <p>预置账号（点击填入）</p>
        <div className="preset-row">
          {PRESETS.map((p) => (
            <button
              key={p.username}
              className="btn btn-ghost"
              type="button"
              onClick={() => {
                setUsername(p.username);
                setPassword(p.password);
                setErr("");
              }}
            >
              <i className={`fa-solid ${p.icon}`} /> {p.label}
            </button>
          ))}
        </div>
      </div>
    </section>
  );
}
