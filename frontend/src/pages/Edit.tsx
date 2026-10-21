import { FormEvent, useEffect, useLayoutEffect, useRef, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { api, Klass, Me } from "../api";
import ChunkStrategy, { applyChunkFields, ChunkForm, defaultChunkForm, toChunkOpts } from "../ChunkStrategy";
import FileDrop from "../FileDrop";

export default function Edit({ me }: { me: Me }) {
  const { id } = useParams();
  const nav = useNavigate();
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [classId, setClassId] = useState(me.class_id);
  const [classes, setClasses] = useState<Klass[]>([]);
  const [chunk, setChunk] = useState<ChunkForm>(defaultChunkForm);
  const [rebuilding, setRebuilding] = useState(false);
  const [note, setNote] = useState("");
  const [saving, setSaving] = useState(false);
  const [fileEpoch, setFileEpoch] = useState(0);
  const pickedFile = useRef(false);
  const keepScroll = useRef<number | null>(null);

  function changeChunk(next: ChunkForm) {
    if (next.strategy !== chunk.strategy) keepScroll.current = window.scrollY;
    setChunk(next);
  }

  useLayoutEffect(() => {
    if (keepScroll.current == null) return;
    const y = keepScroll.current;
    keepScroll.current = null;
    window.scrollTo(0, y);
  }, [chunk.strategy]);

  useEffect(() => {
    if (me.role !== "teacher") {
      nav("/");
      return;
    }
    api.classes().then((d) => setClasses(d.items));
    api.one(Number(id)).then((m) => {
      setTitle(m.title);
      setBody(m.body_text || "");
      setClassId(m.class_id);
    }).catch(() => nav("/"));
  }, [id]);

  async function save(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const fd = new FormData(e.currentTarget);
    const file = fd.get("file");
    if (!(file instanceof File) || !file.name) fd.delete("file");
    fd.set("title", title);
    fd.set("body", body);
    fd.set("class_id", String(classId));
    applyChunkFields(fd, toChunkOpts(chunk));
    setSaving(true);
    setNote("");
    try {
      await api.update(Number(id), fd);
      nav("/");
    } catch (err) {
      setNote("保存失败：" + (err as Error).message);
      setSaving(false);
    }
  }

  async function rebuild() {
    setRebuilding(true);
    setNote("");
    try {
      await api.reindex(Number(id), toChunkOpts(chunk));
      setNote("已按所选策略重建向量");
    } catch (err) {
      setNote("重建失败：" + (err as Error).message);
    } finally {
      setRebuilding(false);
    }
  }

  return (
    <>
      <p className="crumb"><Link className="btn btn-ghost" to="/"><i className="fa-solid fa-arrow-left" /> 返回材料列表</Link></p>
      <section className="card">
        <h1><i className="fa-solid fa-pen-to-square" /> 修改资料</h1>
        <form className="upload-plain edit-form" onSubmit={save}>
          <label>所属班级
            <span className="field">
              <i className="fa-solid fa-chalkboard" />
              <select value={classId} onChange={(e) => setClassId(Number(e.target.value))}>
                {classes.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
              </select>
            </span>
          </label>
          <label>标题
            <span className="field"><i className="fa-solid fa-pen" /><input value={title} onChange={(e) => setTitle(e.target.value)} required /></span>
          </label>
          <label>知识库正文
            <textarea value={body} onChange={(e) => {
              setBody(e.target.value);
              if (pickedFile.current) {
                pickedFile.current = false;
                setFileEpoch((n) => n + 1);
              }
            }} required rows={12} />
          </label>
          <div>
            <p className="upload-label">替换文件（可选）</p>
            <FileDrop
              name="file"
              resetKey={fileEpoch}
              onPick={(file) => {
                pickedFile.current = true;
                file.text().then((text) => setBody(text)).catch(() => setNote("无法读取该文件"));
              }}
              onReject={() => setNote("只接受 .txt 或 .md")}
            />
          </div>
          <h2>重建向量</h2>
          <p className="lead">按下面的规则重新切分并替换这条材料的旧切片。长度和重叠的默认值是 800 和 10%。</p>
          <ChunkStrategy borderOnly value={chunk} onChange={changeChunk} />
          {note && <p className="alert">{note}</p>}
          <p className="actions">
            <button className="btn btn-primary" type="submit" disabled={saving}><i className="fa-solid fa-floppy-disk" /> {saving ? "保存中…" : "保存修改"}</button>
            <button className="btn btn-ghost" type="button" disabled={rebuilding} onClick={rebuild}>
              {rebuilding ? "重建中…" : "重建向量"}
            </button>
          </p>
        </form>
      </section>
    </>
  );
}
