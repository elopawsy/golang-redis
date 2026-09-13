import { useEffect, useState } from "react";

export async function request(path, options = {}) {
  const response = await fetch(path, {
    ...options,
    headers: { "Content-Type": "application/json" },
  });
  if (!response.ok) {
    const body = await response.json().catch(() => ({}));
    throw new Error(body.error || `Erreur HTTP ${response.status}`);
  }
  return response.json();
}

export function usePolling(path, revision) {
  const [result, setResult] = useState({ path: null, data: null, error: "" });
  useEffect(() => {
    if (!path) return;
    const controller = new AbortController();
    let timer;
    async function load() {
      try {
        const data = await request(path, { signal: controller.signal });
        if (!controller.signal.aborted) setResult({ path, data, error: "" });
      } catch (error) {
        if (!controller.signal.aborted) {
          setResult((previous) => ({
            path,
            data: previous.path === path ? previous.data : null,
            error: error.message,
          }));
        }
      } finally {
        if (!controller.signal.aborted) timer = setTimeout(load, 2000);
      }
    }
    load();
    return () => {
      controller.abort();
      clearTimeout(timer);
    };
  }, [path, revision]);
  return result.path === path ? result : { data: null, error: "" };
}

export const statuses = {
  pending: "En attente",
  running: "En cours",
  completed: "Terminées",
  failed: "En erreur",
};
