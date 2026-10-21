import { useEffect, useState } from "react";
import { Link, useLocation, useNavigate, useParams } from "react-router-dom";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { api, Material, Me } from "../api";
import ConfirmDelete from "../ConfirmDelete";

export default function Detail({ me }: { me: Me }) {
  const { id } = useParams();
  const nav = useNavigate();
  const loc = useLocation();
  const back = (loc.state as { back?: string } | null)?.back || "/";
  const [m, setM] = useState<Material | null>(null);
  const [err, setErr] = useState("");
  const [askDel, setAskDel] = useState(false);

  useEffect(() => {
    api.one(Number(id)).then(setM).catch((e) => setErr((e as Error).message));
  }, [id]);

  async function del() {
    if (!m) return;
    setAskDel(false);
    await api.remove(m.id);
    nav("/");
  }

  if (err) {
    return (
      <section className="card">
        <div className="card-icon"><i className="fa-solid fa-shield-halved" /></div>
        <h1>无法访问</h1>
        <p className="lead">没有权限或资源不存在。</p>
        <Link className="btn btn-ghost" to={back}><i className="fa-solid fa-arrow-left" /> 返回</Link>
      </section>
    );
  }
  if (!m) return <p className="pad">加载中…</p>;

  return (
    <>
      <p className="crumb"><Link className="btn btn-ghost" to={back}><i className="fa-solid fa-arrow-left" /> 返回</Link></p>
      <article className="card">
        <h1><i className="fa-regular fa-file-lines" /> {m.title}</h1>
        <p className="lead"><i className="fa-solid fa-database" /> 知识库正文</p>
        {me.role === "teacher" && (
          <p className="actions">
            <Link className="btn btn-ghost" to={`/m/${m.id}/edit`}><i className="fa-solid fa-pen-to-square" /> 修改</Link>
            <button className="btn btn-danger" type="button" onClick={() => setAskDel(true)}><i className="fa-solid fa-trash" /> 删除</button>
            <button className="btn btn-ghost" type="button" onClick={() => api.download(m.id, m.title)}><i className="fa-solid fa-download" /> 下载原文</button>
          </p>
        )}
        {me.role === "student" && (
          <p className="actions">
            <button className="btn btn-ghost" type="button" onClick={() => api.download(m.id, m.title)}><i className="fa-solid fa-download" /> 下载原文</button>
          </p>
        )}
        <div className="body markdown">
          <Markdown remarkPlugins={[remarkGfm]}>{m.body_text || ""}</Markdown>
        </div>
      </article>
      <ConfirmDelete
        open={askDel}
        title={m.title}
        onCancel={() => setAskDel(false)}
        onConfirm={del}
      />
    </>
  );
}
