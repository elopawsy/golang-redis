import { test, expect } from "@playwright/test";

test("créer une file et suivre le cycle complet d’une tâche", async ({
  page,
}) => {
  await page.goto("/");
  await page.getByLabel("Nouvelle file").fill("test-cycle");
  await page.getByRole("button", { name: "Créer la file" }).click();
  await expect(page.getByRole("heading", { name: "test-cycle" })).toBeVisible();
  await page
    .getByLabel("Contenu de la tâche")
    .fill("Envoyer le message de bienvenue");
  await page
    .getByRole("button", { name: "+ Ajouter une tâche", exact: true })
    .click();
  await expect(
    page.getByText("Envoyer le message de bienvenue", { exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Démarrer la suivante" }).click();
  await page
    .getByRole("button", { name: /Marquer la tâche .* en erreur/ })
    .click();
  await expect(page.locator(".badge.failed")).toHaveText("En erreur");
  await page.getByRole("button", { name: /Réessayer la tâche/ }).click();
  await expect(page.locator(".badge.pending")).toHaveText("En attente");
  await page.getByRole("button", { name: "Démarrer la suivante" }).click();
  await page.getByRole("button", { name: /Terminer la tâche/ }).click();
  await expect(page.locator(".badge.completed")).toHaveText("Terminées");
  await page.getByLabel("Statut", { exact: true }).selectOption("pending");
  await expect(page.getByText("Aucune tâche pour ce statut.")).toBeVisible();
});

test("virtualiser mille tâches et charger la fin de la file", async ({
  page,
}) => {
  const calls = [];
  page.on("request", (request) => {
    if (request.url().includes("/tasks?")) calls.push(request.url());
  });
  await page.goto("/");
  await page.getByRole("button", { name: /emails 1/ }).click();
  await expect(
    page.getByText("Tâche emails 0001", { exact: true }),
  ).toBeVisible();
  expect(await page.locator(".task-row").count()).toBeLessThan(30);
  await page.locator(".task-viewport").evaluate((element) => {
    element.scrollTop = element.scrollHeight;
  });
  await expect(
    page.getByText("Tâche emails 1000", { exact: true }),
  ).toBeVisible();
  expect(await page.locator(".task-row").count()).toBeLessThan(30);
  expect(
    calls.some((url) => new URL(url).searchParams.get("offset") === "900"),
  ).toBeTruthy();
  expect(
    calls.every((url) => Number(new URL(url).searchParams.get("limit")) <= 200),
  ).toBeTruthy();
  await page.getByRole("button", { name: /images 1/ }).click();
  await expect(
    page.getByText("Tâche images 0001", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByText("Tâche emails 1000", { exact: true }),
  ).toHaveCount(0);
});

test("afficher une erreur réseau puis récupérer automatiquement", async ({
  page,
}) => {
  await page.route("**/api/queues", (route) => route.abort());
  await page.goto("/");
  await expect(page.getByRole("alert")).toContainText("API indisponible");
  await page.unroute("**/api/queues");
  await expect(
    page.getByRole("heading", { name: "emails", exact: true }),
  ).toBeVisible({ timeout: 10000 });
  await expect(page.getByRole("alert")).toHaveCount(0);
});

test("interface utilisable sur mobile", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/");
  await expect(
    page.getByRole("heading", { name: "emails", exact: true }),
  ).toBeVisible();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBeTruthy();
  await page.getByLabel("Statut", { exact: true }).selectOption("failed");
  await expect(page.locator(".badge.failed").first()).toBeVisible();
});
