import { useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { api, Klass, Material, Me } from "../api";
import ChunkStrategy, { applyChunkFields, ChunkForm, defaultChunkForm, toChunkOpts } from "../ChunkStrategy";
import ConfirmDelete from "../ConfirmDelete";
import FileDrop from "../FileDrop";

function ClassMenu({
  classes,
  classId,
  className,
  onChange,
}: {
  classes: Klass[];
  classId: number;
  className: string;
  onChange: (id: number) => void;
}) {
  const [open, setOpen] = useState(false);
  const root = useRef<HTMLDivElement>(null);
  const options = classes.length ? classes : [{ id: classId, name: className }];

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
    <div className="title-class" ref={root}>
      <button
        type="button"
        className="title-class-btn"
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-label="选择班级"
        onClick={() => setOpen((v) => !v)}
      >
        {className}
      </button>
      {open && (
        <ul className="title-class-menu" role="listbox" aria-label="选择班级">
          {options.map((c) => (
            <li key={c.id}>
              <button
                type="button"
                role="option"
                aria-selected={c.id === classId}
                className={c.id === classId ? "is-on" : ""}
                onClick={() => {
                  setOpen(false);
                  if (c.id !== classId) onChange(c.id);
                }}
              >
                {c.name}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

export default function Materials({ me }: { me: Me }) {
  const [classes, setClasses] = useState<Klass[]>([]);
  const [classId, setClassId] = useState(me.class_id);
  const [items, setItems] = useState<Material[]>([]);
  const [q, setQ] = useState("");
  const [msg, setMsg] = useState("");
  const [uploading, setUploading] = useState(false);
  const [pending, setPending] = useState<Material | null>(null);
  const [chunk, setChunk] = useState<ChunkForm>(defaultChunkForm);
  const [uploadFile, setUploadFile] = useState<File | null>(null);
  const [uploadErr, setUploadErr] = useState("");
  const uploadRef = useRef<HTMLDialogElement>(null);
  const className = classes.find((c) => c.id === classId)?.name || `班级 ${classId}`;

  async function load(cid = classId, query = q) {
    const data = await api.materials(me.role === "teacher" ? cid : undefined, query);
    setItems(data.items);
  }

  useEffect(() => {
    api.classes().then((d) => setClasses(d.items)).catch(() => {});
    load();
  }, []);

  async function changeClass(id: number) {
    setClassId(id);
    const data = await api.materials(id, q);
    setItems(data.items);
  }

  useEffect(() => {
    const el = uploadRef.current;
    if (!el) return;
    if (uploadFile && !el.open) el.showModal();
    if (!uploadFile && el.open) el.close();
  }, [uploadFile]);

  function takeFile(file: File) {
    setMsg("");
    setUploadErr("");
    setUploadFile(file);
  }

  function cancelUpload() {
    if (uploading) return;
    setUploadFile(null);
  }

  async function confirmUpload() {
    if (!uploadFile) return;
    const fd = new FormData();
    fd.set("file", uploadFile);
    fd.set("class_id", String(classId));
    applyChunkFields(fd, toChunkOpts(chunk));
    setUploading(true);
    try {
      await api.upload(fd);
      setMsg("已上传并写入知识库");
      setUploadErr("");
      setUploadFile(null);
      await load(classId, q);
    } catch (err) {
      setUploadErr("上传失败：" + (err as Error).message);
    } finally {
      setUploading(false);
    }
  }

  async function del() {
    if (!pending) return;
    const id = pending.id;
    setPending(null);
    await api.remove(id);
    await load(classId, q);
  }

  return (
    <section className="card panel">
      {msg && <p className="alert"><i className="fa-solid fa-circle-check" /> {msg}</p>}
      <h1>
        <i className="fa-solid fa-folder-open" />
        {me.role === "teacher" ? (
          <ClassMenu classes={classes} classId={classId} className={className} onChange={changeClass} />
        ) : className}
        <span>材料</span>
      </h1>
      {me.role === "teacher" ? (
        <p className="lead">在标题中切换班级；上传会入库到当前所选班级。</p>
      ) : (
        <p className="lead"><i className="fa-solid fa-eye" /> 只读本班材料，班级由服务端会话决定。</p>
      )}

      <div className="materials-toolbar">
        <span className="field search">
          <i className="fa-solid fa-magnifying-glass" />
          <input aria-label="本班材料搜索" placeholder="按标题筛选本班材料" value={q} onChange={(e) => setQ(e.target.value)} onBlur={() => load(classId, q)} onKeyDown={(e) => e.key === "Enter" && load(classId, q)} />
        </span>
      </div>

      {me.role === "teacher" && (
        <div className="upload-panel">
          <p className="upload-label">上传本班材料（.txt / .md）</p>
          <FileDrop
            handoff
            title="拖拽或点击上传文件"
            hint="支持 .txt / .md"
            onPick={takeFile}
            onReject={() => setMsg("只接受 .txt 或 .md")}
          />
        </div>
      )}

      {me.role === "teacher" && (
        <dialog className="upload-dialog strategy-dialog" ref={uploadRef} onCancel={(e) => { e.preventDefault(); cancelUpload(); }}>
          <div className="dialog-inner">
            <h2>分段策略</h2>
            <p className="lead">即将上传「{uploadFile?.name}」。正文先按这里的规则切成切片，再嵌入并写入向量库。</p>
            {uploadErr && <p className="alert">{uploadErr}</p>}
            <div className="strategy-scroll">
              <ChunkStrategy borderOnly value={chunk} onChange={setChunk} />
            </div>
            <p className="dialog-actions">
              <button className="btn btn-ghost" type="button" onClick={cancelUpload} disabled={uploading}>取消</button>
              <button className="btn btn-primary" type="button" onClick={confirmUpload} disabled={uploading}>
                {uploading ? "上传中…" : "确认上传"}
              </button>
            </p>
          </div>
        </dialog>
      )}

      {items.length === 0 ? (
        <p className="empty"><i className="fa-solid fa-inbox" /> 本班暂无材料。</p>
      ) : (
        <ul className="list">
          {items.map((m) => (
            <li key={m.id} className="item">
              <div className="item-main">
                <Link to={`/m/${m.id}`}><i className="fa-regular fa-file-lines" /> {m.title}</Link>
                <span className="meta">
                  <span className="meta-item"><i className="fa-solid fa-user-pen" />{m.uploader}</span>
                  <span className="meta-item"><i className="fa-regular fa-clock" />{m.created_at}</span>
                </span>
              </div>
              {me.role === "teacher" && (
                <p className="actions">
                  <Link className="btn btn-ghost" to={`/m/${m.id}/edit`}><i className="fa-solid fa-pen-to-square" /> 修改</Link>
                  <button className="btn btn-danger" type="button" onClick={() => setPending(m)}><i className="fa-solid fa-trash" /> 删除</button>
                </p>
              )}
            </li>
          ))}
        </ul>
      )}
      <ConfirmDelete
        open={pending !== null}
        title={pending?.title || ""}
        onCancel={() => setPending(null)}
        onConfirm={del}
      />
    </section>
  );
}
