import { randomUUID } from "node:crypto";
import { expect, test } from "@playwright/test";
import type {
  APIRequestContext,
  APIResponse,
  Page,
  Response,
} from "@playwright/test";

type Session = {
  csrfToken: string;
  organization: { id: string; demo: boolean };
};
type Client = { id: string; name: string; cnpj: string; active: boolean };
type Template = {
  id: string;
  name: string;
  items: {
    id: string;
    name: string;
    instruction: string;
    required: boolean;
    maxFiles: number;
  }[];
};
type Request = {
  id: string;
  status: string;
  clientName: string;
  items: {
    id: string;
    name: string;
    required: boolean;
    status: string;
    correctionReason?: string;
    files: { id: string; name: string; superseded: boolean }[];
  }[];
};

const password = "Somente-Testes-DocPing-2026!";
const unique = () => randomUUID().slice(0, 8);
const futureDate = () =>
  new Date(Date.now() + 7 * 86_400_000).toISOString().slice(0, 10);
const competence = () => new Date().toISOString().slice(0, 7);
const matchesResponse = (response: Response, path: string, method: string) =>
  new URL(response.url()).pathname === path &&
  response.request().method() === method;
async function json<T>(response: APIResponse | Response): Promise<T> {
  expect(
    response.ok(),
    `API ${response.url()} returned ${response.status()}: ${await response.text()}`,
  ).toBeTruthy();
  return response.json() as Promise<T>;
}
async function createOffice(request: APIRequestContext): Promise<Session> {
  return json<Session>(
    await request.post("/api/auth/register", {
      data: {
        organizationName: `Escritório fictício ${unique()}`,
        name: "Pessoa de Teste",
        email: `docping-e2e-${unique()}@example.invalid`,
        password,
      },
    }),
  );
}
async function createClient(
  request: APIRequestContext,
  session: Session,
): Promise<Client> {
  return json<Client>(
    await request.post("/api/clients", {
      headers: { "X-CSRF-Token": session.csrfToken },
      data: {
        name: `Empresa fictícia ${unique()}`,
        cnpj: "AB123456000199",
        contactName: "Contato de Teste",
        email: "cliente-sintetico@example.invalid",
        phone: "11999990000",
      },
    }),
  );
}
async function createRequest(
  request: APIRequestContext,
  session: Session,
  clientId: string,
  templateId: string,
) {
  const created = await json<Request[]>(
    await request.post("/api/requests", {
      headers: { "X-CSRF-Token": session.csrfToken },
      data: {
        clientIds: [clientId],
        templateId,
        title: "Solicitação fictícia de teste",
        competence: competence(),
        dueDate: futureDate(),
      },
    }),
  );
  expect(created).toHaveLength(1);
  return created[0];
}
function syntheticPDF(revision: number) {
  const content = `BT /F1 14 Tf 50 740 Td (DOCPING E2E - SYNTHETIC DOCUMENT - REVISION ${revision}) Tj 0 -25 Td (No real personal or business data.) Tj ET`;
  const objects = [
    "<< /Type /Catalog /Pages 2 0 R >>",
    "<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
    "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
    "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
    `<< /Length ${Buffer.byteLength(content)} >>\nstream\n${content}\nendstream`,
  ];
  let output = "%PDF-1.4\n";
  const offsets = [0];
  for (const [index, object] of objects.entries()) {
    offsets.push(Buffer.byteLength(output));
    output += `${index + 1} 0 obj\n${object}\nendobj\n`;
  }
  const xref = Buffer.byteLength(output);
  output += "xref\n0 6\n0000000000 65535 f \n";
  for (const offset of offsets.slice(1))
    output += `${String(offset).padStart(10, "0")} 00000 n \n`;
  return Buffer.from(
    `${output}trailer\n<< /Size 6 /Root 1 0 R >>\nstartxref\n${xref}\n%%EOF\n`,
  );
}
const syntheticPNG = Buffer.from(
  "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jf1cAAAAASUVORK5CYII=",
  "base64",
);
async function upload(
  page: Page,
  token: string,
  item: Request["items"][number],
  revision: number,
  image = false,
) {
  const response = page.waitForResponse((value) =>
    matchesResponse(
      value,
      `/api/portal/${token}/items/${item.id}/upload`,
      "POST",
    ),
  );
  await page
    .getByLabel(`Enviar ${item.name}`, { exact: true })
    .setInputFiles({
      name: `documento-ficticio-${revision}.${image ? "png" : "pdf"}`,
      mimeType: image ? "image/png" : "application/pdf",
      buffer: image ? syntheticPNG : syntheticPDF(revision),
    });
  const updated = await json<Request>(await response);
  expect(updated.items.find((value) => value.id === item.id)?.status).toBe(
    "uploaded",
  );
  return updated;
}

test("@desktop escritório → cliente inline → solicitação → envio público → correção → conclusão", async ({
  page,
  browser,
  baseURL,
}) => {
  test.setTimeout(180_000);
  const office = `Escritório fictício ${unique()}`;
  const clientName = `Empresa fictícia ${unique()}`;
  await page.goto("/cadastro");
  await page.getByLabel("Nome do escritório", { exact: true }).fill(office);
  await page.getByLabel("Seu nome", { exact: true }).fill("Pessoa de Teste");
  await page
    .getByLabel("E-mail de trabalho", { exact: true })
    .fill(`docping-fluxo-${unique()}@example.invalid`);
  await page.getByLabel("Crie uma senha", { exact: true }).fill(password);
  const registration = page.waitForResponse((response) =>
    matchesResponse(response, "/api/auth/register", "POST"),
  );
  await page
    .getByRole("button", { name: "Criar meu escritório", exact: true })
    .click();
  const session = await json<Session>(await registration);
  expect(session.organization.demo).toBe(false);
  await expect(page).toHaveURL(/\/app\/solicitacoes\/nova$/);
  await expect(
    page.getByRole("heading", { name: "Nova solicitação", exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Novo cliente", exact: true }).click();
  const clientDialog = page.getByRole("dialog", {
    name: "Novo cliente",
    exact: true,
  });
  await clientDialog
    .getByLabel("Nome ou razão social", { exact: true })
    .fill(clientName);
  await clientDialog
    .getByLabel("CNPJ", { exact: false })
    .fill("AB123456000199");
  await clientDialog
    .getByLabel("Responsável", { exact: true })
    .fill("Contato de Teste");
  await clientDialog
    .getByLabel("E-mail", { exact: true })
    .fill("cliente-ficticio@example.invalid");
  await clientDialog
    .getByLabel("Telefone / WhatsApp", { exact: true })
    .fill("11999990000");
  const clientResponse = page.waitForResponse((response) =>
    matchesResponse(response, "/api/clients", "POST"),
  );
  await clientDialog
    .getByRole("button", { name: "Cadastrar cliente", exact: true })
    .click();
  const client = await json<Client>(await clientResponse);
  expect(client.active).toBe(true);
  await expect(clientDialog).not.toBeVisible();
  await expect(
    page.getByRole("checkbox", { name: new RegExp(clientName) }),
  ).toBeChecked();
  await page.getByRole("button", { name: "Continuar", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "O que você precisa receber?" }),
  ).toBeVisible();
  await page
    .getByRole("button", { name: /Fechamento mensal.*4 documentos/ })
    .click();
  await expect(
    page.getByLabel("Nome do documento 1", { exact: true }),
  ).toHaveValue("Extrato bancário");
  await page.getByRole("button", { name: "Continuar", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Vamos conferir os detalhes?" }),
  ).toBeVisible();
  await page
    .getByLabel("Nome da solicitação", { exact: true })
    .fill("Fechamento fictício E2E");
  await page.getByLabel("Data-limite", { exact: true }).fill(futureDate());
  const requestResponse = page.waitForResponse((response) =>
    matchesResponse(response, "/api/requests", "POST"),
  );
  await page
    .getByRole("button", { name: "Criar solicitação", exact: true })
    .click();
  const requests = await json<Request[]>(await requestResponse);
  expect(requests).toHaveLength(1);
  const request = requests[0];
  const required = request.items.filter((item) => item.required);
  expect(required.length).toBeGreaterThan(0);
  await expect(
    page.getByRole("heading", { name: "Sua solicitação está pronta." }),
  ).toBeVisible();
  const shareResponse = page.waitForResponse((response) =>
    matchesResponse(response, `/api/requests/${request.id}/share`, "POST"),
  );
  await page.getByRole("button", { name: "Copiar link", exact: true }).click();
  const share = await json<{ url: string; token: string }>(await shareResponse);
  expect(new URL(share.url).origin).toBe(new URL(baseURL!).origin);
  await expect(
    page.getByRole("textbox", { name: `Link de ${clientName}`, exact: true }),
  ).toHaveValue(share.url);
  await page
    .getByRole("button", { name: "Acompanhar solicitação", exact: true })
    .click();
  await expect(page).toHaveURL(new RegExp(`/app/solicitacoes/${request.id}$`));

  // A separate browser context proves that the client portal works without login.
  const customer = await browser.newContext({
    baseURL,
    viewport: { width: 390, height: 844 },
    reducedMotion: "reduce",
  });
  try {
    expect((await customer.request.get("/api/session")).status()).toBe(401);
    const portal = await customer.newPage();
    await portal.goto(share.url);
    await expect(
      portal.getByRole("heading", { name: "Vamos organizar seus documentos?" }),
    ).toBeVisible();
    for (const [index, item] of required.entries())
      await upload(portal, share.token, item, index + 1, index === 0);
    await expect(
      portal.getByRole("heading", { name: "Seus documentos chegaram." }),
    ).toBeVisible();
    await portal.reload();
    await expect(
      portal.getByRole("heading", { name: "Seus documentos chegaram." }),
    ).toBeVisible();
    await page.reload();
    await page
      .locator(".review-document-row")
      .filter({ hasText: required[0].name })
      .click();
    await page
      .getByRole("button", { name: "Solicitar correção", exact: true })
      .click();
    const correction = page.getByRole("dialog", {
      name: "O que precisa ser corrigido?",
    });
    const reason =
      "Arquivo sintético de teste: envie a revisão completa do documento.";
    await correction
      .getByLabel("Motivo da correção", { exact: true })
      .fill(reason);
    const correctedResponse = page.waitForResponse((response) =>
      matchesResponse(
        response,
        `/api/requests/${request.id}/items/${required[0].id}/review`,
        "POST",
      ),
    );
    await correction
      .getByRole("button", { name: "Solicitar correção", exact: true })
      .click();
    const corrected = await json<Request>(await correctedResponse);
    expect(
      corrected.items.find((item) => item.id === required[0].id)?.status,
    ).toBe("correction_required");
    await expect(correction).not.toBeVisible();
    await portal.reload();
    await expect(portal.getByText(reason, { exact: true })).toBeVisible();
    const resent = await upload(portal, share.token, required[0], 99);
    const replaced = resent.items.find((item) => item.id === required[0].id)!;
    expect(replaced.files.filter((file) => file.superseded)).toHaveLength(1);
    expect(replaced.files.filter((file) => !file.superseded)).toHaveLength(1);
    await page.reload();
    for (const item of required) {
      await page
        .locator(".review-document-row")
        .filter({ hasText: item.name })
        .click();
      const approval = page.waitForResponse((response) =>
        matchesResponse(
          response,
          `/api/requests/${request.id}/items/${item.id}/review`,
          "POST",
        ),
      );
      await page
        .getByRole("button", { name: "Aprovar documento", exact: true })
        .click();
      const approved = await json<Request>(await approval);
      expect(approved.items.find((value) => value.id === item.id)?.status).toBe(
        "approved",
      );
    }
    await expect(
      page.getByText("Esta solicitação está concluída.", { exact: false }),
    ).toBeVisible();
    const final = await json<Request>(
      await page.request.get(`/api/requests/${request.id}`),
    );
    expect(final.status).toBe("completed");
    await portal.reload();
    await expect(
      portal.getByRole("heading", {
        name: "Tudo certo com os seus documentos.",
      }),
    ).toBeVisible();
    await expect(portal.locator('input[type="file"]')).toHaveCount(0);
  } finally {
    await customer.close();
  }
});

test("@desktop edição de modelo preserva solicitações anteriores e busca aceita CNPJ formatado", async ({
  page,
}) => {
  const session = await createOffice(page.request);
  const client = await createClient(page.request, session);
  const modelName = `Modelo fictício ${unique()}`;
  await page.goto("/app/modelos");
  await page.getByRole("button", { name: "Novo modelo", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "Novo modelo", exact: true });
  await dialog.getByLabel("Nome do modelo", { exact: true }).fill(modelName);
  await dialog
    .getByLabel("Nome do documento 1", { exact: true })
    .fill("Extrato original de teste");
  await dialog
    .getByRole("button", { name: "Adicionar documento", exact: true })
    .click();
  await dialog
    .getByLabel("Nome do documento 2", { exact: true })
    .fill("Comprovante original de teste");
  const createdResponse = page.waitForResponse((response) =>
    matchesResponse(response, "/api/templates", "POST"),
  );
  await dialog
    .getByRole("button", { name: "Criar modelo", exact: true })
    .click();
  const model = await json<Template>(await createdResponse);
  const original = await createRequest(
    page.request,
    session,
    client.id,
    model.id,
  );
  await expect(dialog).not.toBeVisible();
  await page
    .getByRole("article")
    .filter({
      has: page.getByRole("heading", { name: modelName, exact: true }),
    })
    .getByRole("button", { name: "Editar modelo", exact: true })
    .click();
  const editor = page.getByRole("dialog", {
    name: "Editar modelo",
    exact: true,
  });
  await editor
    .getByLabel("Nome do documento 1", { exact: true })
    .fill("Extrato alterado de teste");
  await editor
    .getByRole("button", { name: "Remover documento 2", exact: true })
    .click();
  const savedResponse = page.waitForResponse((response) =>
    matchesResponse(response, `/api/templates/${model.id}`, "PUT"),
  );
  await editor
    .getByRole("button", { name: "Salvar modelo", exact: true })
    .click();
  await json<Template>(await savedResponse);
  const previous = await json<Request>(
    await page.request.get(`/api/requests/${original.id}`),
  );
  expect(previous.items.map((item) => item.name)).toEqual([
    "Extrato original de teste",
    "Comprovante original de teste",
  ]);
  const next = await createRequest(page.request, session, client.id, model.id);
  expect(next.items.map((item) => item.name)).toEqual([
    "Extrato alterado de teste",
  ]);
  await page.goto(`/app/solicitacoes/${original.id}`);
  await expect(
    page.getByRole("heading", {
      name: "Extrato original de teste",
      exact: true,
    }),
  ).toBeVisible();
  await expect(
    page
      .locator(".review-document-row")
      .filter({ hasText: "Comprovante original de teste" }),
  ).toBeVisible();
  await page.goto("/app/clientes");
  await page
    .getByRole("textbox", { name: "Buscar clientes", exact: true })
    .fill("AB.123.456/0001-99");
  await expect(
    page.getByRole("row").filter({ hasText: client.name }),
  ).toBeVisible();
  await page
    .getByRole("textbox", { name: "Buscar clientes", exact: true })
    .fill("não existe este cliente");
  await expect(
    page.getByRole("heading", {
      name: "Nenhum cliente encontrado",
      exact: true,
    }),
  ).toBeVisible();
});

test("@desktop recorrência persiste configuração, pausa e reativação", async ({
  page,
}) => {
  const session = await createOffice(page.request);
  const client = await createClient(page.request, session);
  const name = `Rotina fictícia ${unique()}`;
  await page.goto("/app/recorrencias");
  await page
    .getByRole("button", { name: "Nova recorrência", exact: true })
    .click();
  const dialog = page.getByRole("dialog", {
    name: "Nova recorrência mensal",
    exact: true,
  });
  await dialog.getByLabel("Nome da recorrência", { exact: true }).fill(name);
  await dialog.getByLabel("Criar todo dia", { exact: true }).selectOption("31");
  await dialog.getByLabel("Prazo para envio", { exact: false }).fill("7");
  await dialog.getByRole("checkbox", { name: new RegExp(client.name) }).check();
  const created = page.waitForResponse((response) =>
    matchesResponse(response, "/api/schedules", "POST"),
  );
  await dialog
    .getByRole("button", { name: "Criar recorrência", exact: true })
    .click();
  const schedule = await json<{
    id: string;
    active: boolean;
    dayOfMonth: number;
    dueDays: number;
    clientIds: string[];
  }>(await created);
  expect(schedule).toMatchObject({
    active: true,
    dayOfMonth: 31,
    dueDays: 7,
    clientIds: [client.id],
  });
  await expect(dialog).not.toBeVisible();
  const row = page
    .getByRole("article")
    .filter({ has: page.getByRole("heading", { name, exact: true }) });
  const paused = page.waitForResponse((response) =>
    matchesResponse(response, `/api/schedules/${schedule.id}`, "PUT"),
  );
  await row.getByRole("button", { name: "Pausar", exact: true }).click();
  expect((await json<{ active: boolean }>(await paused)).active).toBe(false);
  await expect(row.getByText("Pausada", { exact: true })).toBeVisible();
  await page.reload();
  const resumed = page.waitForResponse((response) =>
    matchesResponse(response, `/api/schedules/${schedule.id}`, "PUT"),
  );
  await row.getByRole("button", { name: "Reativar", exact: true }).click();
  expect((await json<{ active: boolean }>(await resumed)).active).toBe(true);
  await expect(row.getByText("Ativa", { exact: true })).toBeVisible();
  await row
    .getByRole("button", { name: `Histórico de ${name}`, exact: true })
    .click();
  await expect(
    page.getByRole("dialog", { name: "Histórico de execução", exact: true }),
  ).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog")).not.toBeVisible();
});
