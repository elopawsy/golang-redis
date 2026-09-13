import { useEffect, useRef, useState } from "react";
import { statuses, usePolling } from "./api";

const ROW_HEIGHT = 80;
const PAGE_SIZE = 100;
const OVERSCAN = 4;

export default function TaskList({ base, filter, revision, busy, mutate }) {
  const viewport = useRef(null);
  const [scrollTop, setScrollTop] = useState(0);
  const [height, setHeight] = useState(480);
  const [total, setTotal] = useState(0);
  const first = Math.max(0, Math.floor(scrollTop / ROW_HEIGHT) - OVERSCAN);
  const last = Math.min(
    total,
    Math.ceil((scrollTop + height) / ROW_HEIGHT) + OVERSCAN,
  );
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
    const maximum = Math.max(0, data.total * ROW_HEIGHT - height);
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
        className="task-row"
        key={task?.id ?? `loading-${index}`}
        style={{
          position: "absolute",
          top: index * ROW_HEIGHT,
          height: ROW_HEIGHT,
        }}
      >
        <div role="cell" className="task-id">
          {task ? `#${task.id}` : "…"}
        </div>
        <div role="cell" className="task-content">
          <span title={task?.payload}>{task?.payload || "Chargement…"}</span>
          {task ? (
            <time dateTime={task.createdAt}>
              {new Date(task.createdAt).toLocaleString("fr-FR")}
            </time>
          ) : null}
        </div>
        <div role="cell">
          {task ? (
            <span className={`badge ${task.status}`}>
              {statuses[task.status]}
            </span>
          ) : null}
        </div>
        <div role="cell" className="task-actions">
          {task?.status === "running" ? (
            <>
              <button
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
                className="danger-text"
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
          {task?.status === "completed" ? (
            <span className="muted">Traitée ✓</span>
          ) : null}
          {task?.status === "pending" ? (
            <span className="muted">À démarrer</span>
          ) : null}
        </div>
      </div>,
    );
  }

  return (
    <>
      {error ? (
        <p role="alert" className="error">
          Chargement des tâches impossible : {error}. Nouvelle tentative
          automatique.
        </p>
      ) : null}
      <div
        role="table"
        aria-label="Tâches de la file"
        aria-rowcount={total + 1}
        aria-colcount={4}
      >
        <div role="row" className="table-header">
          <span role="columnheader">ID</span>
          <span role="columnheader">TÂCHE</span>
          <span role="columnheader">STATUT</span>
          <span role="columnheader">ACTIONS</span>
        </div>
        <div
          ref={viewport}
          className="task-viewport"
          tabIndex={0}
          role="rowgroup"
          aria-label="Liste défilante des tâches"
          onScroll={(event) => setScrollTop(event.currentTarget.scrollTop)}
        >
          <div className="virtual-space" style={{ height: total * ROW_HEIGHT }}>
            {rows}
          </div>
          {total === 0 ? (
            <div className="list-empty" role="row">
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
      <div className="list-footer">
        <span>
          {total.toLocaleString("fr-FR")} tâches
          {filter ? " dans ce statut" : ""}
        </span>
        <span>Défilement virtuel · {rows.length} lignes affichées</span>
      </div>
    </>
  );
}
