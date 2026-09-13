import { useState } from "react";
import { request, statuses, usePolling } from "./api";
import TaskList from "./TaskList";

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
  const total = queues?.reduce((sum, queue) => sum + queue.total, 0) || 0;

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
    <div className="app">
      <aside className="sidebar">
        <a className="brand" href="/" aria-label="WasmRedis, accueil">
          <span className="brand-icon">W</span> WasmRedis
          <span className="version">GO</span>
        </a>
        <div className="workspace">
          <span className="workspace-icon">L</span>
          <div>
            Mon espace local<small>Moteur en mémoire</small>
          </div>
        </div>
        <div className="section-label">
          EXPLORATEUR <span>{queues?.length || 0}</span>
        </div>
        <h2 className="sidebar-title">Files d’attente</h2>
        <nav aria-label="Files d’attente">
          {queues?.map((queue) => (
            <button
              key={queue.name}
              className={`queue-link ${active?.name === queue.name ? "active" : ""}`}
              onClick={() => setSelected(queue.name)}
              aria-current={active?.name === queue.name ? "page" : undefined}
            >
              <span className="queue-icon" aria-hidden="true">
                ≡
              </span>
              <span>{queue.name}</span>
              <span className="queue-count">{count.format(queue.total)}</span>
            </button>
          ))}
          {queues?.length === 0 ? (
            <p className="muted">Aucune file pour le moment.</p>
          ) : null}
        </nav>
        <form className="queue-form" onSubmit={createQueue}>
          <label htmlFor="queue-name">Nouvelle file</label>
          <div className="input-row">
            <input
              id="queue-name"
              name="name"
              placeholder="ex. notifications"
              pattern="[A-Za-z0-9_\x2D]+"
              maxLength={64}
              required
              autoComplete="off"
            />
            <button disabled={busy} aria-label="Créer la file">
              +
            </button>
          </div>
        </form>
        <div className="sidebar-bottom">
          <span className={`dot ${connectionError ? "offline" : ""}`} />
          {connectionError
            ? "Connexion interrompue"
            : queues
              ? "API connectée"
              : "Connexion…"}
          <small>Actualisation toutes les 2 secondes</small>
        </div>
      </aside>
      <main>
        <header className="topbar">
          <span>
            Espace local <span className="breadcrumb">/</span> Files d’attente
          </span>
          <span className="memory-tag">En mémoire</span>
        </header>
        <div className="content">
          <div className="page-heading">
            <div>
              <p className="eyebrow">TABLEAU DE BORD</p>
              <h1>Files d’attente</h1>
              <p className="muted">
                Suivez vos tâches, de leur création à leur résultat.
              </p>
            </div>
            <span className="total-tag">
              {count.format(total)} tâches au total
            </span>
          </div>
          <div className="memory-note">
            <span aria-hidden="true">ⓘ</span> Les données sont temporaires :
            elles disparaissent au redémarrage du serveur.
          </div>
          {connectionError || error ? (
            <div role="alert" className="error">
              {connectionError
                ? `API indisponible : ${connectionError}. Nouvelle tentative automatique.`
                : error}
            </div>
          ) : null}
          <div className="sr-only" role="status">
            {notice}
          </div>
          {!queues && !connectionError ? (
            <p role="status">Chargement des files…</p>
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
            <section className="empty-state">
              <span className="empty-icon">≡</span>
              <h2>Votre première file vous attend</h2>
              <p>Créez une file dans le menu, puis ajoutez une tâche.</p>
            </section>
          ) : null}
          <footer>
            WasmRedis <span>Go + React · Projet d’apprentissage</span>
          </footer>
        </div>
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
    <>
      <div className="stats">
        {Object.entries(statuses).map(([status, label]) => (
          <button
            key={status}
            className={`stat-card ${status} ${filter === status ? "chosen" : ""}`}
            onClick={() => setFilter(filter === status ? "" : status)}
            aria-pressed={filter === status}
          >
            <span className="stat-label">
              <span className="dot" />
              {label}
            </span>
            <strong>{count.format(queue.counts[status])}</strong>
            <span className="stat-hint">
              Afficher les tâches <span aria-hidden="true">↗</span>
            </span>
          </button>
        ))}
      </div>
      <section className="task-panel" aria-labelledby="queue-title">
        <div className="panel-heading">
          <div className="queue-title">
            <span className="panel-icon" aria-hidden="true">
              ≡
            </span>
            <div>
              <h2 id="queue-title">{queue.name}</h2>
              <p>{count.format(queue.total)} tâches · ordre d’arrivée</p>
            </div>
          </div>
          <button
            className="primary"
            disabled={busy || queue.counts.pending === 0}
            onClick={() => mutate(`${base}/claim`, "POST")}
          >
            ▶ Démarrer la suivante
          </button>
        </div>
        <form className="task-form" onSubmit={addTask}>
          <label className="sr-only" htmlFor="payload">
            Contenu de la tâche
          </label>
          <input
            id="payload"
            name="payload"
            required
            maxLength={4096}
            placeholder="Décrivez une nouvelle tâche…"
            autoComplete="off"
          />
          <button className="secondary" disabled={busy}>
            + Ajouter une tâche
          </button>
        </form>
        <div className="list-toolbar">
          <div>
            <label htmlFor="status-filter">Statut</label>
            <select
              id="status-filter"
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
          <span>Traitement manuel</span>
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
      <p className="help-text">
        « Démarrer la suivante » prend la première tâche en attente. Une tâche
        en erreur peut être remise en attente. Aucun programme n’exécute
        automatiquement son contenu.
      </p>
    </>
  );
}
