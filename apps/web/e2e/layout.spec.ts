import { mkdir } from "node:fs/promises";
import { dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { expect, test } from "@playwright/test";
import type { Page } from "@playwright/test";

async function noPageOverflow(page: Page) {
  const size = await page.evaluate(() => ({
    viewport: document.documentElement.clientWidth,
    page: Math.max(
      document.documentElement.scrollWidth,
      document.body.scrollWidth,
    ),
  }));
  expect(
    size.page,
    `The page should fit the ${size.viewport}px viewport; table scrolling must stay inside its container.`,
  ).toBeLessThanOrEqual(size.viewport + 1);
}
async function openDemo(page: Page) {
  await page.goto("/entrar");
  await expect(page.getByText(/clientes e arquivos fictícios/)).toBeVisible();
  const response = page.waitForResponse(
    (value) =>
      new URL(value.url()).pathname === "/api/auth/demo" &&
      value.request().method() === "POST",
  );
  await page
    .getByRole("button", { name: "Explorar demonstração", exact: true })
    .click();
  const result = await response;
  expect(result.ok()).toBeTruthy();
  expect((await result.json()).organization.demo).toBe(true);
  await expect(page).toHaveURL(/\/app$/);
  await expect(
    page.getByRole("heading", { level: 1, name: /Mariana/ }),
  ).toBeVisible();
}

test("@desktop páginas públicas cabem na tela e formulário informa erros acessíveis", async ({
  page,
}) => {
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  for (const route of [
    "/",
    "/entrar",
    "/cadastro",
    "/esqueci-senha",
    "/redefinir-senha",
  ]) {
    await page.goto(route);
    await expect(page.getByRole("main")).toBeVisible();
    await noPageOverflow(page);
  }
  await page.goto("/cadastro");
  await page
    .getByRole("button", { name: "Criar meu escritório", exact: true })
    .click();
  await expect(
    page.getByText("Informe o nome do escritório.", { exact: true }),
  ).toBeVisible();
  await expect(
    page
      .getByText("Informe um e-mail válido.", { exact: true })
      .or(page.getByText("Informe seu e-mail.", { exact: true })),
  ).toBeVisible();
  await expect(
    page.getByLabel("Nome do escritório", { exact: true }),
  ).toBeFocused();
  expect(errors).toEqual([]);
});

test("@desktop demonstração real, páginas operacionais e modal pelo teclado", async ({
  page,
}) => {
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await openDemo(page);
  await noPageOverflow(page);
  if (process.env.CAPTURE_PREVIEW === "1") {
    // This is a real rendered dashboard. The UI labels the fictional demo data.
    await expect(page.getByText("Demonstração", { exact: true })).toBeVisible();
    await page.evaluate(async () => {
      await document.fonts.ready;
    });
    const target = fileURLToPath(
      new URL("../public/product-preview.png", import.meta.url),
    );
    await mkdir(dirname(target), { recursive: true });
    await page.screenshot({
      path: target,
      fullPage: false,
      animations: "disabled",
    });
  }
  for (const route of [
    "/app/solicitacoes",
    "/app/clientes",
    "/app/modelos",
    "/app/recorrencias",
    "/app/configuracoes",
  ]) {
    await page.goto(route);
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
    await noPageOverflow(page);
  }
  await page.goto("/app/clientes");
  const trigger = page.getByRole("button", {
    name: "Novo cliente",
    exact: true,
  });
  await trigger.focus();
  await page.keyboard.press("Enter");
  const dialog = page.getByRole("dialog", {
    name: "Novo cliente",
    exact: true,
  });
  await expect(dialog).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(dialog).not.toBeVisible();
  await expect(trigger).toBeFocused();
  expect(errors).toEqual([]);
});

test("@mobile público, escritório e portal cabem em 390 e 360 pixels", async ({
  page,
  baseURL,
}) => {
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  for (const route of [
    "/",
    "/entrar",
    "/cadastro",
    "/esqueci-senha",
    "/redefinir-senha",
  ]) {
    await page.goto(route);
    await expect(page.getByRole("main")).toBeVisible();
    await noPageOverflow(page);
  }
  await openDemo(page);
  await noPageOverflow(page);
  await page.getByRole("button", { name: "Abrir menu", exact: true }).click();
  const menu = page.getByRole("navigation", {
    name: "Menu principal",
    exact: true,
  });
  await expect(
    menu.getByRole("link", { name: "Clientes", exact: true }),
  ).toBeInViewport();
  await menu.getByRole("link", { name: "Clientes", exact: true }).click();
  await expect(page).toHaveURL(/\/app\/clientes$/);
  for (const route of [
    "/app/clientes",
    "/app/modelos",
    "/app/recorrencias",
    "/app/configuracoes",
    "/app/solicitacoes/nova",
  ]) {
    await page.goto(route);
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
    await noPageOverflow(page);
  }
  await page.goto("/app/clientes");
  await page.getByRole("button", { name: "Novo cliente", exact: true }).click();
  const dialog = page.getByRole("dialog", {
    name: "Novo cliente",
    exact: true,
  });
  await expect(dialog).toBeVisible();
  const dimensions = await dialog.boundingBox();
  expect(dimensions).not.toBeNull();
  expect(dimensions!.x).toBeGreaterThanOrEqual(0);
  expect(dimensions!.x + dimensions!.width).toBeLessThanOrEqual(
    page.viewportSize()!.width,
  );
  await page.keyboard.press("Escape");
  await expect(dialog).not.toBeVisible();
  const sessionResponse = await page.request.get("/api/session");
  expect(sessionResponse.ok()).toBeTruthy();
  const session = await sessionResponse.json();
  const requestsResponse = await page.request.get("/api/requests");
  expect(requestsResponse.ok()).toBeTruthy();
  const requests = await requestsResponse.json();
  const request = requests.find(
    (value: { status: string }) =>
      value.status !== "completed" && value.status !== "cancelled",
  );
  expect(request).toBeTruthy();
  const shareResponse = await page.request.post(
    `/api/requests/${request.id}/share`,
    { headers: { "X-CSRF-Token": session.csrfToken } },
  );
  expect(shareResponse.ok()).toBeTruthy();
  const share = await shareResponse.json();
  expect(new URL(share.url).origin).toBe(new URL(baseURL!).origin);
  await page.goto(share.url);
  await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
  await expect(
    page.getByText(
      "Este é um ambiente de demonstração. Use apenas arquivos fictícios.",
      { exact: true },
    ),
  ).toBeVisible();
  await noPageOverflow(page);
  const item = page.locator(".portal-document-heading").first();
  await item.click();
  await noPageOverflow(page);
  expect(errors).toEqual([]);
});
