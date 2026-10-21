import { useEffect, useState } from "react";
import { NavLink, Navigate, Route, Routes, useNavigate } from "react-router-dom";
import { api, Me } from "./api";
import Login from "./pages/Login";
import Materials from "./pages/Materials";
import Detail from "./pages/Detail";
import Edit from "./pages/Edit";
import AskPage from "./pages/Ask";
import AdminPage from "./pages/Admin";

export default function App() {
  const [me, setMe] = useState<Me | null>(null);
  const [ready, setReady] = useState(false);
  const [theme, setTheme] = useState(localStorage.getItem("theme") || "light");
  const nav = useNavigate();

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    localStorage.setItem("theme", theme);
  }, [theme]);

  useEffect(() => {
    api.me().then(setMe).catch(() => setMe(null)).finally(() => setReady(true));
  }, []);

  async function logout() {
    await api.logout();
    sessionStorage.removeItem("tutor-conv");
    setMe(null);
    nav("/login");
  }

  if (!ready) return <p className="pad">加载中…</p>;

  return (
    <div className="shell">
      <header className="top">
        <NavLink className="brand" to={me ? "/" : "/login"}>
          <i className="fa-solid fa-paw" /> CampusClaw
        </NavLink>
        <p className="tag"><i className="fa-solid fa-book-open" /> 班级知识库</p>
        <nav>
          <button className="btn btn-ghost" type="button" onClick={() => setTheme(theme === "dark" ? "light" : "dark")}>
            <i className={`fa-solid ${theme === "dark" ? "fa-sun" : "fa-moon"}`} /> {theme === "dark" ? "浅色" : "深色"}
          </button>
          {me && (
            <>
              <NavLink className="btn btn-ghost" to="/"><i className="fa-solid fa-folder-open" /> 材料</NavLink>
              <NavLink className="btn btn-ghost" to="/ask"><i className="fa-solid fa-comments" /> 解题助手</NavLink>
              {me.role === "teacher" && (
                <>
                  <NavLink className="btn btn-ghost" to="/admin"><i className="fa-solid fa-robot" /> 智能体管理</NavLink>
                  <a className="btn btn-ghost" href="/swagger/">接口文档</a>
                </>
              )}
              <span className="who">
                <i className={`fa-solid ${me.role === "teacher" ? "fa-chalkboard-user" : "fa-user"}`} />
                {me.username} · {me.role === "teacher" ? "教师" : "学生"}
              </span>
              <button className="btn btn-ghost" type="button" onClick={logout}>
                <i className="fa-solid fa-right-from-bracket" /> 退出
              </button>
            </>
          )}
        </nav>
      </header>
      <Routes>
        <Route path="/login" element={me ? <Navigate to="/" replace /> : <Login onLogin={setMe} />} />
        <Route path="/" element={me ? <Materials me={me} /> : <Navigate to="/login" replace />} />
        <Route path="/search" element={<Navigate to="/ask" replace />} />
        <Route path="/ask" element={me ? <AskPage me={me} /> : <Navigate to="/login" replace />} />
        <Route path="/admin" element={me?.role === "teacher" ? <AdminPage /> : <Navigate to={me ? "/" : "/login"} replace />} />
        <Route path="/m/:id" element={me ? <Detail me={me} /> : <Navigate to="/login" replace />} />
        <Route path="/m/:id/edit" element={me ? <Edit me={me} /> : <Navigate to="/login" replace />} />
      </Routes>
    </div>
  );
}
