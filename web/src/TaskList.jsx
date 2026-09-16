import { useEffect, useRef, useState } from "react";
import { statuses, usePolling } from "./api";
import {
  alert,
  badge,
  dangerButton,
  dot,
  gridRow,
  rowButton,
  statusDot,
} from "./ui";

const ROW_HEIGHT = 56;
const PAGE_SIZE = 100;
const OVERSCAN = 4;
const MAX_SPACE = 30000000;

export default function TaskList({ base, filter, revision, busy, mutate }) {
  const viewport = useRef(null);
  const [scrollTop, setScrollTop] = useState(0);
  const [height, setHeight] = useState(480);
  const [total, setTotal] = useState(0);
  const scaled = total * ROW_HEIGHT > MAX_SPACE;
  const space = scaled ? MAX_SPACE : total * ROW_HEIGHT;
  const visible = Math.ceil(height / ROW_HEIGHT) + 1;
  const ratio = Math.min(
    1,
    Math.max(0, scrollTop / Math.max(1, space - height)),
  );
  const anchor = scaled
    ? Math.round(ratio * Math.max(0, total - visible))
    : Math.floor(scrollTop / ROW_HEIGHT);
  const first = Math.max(0, anchor - OVERSCAN);
  const last = Math.min(total, anchor + visible + OVERSCAN);
  const origin = scaled
    ? scrollTop - (anchor - first) * ROW_HEIGHT
    : first * ROW_HEIGHT;
  const offset = Math.floor(first / PAGE_SIZE) * PAGE_SIZE;
  const path = `${base}/tasks?offset=${offset}&limit=200&status=${filter}`;
  const { data, error } = usePolling(path, revision);

  useEffect(() => {
    const observer = new ResizeObserver(([entry]) =>
      setHeight(entry.contentRect.height),
    );
    observer.observe(viewport.current);
    return () => observer.disconnect();
  }, []);

  useEffect(() => {
    if (!data) return;
    setTotal(data.total);
    const maximum = Math.max(
      0,
      Math.min(data.total * ROW_HEIGHT, MAX_SPACE) - height,
    );
    if (viewport.current.scrollTop > maximum) {
      viewport.current.scrollTop = maximum;
      setScrollTop(maximum);
    }
  }, [data, height]);

  const rows = [];
  for (let index = first; index < last; index++) {
    const task = data?.items[index - offset];
    rows.push(
      <div
        role="row"
        aria-rowindex={index + 2}
        className={`task-row ${gridRow} absolute border-b border-line text-md hover:bg-hover`}
        key={task?.id ?? `loading-${index}`}
        style={{
          top: origin + (index - first) * ROW_HEIGHT,
          height: ROW_HEIGHT,
        }}
      >
        <div role="cell" className="text-2xs text-faint tabular-nums">
          {task ? `#${task.id}` : "…"}
        </div>
        <div role="cell" className="flex min-w-0 items-baseline gap-3">
          <span className="truncate" title={task?.payload}>
            {task?.payload || "Chargement…"}
          </span>
          {task ? (
            <time
              className="shrink-0 text-2xs text-faint max-narrow:hidden"
              dateTime={task.createdAt}
            >
              {new Date(task.createdAt).toLocaleString("fr-FR")}
            </time>
          ) : null}
        </div>
        <div role="cell">
          {task ? (
            <span className={`${badge} ${task.status}`}>
              <span className={`${dot} ${statusDot[task.status]}`} />
              {statuses[task.status]}
            </span>
          ) : null}
        </div>
        <div role="cell" className="flex flex-wrap items-center gap-1.5">
          {task?.status === "running" ? (
            <>
              <button
                className={rowButton}
                disabled={busy}
                aria-label={`Terminer la tâche ${task.id}`}
                onClick={() =>
                  mutate(`${base}/tasks/${task.id}`, "PATCH", {
                    status: "completed",
                  })
                }
              >
                Terminer
              </button>
              <button
                className={dangerButton}
                disabled={busy}
                aria-label={`Marquer la tâche ${task.id} en erreur`}
                onClick={() =>
                  mutate(`${base}/tasks/${task.id}`, "PATCH", {
                    status: "failed",
                  })
                }
              >
                Erreur
              </button>
            </>
          ) : null}
          {task?.status === "failed" ? (
            <button
              className={rowButton}
              disabled={busy}
              aria-label={`Réessayer la tâche ${task.id}`}
              onClick={() =>
                mutate(`${base}/tasks/${task.id}`, "PATCH", {
                  status: "pending",
                })
              }
            >
              Réessayer
            </button>
          ) : null}
        </div>
      </div>,
    );
  }

  return (
    <>
      {error ? (
        <p role="alert" className={alert}>
          Chargement des tâches impossible : {error}. Nouvelle tentative
          automatique.
        </p>
      ) : null}
      <div
        role="table"
        aria-label="Tâches de la file"
        aria-rowcount={total + 1}
        aria-colcount={4}
        className="mt-2"
      >
        <div
          role="row"
          className={`${gridRow} h-8 border-b border-line text-2xs tracking-[0.6px] text-faint`}
        >
          <span role="columnheader">ID</span>
          <span role="columnheader">TÂCHE</span>
          <span role="columnheader">STATUT</span>
          <span role="columnheader">ACTIONS</span>
        </div>
        <div
          ref={viewport}
          className="task-viewport relative h-[clamp(320px,52vh,640px)] overflow-auto [overflow-anchor:none]"
          tabIndex={0}
          role="rowgroup"
          aria-label="Liste défilante des tâches"
          onScroll={(event) => setScrollTop(event.currentTarget.scrollTop)}
        >
          <div className="relative" style={{ height: space }}>
            {rows}
          </div>
          {total === 0 ? (
            <div
              className="absolute inset-0 grid place-items-center text-md text-faint"
              role="row"
            >
              <span role="cell">
                {!data
                  ? error
                    ? "En attente de la connexion…"
                    : "Chargement…"
                  : "Aucune tâche pour ce statut."}
              </span>
            </div>
          ) : null}
        </div>
      </div>
    </>
  );
}
