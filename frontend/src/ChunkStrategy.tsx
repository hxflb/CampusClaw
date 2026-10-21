import { ChunkOpts } from "./api";

export type ChunkForm = {
  strategy: ChunkOpts["strategy"];
  delimiter: string;
  maxChars: number;
  overlapPct: number;
  collapseWS: boolean;
  stripURLs: boolean;
};

export function defaultChunkForm(): ChunkForm {
  return {
    strategy: "custom",
    delimiter: "newline",
    maxChars: 800,
    overlapPct: 10,
    collapseWS: false,
    stripURLs: false,
  };
}

export function toChunkOpts(form: ChunkForm): ChunkOpts {
  const opts: ChunkOpts = { strategy: form.strategy };
  if (form.strategy === "hierarchy" || form.strategy === "custom") {
    opts.max_chars = form.maxChars;
    opts.overlap_pct = form.overlapPct;
  }
  if (form.strategy === "custom") {
    opts.delimiter = form.delimiter;
    opts.collapse_ws = form.collapseWS;
    opts.strip_urls = form.stripURLs;
  }
  return opts;
}

export function applyChunkFields(fd: FormData, opts: ChunkOpts) {
  fd.set("strategy", opts.strategy);
  if (opts.delimiter) fd.set("delimiter", opts.delimiter);
  if (opts.max_chars) fd.set("max_chars", String(opts.max_chars));
  if (opts.overlap_pct != null && opts.strategy !== "auto") fd.set("overlap_pct", String(opts.overlap_pct));
  if (opts.collapse_ws) fd.set("collapse_ws", "1");
  if (opts.strip_urls) fd.set("strip_urls", "1");
}

export default function ChunkStrategy({ value, onChange, borderOnly = false }: { value: ChunkForm; onChange: (next: ChunkForm) => void; borderOnly?: boolean }) {
  const set = (patch: Partial<ChunkForm>) => onChange({ ...value, ...patch });
  const klass = (key: ChunkForm["strategy"]) => `strategy-option${value.strategy === key ? " is-on" : ""}${borderOnly ? " border-only" : ""}`;
  return (
    <div className="chunk-box">
      <div className={klass("hierarchy")} onClick={borderOnly ? () => set({ strategy: "hierarchy" }) : undefined}>
        {!borderOnly && <input type="radio" name="strategy" aria-label="按层级分段" checked={value.strategy === "hierarchy"} onChange={() => set({ strategy: "hierarchy" })} />}
        <span onClick={() => set({ strategy: "hierarchy" })}>
          <strong><span className="recommend">推荐</span>按层级分段</strong>
          <small>按 Markdown 标题（#、##、###）把文档分成带标题的章节。某一章超过最大长度时，再按重叠比例切开。适合长文档。</small>
        </span>
        {value.strategy === "hierarchy" && (
          <div className="strategy-fields" onClick={(e) => e.stopPropagation()}>
            <label>分段最大长度
              <input type="number" min={100} max={2000} value={value.maxChars} onChange={(e) => set({ maxChars: Number(e.target.value) })} />
            </label>
            <label>分段重叠度%
              <input type="number" min={0} max={50} value={value.overlapPct} onChange={(e) => set({ overlapPct: Number(e.target.value) })} />
            </label>
          </div>
        )}
      </div>
      <label className={klass("auto")} onClick={borderOnly ? () => set({ strategy: "auto" }) : undefined}>
        {!borderOnly && <input type="radio" name="strategy" checked={value.strategy === "auto"} onChange={() => set({ strategy: "auto" })} />}
        <span>
          <strong>自动分段与清洗</strong>
          <small>优先在空行、换行、句号处断开。最大长度 800 字，重叠 80 字（10%），使用服务器默认值。</small>
        </span>
      </label>
      <div className={klass("custom")} onClick={borderOnly ? () => set({ strategy: "custom" }) : undefined}>
        {!borderOnly && <input type="radio" name="strategy" aria-label="自定义" checked={value.strategy === "custom"} onChange={() => set({ strategy: "custom" })} />}
        <span onClick={() => set({ strategy: "custom" })}>
          <strong>自定义</strong>
          <small>自己指定分段标识符、最大长度、重叠比例，以及入库前的文本清洗。</small>
        </span>
        {value.strategy === "custom" && (
          <div className="strategy-fields" onClick={(e) => e.stopPropagation()}>
            <label>分段标识符
              <select value={value.delimiter} onChange={(e) => set({ delimiter: e.target.value })}>
                <option value="blank">空行</option>
                <option value="newline">换行</option>
                <option value="period">句号</option>
              </select>
            </label>
            <label>分段最大长度
              <input type="number" min={100} max={2000} value={value.maxChars} onChange={(e) => set({ maxChars: Number(e.target.value) })} />
            </label>
            <label>分段重叠度%
              <input type="number" min={0} max={50} value={value.overlapPct} onChange={(e) => set({ overlapPct: Number(e.target.value) })} />
            </label>
            <fieldset>
              <legend>文本预处理规则</legend>
              <label className="check">
                <input type="checkbox" checked={value.collapseWS} onChange={(e) => set({ collapseWS: e.target.checked })} />
                替换掉连续的空格、换行符和制表符
              </label>
              <label className="check">
                <input type="checkbox" checked={value.stripURLs} onChange={(e) => set({ stripURLs: e.target.checked })} />
                删除所有 URL 和电子邮箱地址
              </label>
            </fieldset>
          </div>
        )}
      </div>
    </div>
  );
}
