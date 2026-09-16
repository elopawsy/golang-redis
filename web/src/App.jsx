import { useState } from "react";
import { request, statuses, usePolling } from "./api";
import TaskList from "./TaskList";
import {
  alert,
  button,
  dot,
  input,
  primaryButton,
  select,
  statusDot,
} from "./ui";

const count = new Intl.NumberFormat("fr-FR");

export default function App() {
  const [selected, setSelected] = useState("");
  const [revision, setRevision] = useState(0);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const { data: queues, error: connectionError } = usePolling(
    "/api/queues",
    revision,
  );
  const active =
    queues?.find((queue) => queue.name === selected) || queues?.[0];

  async function mutate(path, method, body) {
    setBusy(true);
    setError("");
    setNotice("");
    try {
      const result = await request(path, {
        method,
        body: body ? JSON.stringify(body) : undefined,
      });
      setRevision((value) => value + 1);
      setNotice("Modification enregistrée.");
      return result;
    } catch (error) {
      setError(error.message);
      return null;
    } finally {
      setBusy(false);
    }
  }

  async function createQueue(event) {
    event.preventDefault();
    const form = event.currentTarget;
    const name = new FormData(form).get("name").trim();
    if (await mutate("/api/queues", "POST", { name })) {
      setSelected(name);
      form.reset();
    }
  }

  return (
    <div className="flex min-h-screen max-narrow:block">
      <aside className="flex w-[220px] shrink-0 flex-col border-r border-line px-5 py-6 max-narrow:w-auto max-narrow:gap-3 max-narrow:border-r-0 max-narrow:border-b max-narrow:px-4 max-narrow:py-3">
        <a href="/" className="text-md font-semibold text-ink">
          WasmRedis
        </a>
        <nav
          aria-label="Files d’attente"
          className="mt-6 flex flex-col gap-0.5 max-narrow:mt-0 max-narrow:flex-row max-narrow:overflow-x-auto"
        >
          {queues?.map((queue) => (
            <button
              key={queue.name}
              className={`flex w-full cursor-pointer items-center gap-2 rounded-sm px-2 py-1.5 text-left text-md motion-safe:transition-colors max-narrow:w-auto max-narrow:shrink-0 ${
                active?.name === queue.name
                  ? "bg-active text-ink"
                  : "text-muted hover:bg-hover"
              }`}
              onClick={() => setSelected(queue.name)}
              aria-current={active?.name === queue.name ? "page" : undefined}
            >
              <span className="truncate">{queue.name}</span>
              <span className="ml-auto text-2xs text-faint max-narrow:ml-1">
                {count.format(queue.total)}
              </span>
            </button>
          ))}
        </nav>
        <form className="mt-6 max-narrow:mt-0" onSubmit={createQueue}>
          <label
            htmlFor="queue-name"
            className="mb-1.5 block text-2xs text-faint"
          >
            Nouvelle file
          </label>
          <div className="flex gap-1.5">
            <input
              id="queue-name"
              name="name"
              className={`${input} w-full max-narrow:max-w-[240px]`}
              placeholder="ex. notifications"
              pattern="[A-Za-z0-9_\x2D]+"
              maxLength={64}
              required
              autoComplete="off"
            />
            <button
              className={button}
              disabled={busy}
              aria-label="Créer la file"
            >
              +
            </button>
          </div>
        </form>
      </aside>
      <main className="mx-auto w-full max-w-[1040px] min-w-0 flex-1 px-8 py-8 max-narrow:px-4 max-narrow:py-6">
        {connectionError || error ? (
          <div role="alert" className={alert}>
            {connectionError
              ? `API indisponible : ${connectionError}. Nouvelle tentative automatique.`
              : error}
          </div>
        ) : null}
        <div className="sr-only" role="status">
          {notice}
        </div>
        {!queues && !connectionError ? (
          <p role="status" className="text-md text-muted">
            Chargement des files…
          </p>
        ) : null}
        {active ? (
          <QueueView
            key={active.name}
            queue={active}
            revision={revision}
            busy={busy}
            mutate={mutate}
          />
        ) : queues ? (
          <>
            <h1 className="text-lg font-medium tracking-[-0.4px]">
              Files d’attente
            </h1>
            <p className="mt-3 text-md text-muted">
              Aucune file. Créez-en une dans le menu, puis ajoutez une tâche.
            </p>
          </>
        ) : null}
        <p className="mt-10 text-2xs text-faint">
          Files en mémoire : elles disparaissent au redémarrage du serveur.
        </p>
      </main>
    </div>
  );
}

function QueueView({ queue, revision, busy, mutate }) {
  const [filter, setFilter] = useState("");
  const base = `/api/queues/${encodeURIComponent(queue.name)}`;
  async function addTask(event) {
    event.preventDefault();
    const form = event.currentTarget;
    const payload = new FormData(form).get("payload");
    if (await mutate(`${base}/tasks`, "POST", { payload })) form.reset();
  }
  return (
    <section aria-labelledby="queue-title">
      <div className="flex items-center justify-between gap-4">
        <h1
          id="queue-title"
          className="min-w-0 text-lg font-medium tracking-[-0.4px] [overflow-wrap:anywhere]"
        >
          {queue.name}
        </h1>
        <button
          className={primaryButton}
          disabled={busy || queue.counts.pending === 0}
          onClick={() => mutate(`${base}/claim`, "POST")}
        >
          Démarrer la suivante
        </button>
      </div>
      <div className="mt-3 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted">
        {Object.entries(statuses).map(([status, label]) => (
          <span key={status} className="flex items-center gap-1.5">
            <span className={`${dot} ${statusDot[status]}`} />
            {count.format(queue.counts[status])} {label.toLowerCase()}
          </span>
        ))}
      </div>
      <form className="mt-6 flex gap-2 max-narrow:flex-wrap" onSubmit={addTask}>
        <label className="sr-only" htmlFor="payload">
          Contenu de la tâche
        </label>
        <input
          id="payload"
          name="payload"
          className={`${input} flex-1 max-narrow:basis-full`}
          required
          maxLength={4096}
          placeholder="Nouvelle tâche…"
          autoComplete="off"
        />
        <button className={button} disabled={busy}>
          + Ajouter une tâche
        </button>
      </form>
      <div className="mt-6 flex items-center justify-between gap-3 text-xs text-muted">
        <div className="flex items-center gap-2">
          <label htmlFor="status-filter">Statut</label>
          <select
            id="status-filter"
            className={select}
            value={filter}
            onChange={(event) => setFilter(event.target.value)}
          >
            <option value="">Tous les statuts</option>
            {Object.entries(statuses).map(([status, label]) => (
              <option key={status} value={status}>
                {label}
              </option>
            ))}
          </select>
        </div>
        <span>
          {count.format(filter ? queue.counts[filter] : queue.total)} tâches
        </span>
      </div>
      <TaskList
        key={filter}
        base={base}
        filter={filter}
        revision={revision}
        busy={busy}
        mutate={mutate}
      />
    </section>
  );
}
