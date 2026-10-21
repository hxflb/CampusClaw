import { FormEvent, useEffect, useRef } from "react";

type Props = {
  open: boolean;
  title: string;
  onCancel: () => void;
  onConfirm: () => void;
};

export default function ConfirmDelete({ open, title, onCancel, onConfirm }: Props) {
  const ref = useRef<HTMLDialogElement>(null);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    if (open && !el.open) el.showModal();
    if (!open && el.open) el.close();
  }, [open]);

  function submit(e: FormEvent) {
    e.preventDefault();
    onConfirm();
  }

  return (
    <dialog className="upload-dialog confirm-dialog" ref={ref} onCancel={onCancel}>
      <form className="dialog-inner" onSubmit={submit}>
        <h2><i className="fa-solid fa-triangle-exclamation" /> 确认删除</h2>
        <p className="lead">确定删除「{title}」？删除后无法恢复。</p>
        <p className="dialog-actions">
          <button className="btn btn-ghost" type="button" onClick={onCancel}>取消</button>
          <button className="btn btn-danger" type="submit"><i className="fa-solid fa-trash" /> 确认删除</button>
        </p>
      </form>
    </dialog>
  );
}
