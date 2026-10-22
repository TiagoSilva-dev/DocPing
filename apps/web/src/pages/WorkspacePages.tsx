import { useEffect, useRef, useState } from "react";
import type { FormEvent } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useSearchParams } from "react-router-dom";
import {
  Archive,
  ArrowDown,
  ArrowRight,
  ArrowUp,
  CalendarDays,
  Check,
  ChevronRight,
  Download,
  FileText,
  History,
  Mail,
  Pause,
  Pencil,
  Play,
  Plus,
  RotateCcw,
  Search,
  Trash2,
  Upload,
  Users,
  X,
} from "lucide-react";
import { api, openBillingPortal, post, put } from "@/lib/api";
import { planName, subscriptionLabel } from "@/lib/plans";
import { usePageMeta } from "@/lib/seo";
import type {
  Client,
  Organization,
  Schedule,
  Template,
  TemplateItem,
} from "@/lib/types";
import { dateLabel, shortMonth } from "@/lib/format";
import { useSession } from "@/lib/session";
import { useToast } from "@/lib/toast";
import {
  Avatar,
  Button,
  EmptyState,
  Field,
  Modal,
  PageHeading,
} from "@/components/shared";
import "./workspace-pages.css";

const message = (error: unknown) =>
  error instanceof Error
    ? error.message
    : "Não foi possível salvar. Tente novamente.";
const clean = (value: string) => value.trim();
const normalizeCNPJ = (value: string) =>
  value.replace(/[.\/\-\s]/g, "").toUpperCase();
const cnpjLabel = (value: string) =>
  value.replace(
    /^([A-Z\d]{2})([A-Z\d]{3})([A-Z\d]{3})([A-Z\d]{4})(\d{2})$/,
    "$1.$2.$3/$4-$5",
  );

function LoadError({ error, retry }: { error: unknown; retry: () => void }) {
  return (
    <div className="wp-load-error" role="alert">
      <p>{message(error)}</p>
      <Button variant="secondary" onClick={retry}>
        Tentar novamente
      </Button>
    </div>
  );
}
function LoadingRows() {
  return (
    <div className="wp-loading" role="status" aria-label="Carregando">
      <span />
      <span />
      <span />
      <span />
    </div>
  );
}
type ClientDraft = Pick<
  Client,
  "name" | "cnpj" | "contactName" | "email" | "phone"
>;
const emptyClient = (): ClientDraft => ({
  name: "",
  cnpj: "",
  contactName: "",
  email: "",
  phone: "",
});
const clientFields = (client: ClientDraft): ClientDraft => ({
  name: client.name,
  cnpj: client.cnpj,
  contactName: client.contactName,
  email: client.email,
  phone: client.phone,
});

function validateClient(client: ClientDraft) {
  const errors: string[] = [];
  if (!client.name.trim()) errors.push("Informe o nome da empresa.");
  if (client.name.length > 180)
    errors.push("O nome deve ter até 180 caracteres.");
  if (client.email && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(client.email))
    errors.push("Informe um e-mail válido.");
  if (client.cnpj && !/^[A-Z\d]{12}\d{2}$/.test(normalizeCNPJ(client.cnpj)))
    errors.push(
      "O CNPJ deve ter 14 caracteres, com os dois últimos numéricos.",
    );
  if (client.phone && client.phone.replace(/\D/g, "").length < 10)
    errors.push("Informe o telefone com DDD.");
  return errors;
}

export function ClientEditor({
  client,
  onClose,
  onSaved,
}: {
  client: Client | null;
  onClose: () => void;
  onSaved?: (client: Client) => void;
}) {
  const [draft, setDraft] = useState<ClientDraft>(
    client ? clientFields(client) : emptyClient,
  );
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const queryClient = useQueryClient();
  const toast = useToast();
  const change = (key: keyof ClientDraft, value: string) =>
    setDraft((prev) => ({ ...prev, [key]: value }));
  async function submit(event: FormEvent) {
    event.preventDefault();
    const body = Object.fromEntries(
      Object.entries(draft).map(([key, value]) => [key, clean(value)]),
    ) as ClientDraft;
    const invalid = validateClient(body);
    if (invalid.length) return setError(invalid.join(" "));
    body.cnpj = normalizeCNPJ(body.cnpj);
    setSaving(true);
    setError("");
    try {
      const saved = client
        ? await put<Client>(`/clients/${client.id}`, {
            ...body,
            active: client.active,
          })
        : await post<Client>("/clients", body);
      await queryClient.invalidateQueries({ queryKey: ["clients"] });
      toast(client ? "Dados do cliente atualizados." : "Cliente cadastrado.");
      onSaved?.(saved);
      onClose();
    } catch (err) {
      setError(message(err));
    } finally {
      setSaving(false);
    }
  }
  return (
    <Modal
      title={client ? "Editar cliente" : "Novo cliente"}
      description="Só o essencial para começar a pedir documentos."
      onClose={onClose}
    >
      <form onSubmit={submit} className="wp-form">
        <Field label="Nome ou razão social">
          <input
            autoFocus
            className="input"
            required
            maxLength={180}
            value={draft.name}
            onChange={(e) => change("name", e.target.value)}
            placeholder="Ex.: Horizonte Arquitetura"
          />
        </Field>
        <Field
          label="CNPJ"
          hint="Opcional para pessoa física ou empresa em abertura."
        >
          <input
            className="input"
            value={draft.cnpj}
            onChange={(e) => change("cnpj", e.target.value)}
            placeholder="00.000.000/0000-00"
            maxLength={18}
            autoCapitalize="characters"
          />
        </Field>
        <Field label="Responsável">
          <input
            className="input"
            maxLength={120}
            value={draft.contactName}
            onChange={(e) => change("contactName", e.target.value)}
            placeholder="Nome de quem envia os documentos"
          />
        </Field>
        <Field label="E-mail">
          <input
            className="input"
            type="email"
            maxLength={254}
            value={draft.email}
            onChange={(e) => change("email", e.target.value)}
            placeholder="contato@empresa.com.br"
          />
        </Field>
        <Field label="Telefone / WhatsApp">
          <input
            className="input"
            type="tel"
            maxLength={25}
            value={draft.phone}
            onChange={(e) => change("phone", e.target.value)}
            placeholder="(11) 99999-9999"
          />
        </Field>
        {error && (
          <p role="alert" className="inline-error">
            {error}
          </p>
        )}
        <div className="form-actions">
          <Button variant="secondary" type="button" onClick={onClose}>
            Cancelar
          </Button>
          <Button loading={saving} type="submit">
            {client ? "Salvar alterações" : "Cadastrar cliente"}
          </Button>
        </div>
      </form>
    </Modal>
  );
}

// A small RFC 4180 parser: escaped quotes, quoted separators and line breaks are preserved.
export function parseClientCSV(text: string): string[][] {
  const source = text.replace(/^\uFEFF/, "");
  let quoted = false;
  let firstLine = "";
  for (const char of source) {
    if (char === '"') quoted = !quoted;
    if ((char === "\n" || char === "\r") && !quoted) break;
    firstLine += char;
  }
  const separator =
    (firstLine.match(/;/g) ?? []).length > (firstLine.match(/,/g) ?? []).length
      ? ";"
      : ",";
  const rows: string[][] = [];
  let row: string[] = [];
  let field = "";
  quoted = false;
  let closed = false;
  for (let index = 0; index < source.length; index++) {
    const char = source[index];
    if (quoted) {
      if (char === '"' && source[index + 1] === '"') {
        field += '"';
        index++;
      } else if (char === '"') {
        quoted = false;
        closed = true;
      } else field += char;
    } else if (char === '"') {
      if (field.trim() || closed)
        throw new Error(
          "Há aspas fora de posição no CSV. Verifique o arquivo.",
        );
      field = "";
      quoted = true;
    } else if (char === separator || char === "\n" || char === "\r") {
      row.push(field.trim());
      field = "";
      closed = false;
      if (char !== separator) {
        if (row.some(Boolean)) rows.push(row);
        row = [];
        if (char === "\r" && source[index + 1] === "\n") index++;
      }
    } else {
      if (closed && char.trim())
        throw new Error("Há texto após o fechamento de aspas no CSV.");
      field += char;
    }
  }
  if (quoted) throw new Error("Há um campo com aspas não fechadas no CSV.");
  row.push(field.trim());
  if (row.some(Boolean)) rows.push(row);
  return rows;
}

type ImportRow = {
  line: number;
  client: ClientDraft;
  errors: string[];
  imported?: boolean;
};
type ImportResponse = {
  clients: Client[];
  errors: { row: number; message: string }[];
  imported: number;
};
const headerKey = (value: string) =>
  value
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .toLowerCase()
    .replace(/[^a-z0-9]/g, "");
function previewClients(text: string, existing: Client[]): ImportRow[] {
  const [headers, ...data] = parseClientCSV(text);
  if (!headers || !data.length)
    throw new Error(
      "O arquivo precisa ter um cabeçalho e pelo menos um cliente.",
    );
  if (data.length > 1000)
    throw new Error("Importe até 1.000 clientes por arquivo.");
  const names: Record<keyof ClientDraft, string[]> = {
    name: ["nome", "name", "empresa"],
    cnpj: ["cnpj"],
    contactName: ["responsavel", "contactname"],
    email: ["email"],
    phone: ["telefone", "phone", "whatsapp"],
  };
  const keys = headers.map(headerKey);
  const indexes = Object.fromEntries(
    Object.entries(names).map(([key, aliases]) => [
      key,
      keys.findIndex((header) => aliases.includes(header)),
    ]),
  ) as Record<keyof ClientDraft, number>;
  if (indexes.name < 0)
    throw new Error(
      "Não encontramos a coluna “nome”. Use o modelo de CSV abaixo.",
    );
  const seen = new Set(
    existing
      .filter((client) => client.cnpj)
      .map((client) => normalizeCNPJ(client.cnpj)),
  );
  return data.map((fields, index) => {
    const client = Object.fromEntries(
      Object.keys(names).map((key) => [
        key,
        fields[indexes[key as keyof ClientDraft]] ?? "",
      ]),
    ) as ClientDraft;
    const errors = validateClient(client);
    client.cnpj = normalizeCNPJ(client.cnpj);
    if (fields.length !== headers.length)
      errors.push("A quantidade de colunas não corresponde ao cabeçalho.");
    if (client.cnpj && seen.has(client.cnpj))
      errors.push("CNPJ duplicado no arquivo ou já cadastrado.");
    if (client.cnpj) seen.add(client.cnpj);
    return { line: index + 2, client, errors };
  });
}
function serializeClients(rows: ImportRow[]) {
  const keys: (keyof ClientDraft)[] = [
    "name",
    "cnpj",
    "contactName",
    "email",
    "phone",
  ];
  const quote = (value: string) => `"${value.replace(/"/g, '""')}"`;
  return [
    keys.join(","),
    ...rows.map((row) => keys.map((key) => quote(row.client[key])).join(",")),
  ].join("\r\n");
}
function downloadCSVExample() {
  const blob = new Blob(
    [
      "\uFEFFnome;cnpj;responsavel;email;telefone\nEmpresa Exemplo;;Ana;ana@exemplo.com.br;11999999999\n",
    ],
    { type: "text/csv;charset=utf-8;" },
  );
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = "docping-modelo-clientes.csv";
  anchor.click();
  URL.revokeObjectURL(url);
}

function ClientImport({
  existing,
  onClose,
}: {
  existing: Client[];
  onClose: () => void;
}) {
  const [rows, setRows] = useState<ImportRow[]>([]);
  const [filename, setFilename] = useState("");
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  const [checking, setChecking] = useState(false);
  const [csv, setCSV] = useState("");
  const [validated, setValidated] = useState(false);
  const fileRef = useRef<HTMLInputElement>(null);
  const queryClient = useQueryClient();
  const toast = useToast();
  const valid = rows.filter((row) => !row.errors.length && !row.imported);
  const invalid = rows.filter((row) => row.errors.length);
  async function readFile(file?: File) {
    if (!file) return;
    setError("");
    setRows([]);
    setCSV("");
    setValidated(false);
    setFilename(file.name);
    if (file.size > 512 * 1024)
      return setError("O arquivo deve ter no máximo 512 KB.");
    setChecking(true);
    try {
      const text = await file.text();
      const preview = previewClients(text, existing);
      const normalizedCSV = serializeClients(preview);
      setRows(preview);
      if (!preview.some((row) => row.errors.length)) {
        const result = await post<ImportResponse>("/clients/import", {
          csv: normalizedCSV,
          commit: false,
        });
        setRows(
          preview.map((row) => ({
            ...row,
            errors: result.errors
              .filter((err) => err.row === row.line)
              .map((err) => err.message),
          })),
        );
        setValidated(result.errors.length === 0);
        if (
          result.errors.some(
            (err) => !preview.some((row) => row.line === err.row),
          )
        )
          setError(
            result.errors
              .map((err) => `Linha ${err.row}: ${err.message}`)
              .join(" "),
          );
      }
      setCSV(normalizedCSV);
    } catch (err) {
      setError(message(err));
    } finally {
      setChecking(false);
    }
  }
  async function importRows() {
    setSaving(true);
    setError("");
    try {
      const result = await post<ImportResponse>("/clients/import", {
        csv,
        commit: true,
      });
      if (result.errors.length) {
        setRows((prev) =>
          prev.map((row) => ({
            ...row,
            errors: result.errors
              .filter((err) => err.row === row.line)
              .map((err) => err.message),
          })),
        );
        setValidated(false);
        setError("Revise os erros apontados antes de importar novamente.");
      } else {
        setRows((prev) => prev.map((row) => ({ ...row, imported: true })));
        await queryClient.invalidateQueries({ queryKey: ["clients"] });
        toast(
          `${result.imported} ${result.imported === 1 ? "cliente importado" : "clientes importados"}.`,
        );
      }
    } catch (err) {
      setError(message(err));
    } finally {
      setSaving(false);
    }
  }
  return (
    <Modal
      title="Importar clientes"
      description="Confira os dados antes de adicionar à sua carteira."
      onClose={saving ? () => undefined : onClose}
      wide
    >
      <div className="wp-import">
        <div className="wp-import-instructions">
          <div>
            <strong>Um CSV. Todos os seus clientes.</strong>
            <p>
              Colunas: nome, cnpj, responsavel, email e telefone. Somente o nome
              é obrigatório. Aceitamos vírgula ou ponto e vírgula.
            </p>
          </div>
          <Button variant="secondary" onClick={downloadCSVExample}>
            <Download size={15} />
            Baixar modelo
          </Button>
        </div>
        <input
          ref={fileRef}
          className="wp-hidden-input"
          type="file"
          accept=".csv,text/csv"
          aria-label="Selecionar arquivo CSV"
          disabled={saving || checking}
          onChange={(e) => {
            void readFile(e.target.files?.[0]);
            e.target.value = "";
          }}
        />
        <button
          className="wp-file-picker"
          type="button"
          disabled={saving || checking}
          onClick={() => fileRef.current?.click()}
        >
          <Upload size={22} />
          <strong>
            {checking
              ? "Validando arquivo…"
              : filename || "Escolher arquivo CSV"}
          </strong>
          <span>Até 1.000 linhas · máximo de 512 KB</span>
        </button>
        {error && (
          <p className="inline-error" role="alert">
            {error}
          </p>
        )}
        {!!rows.length && (
          <>
            <div className="wp-import-summary">
              <strong>{rows.length} clientes no arquivo</strong>
              <span>
                {valid.length} prontos para importar
                {invalid.length > 0 && ` · ${invalid.length} com erros`}
              </span>
            </div>
            <div className="wp-import-table">
              <table className="wp-table">
                <thead>
                  <tr>
                    <th>Linha</th>
                    <th>Cliente</th>
                    <th>E-mail</th>
                    <th>Validação</th>
                  </tr>
                </thead>
                <tbody>
                  {rows.map((row) => (
                    <tr key={row.line}>
                      <td className="mono">{row.line}</td>
                      <td>
                        {row.client.name || "Sem nome"}
                        <span className="wp-subline mono">
                          {cnpjLabel(row.client.cnpj)}
                        </span>
                      </td>
                      <td>{row.client.email || "—"}</td>
                      <td
                        className={
                          row.errors.length ? "wp-row-error" : "wp-row-success"
                        }
                      >
                        {row.imported
                          ? "Importado"
                          : row.errors.length
                            ? row.errors.join(" ")
                            : "Pronto para importar"}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            {!!invalid.length && (
              <p className="small muted">
                Corrija as linhas com erro e selecione o arquivo novamente. A
                importação só será concluída quando todos os clientes estiverem
                válidos.
              </p>
            )}
          </>
        )}
        <div className="form-actions">
          <Button variant="secondary" disabled={saving} onClick={onClose}>
            Fechar
          </Button>
          <Button
            disabled={
              !valid.length || !!invalid.length || checking || !validated
            }
            loading={saving}
            onClick={() => void importRows()}
          >
            {saving
              ? "Importando clientes…"
              : `Importar ${valid.length || ""} ${valid.length === 1 ? "cliente" : "clientes"}`}
          </Button>
        </div>
      </div>
    </Modal>
  );
}

export function ClientsPage() {
  usePageMeta({ title: "Clientes — DocPing" });
  const query = useQuery({
    queryKey: ["clients"],
    queryFn: () => api<Client[]>("/clients"),
  });
  const [searchParams] = useSearchParams();
  const searchParam = searchParams.get("busca");
  const [search, setSearch] = useState(searchParam ?? "");
  const [filter, setFilter] = useState<"active" | "archived" | "all">(
    searchParam ? "all" : "active",
  );
  useEffect(() => {
    if (searchParam !== null) {
      setSearch(searchParam);
      setFilter("all");
    }
  }, [searchParam]);
  const [editor, setEditor] = useState<Client | "new" | null>(null);
  const [importing, setImporting] = useState(false);
  const [archiving, setArchiving] = useState<Client | null>(null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const queryClient = useQueryClient();
  const toast = useToast();
  const clients = query.data ?? [];
  const filtered = clients.filter(
    (client) =>
      (filter === "all" || client.active === (filter === "active")) &&
      `${client.name} ${client.cnpj} ${cnpjLabel(client.cnpj)} ${client.email} ${client.contactName}`
        .toLowerCase()
        .includes(search.toLowerCase()),
  );
  async function toggleArchive(client: Client) {
    setSaving(true);
    setError("");
    try {
      await put(`/clients/${client.id}`, {
        ...clientFields(client),
        active: !client.active,
      });
      await queryClient.invalidateQueries({ queryKey: ["clients"] });
      toast(
        client.active
          ? "Cliente arquivado. O histórico foi preservado."
          : "Cliente reativado.",
      );
      setArchiving(null);
    } catch (err) {
      setError(message(err));
    } finally {
      setSaving(false);
    }
  }
  return (
    <div className="wp-page">
      <PageHeading
        eyebrow="SUA CARTEIRA"
        title="Clientes"
        description="Os contatos certos, prontos para a próxima solicitação."
        action={
          <div className="wp-heading-actions">
            <Button variant="secondary" onClick={() => setImporting(true)}>
              <Upload size={16} />
              Importar CSV
            </Button>
            <Button onClick={() => setEditor("new")}>
              <Plus size={17} />
              Novo cliente
            </Button>
          </div>
        }
      />
      <div className="panel wp-clients-panel">
        <div className="wp-toolbar">
          <div className="wp-tabs" aria-label="Filtrar clientes">
            {(
              [
                ["active", "Ativos"],
                ["archived", "Arquivados"],
                ["all", "Todos"],
              ] as const
            ).map(([value, label]) => (
              <button
                type="button"
                className={filter === value ? "wp-tab wp-tab-active" : "wp-tab"}
                key={value}
                aria-pressed={filter === value}
                onClick={() => setFilter(value)}
              >
                {label}
                <span>
                  {
                    clients.filter(
                      (client) =>
                        value === "all" ||
                        client.active === (value === "active"),
                    ).length
                  }
                </span>
              </button>
            ))}
          </div>
          <label className="wp-search">
            <Search size={16} />
            <input
              aria-label="Buscar clientes"
              placeholder="Buscar nome, CNPJ ou e-mail"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
            {search && (
              <button aria-label="Limpar busca" onClick={() => setSearch("")}>
                <X size={14} />
              </button>
            )}
          </label>
        </div>
        {query.isPending ? (
          <LoadingRows />
        ) : query.isError ? (
          <LoadError error={query.error} retry={() => void query.refetch()} />
        ) : !filtered.length ? (
          <EmptyState
            title={
              search
                ? "Nenhum cliente encontrado"
                : filter === "archived"
                  ? "Nenhum cliente arquivado"
                  : "Sua carteira começa aqui"
            }
            description={
              search
                ? "Tente buscar outro nome, CNPJ ou e-mail."
                : filter === "archived"
                  ? "Clientes arquivados continuam com o histórico preservado."
                  : "Cadastre um cliente para pedir seus primeiros documentos."
            }
            action={
              !search && filter !== "archived" ? (
                <Button onClick={() => setEditor("new")}>
                  <Plus size={16} />
                  Cadastrar cliente
                </Button>
              ) : undefined
            }
          />
        ) : (
          <div className="wp-table-scroll">
            <table className="wp-table wp-clients-table">
              <thead>
                <tr>
                  <th>
                    Cliente{" "}
                    <span className="wp-th-count">{filtered.length}</span>
                  </th>
                  <th>Responsável</th>
                  <th>Contato</th>
                  <th>Status</th>
                  <th>
                    <span className="wp-sr-only">Ações</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {filtered.map((client) => (
                  <tr key={client.id}>
                    <td>
                      <div className="wp-client-name">
                        <Avatar name={client.name} />
                        <div>
                          <strong>{client.name}</strong>
                          <span className="wp-subline mono">
                            {client.cnpj
                              ? cnpjLabel(client.cnpj)
                              : "CNPJ não informado"}
                          </span>
                        </div>
                      </div>
                    </td>
                    <td>
                      {client.contactName || (
                        <span className="muted">Não informado</span>
                      )}
                    </td>
                    <td>
                      <span>{client.email || "—"}</span>
                      {client.phone && (
                        <span className="wp-subline">{client.phone}</span>
                      )}
                    </td>
                    <td>
                      <span
                        className={
                          client.active
                            ? "wp-active-status"
                            : "wp-archived-status"
                        }
                      >
                        <i />
                        {client.active ? "Ativo" : "Arquivado"}
                      </span>
                    </td>
                    <td>
                      <div className="wp-row-actions">
                        <Link
                          className="wp-history-link"
                          to={`/app/solicitacoes?client=${encodeURIComponent(client.id)}`}
                        >
                          Histórico
                          <ChevronRight size={14} />
                        </Link>
                        <Button
                          variant="ghost"
                          title={`Editar ${client.name}`}
                          aria-label={`Editar ${client.name}`}
                          onClick={() => setEditor(client)}
                        >
                          <Pencil size={15} />
                        </Button>
                        <Button
                          variant="ghost"
                          title={
                            client.active
                              ? "Arquivar cliente"
                              : "Reativar cliente"
                          }
                          aria-label={`${client.active ? "Arquivar" : "Reativar"} ${client.name}`}
                          onClick={() => {
                            setError("");
                            setArchiving(client);
                          }}
                        >
                          {client.active ? (
                            <Archive size={15} />
                          ) : (
                            <Play size={15} />
                          )}
                        </Button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        {!query.isPending && clients.length > 0 && (
          <div className="wp-table-footer">
            <span>
              {filtered.length} de {clients.length} clientes
            </span>
            <span>Seu histórico fica sempre com você.</span>
          </div>
        )}
      </div>
      <div className="wp-page-note">
        <Users size={16} />
        <span>
          Seus clientes acessam as solicitações por um link. Sem senha e sem
          aplicativo.
        </span>
      </div>
      {editor && (
        <ClientEditor
          client={editor === "new" ? null : editor}
          onClose={() => setEditor(null)}
        />
      )}
      {importing && (
        <ClientImport existing={clients} onClose={() => setImporting(false)} />
      )}
      {archiving && (
        <Modal
          title={archiving.active ? "Arquivar cliente?" : "Reativar cliente?"}
          onClose={() => setArchiving(null)}
        >
          <p className="wp-dialog-copy">
            {archiving.active ? (
              <>
                <strong>{archiving.name}</strong> sairá da lista de clientes
                ativos. As solicitações e documentos anteriores continuarão
                disponíveis no histórico.
              </>
            ) : (
              <>
                <strong>{archiving.name}</strong> voltará a aparecer na sua
                carteira e poderá receber novas solicitações.
              </>
            )}
          </p>
          {error && (
            <p className="inline-error" role="alert">
              {error}
            </p>
          )}
          <div className="form-actions">
            <Button variant="secondary" onClick={() => setArchiving(null)}>
              Cancelar
            </Button>
            <Button
              loading={saving}
              onClick={() => void toggleArchive(archiving)}
            >
              {archiving.active ? "Arquivar cliente" : "Reativar cliente"}
            </Button>
          </div>
        </Modal>
      )}
    </div>
  );
}

const newItem = (): TemplateItem => ({
  id: crypto.randomUUID(),
  name: "",
  instruction: "",
  required: true,
  maxFiles: 5,
});
function TemplateEditor({
  template,
  onClose,
}: {
  template: Template | null;
  onClose: () => void;
}) {
  const [name, setName] = useState(template?.name ?? "");
  const [description, setDescription] = useState(template?.description ?? "");
  const [items, setItems] = useState<TemplateItem[]>(
    template?.items.map((item) => ({ ...item })) ?? [newItem()],
  );
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const queryClient = useQueryClient();
  const toast = useToast();
  function changeItem(id: string, change: Partial<TemplateItem>) {
    setItems((prev) =>
      prev.map((item) => (item.id === id ? { ...item, ...change } : item)),
    );
  }
  function move(index: number, offset: number) {
    setItems((prev) => {
      const next = [...prev];
      [next[index], next[index + offset]] = [next[index + offset], next[index]];
      return next;
    });
  }
  async function submit(event: FormEvent) {
    event.preventDefault();
    setError("");
    if (!items.length)
      return setError("Adicione pelo menos um documento ao modelo.");
    if (!items.some((item) => item.required))
      return setError("Inclua pelo menos um documento obrigatório.");
    if (
      items.some(
        (item) =>
          !item.name.trim() ||
          !Number.isInteger(item.maxFiles) ||
          item.maxFiles < 1 ||
          item.maxFiles > 20,
      )
    )
      return setError(
        "Todos os documentos precisam de nome e limite de 1 a 20 arquivos.",
      );
    setSaving(true);
    const body = {
      name: name.trim(),
      description: description.trim(),
      items: items.map((item) => ({
        ...item,
        name: item.name.trim(),
        instruction: item.instruction.trim(),
      })),
    };
    try {
      if (template) await put(`/templates/${template.id}`, body);
      else await post("/templates", body);
      await queryClient.invalidateQueries({ queryKey: ["templates"] });
      toast(template ? "Modelo atualizado." : "Modelo criado.");
      onClose();
    } catch (err) {
      setError(message(err));
    } finally {
      setSaving(false);
    }
  }
  return (
    <Modal
      title={template ? "Editar modelo" : "Novo modelo"}
      description="Deixe a lista pronta. Reutilize sempre que precisar."
      onClose={onClose}
      wide
    >
      <form className="wp-form" onSubmit={submit}>
        <Field label="Nome do modelo">
          <input
            autoFocus
            className="input"
            required
            maxLength={120}
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="Ex.: Fechamento mensal"
          />
        </Field>
        <Field
          label="Descrição"
          hint="Uma indicação rápida de quando usar este modelo."
        >
          <input
            className="input"
            maxLength={500}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="Documentos para o fechamento contábil do mês"
          />
        </Field>
        <div className="wp-document-heading">
          <div>
            <strong>Documentos solicitados</strong>
            <p className="muted small">
              Organize na ordem em que o cliente deve enviar.
            </p>
          </div>
          <span className="wp-count-label">
            {items.length} {items.length === 1 ? "item" : "itens"}
          </span>
        </div>
        <div className="wp-template-items">
          {items.map((item, index) => (
            <section
              className="wp-template-item"
              key={item.id}
              aria-label={`Documento ${index + 1}`}
            >
              <div className="wp-item-top">
                <span className="wp-item-number mono">
                  {String(index + 1).padStart(2, "0")}
                </span>
                <input
                  aria-label={`Nome do documento ${index + 1}`}
                  required
                  className="input wp-item-name"
                  maxLength={160}
                  value={item.name}
                  onChange={(e) =>
                    changeItem(item.id, { name: e.target.value })
                  }
                  placeholder="Nome do documento"
                />
                <div className="wp-item-controls">
                  <button
                    type="button"
                    aria-label={`Mover documento ${index + 1} para cima`}
                    disabled={index === 0}
                    onClick={() => move(index, -1)}
                  >
                    <ArrowUp size={15} />
                  </button>
                  <button
                    type="button"
                    aria-label={`Mover documento ${index + 1} para baixo`}
                    disabled={index === items.length - 1}
                    onClick={() => move(index, 1)}
                  >
                    <ArrowDown size={15} />
                  </button>
                  <button
                    type="button"
                    aria-label={`Remover documento ${index + 1}`}
                    disabled={items.length === 1}
                    onClick={() =>
                      setItems((prev) =>
                        prev.filter((value) => value.id !== item.id),
                      )
                    }
                  >
                    <Trash2 size={15} />
                  </button>
                </div>
              </div>
              <div className="wp-item-details">
                <input
                  className="input"
                  aria-label={`Instrução do documento ${index + 1}`}
                  maxLength={1000}
                  value={item.instruction}
                  onChange={(e) =>
                    changeItem(item.id, { instruction: e.target.value })
                  }
                  placeholder="Instrução para o cliente (opcional)"
                />
                <div className="wp-item-options">
                  <label className="wp-check">
                    <input
                      type="checkbox"
                      checked={item.required}
                      onChange={(e) =>
                        changeItem(item.id, { required: e.target.checked })
                      }
                    />
                    Obrigatório
                  </label>
                  <label className="wp-file-limit">
                    Até{" "}
                    <input
                      aria-label={`Limite de arquivos do documento ${index + 1}`}
                      className="input"
                      required
                      type="number"
                      min={1}
                      max={20}
                      value={item.maxFiles}
                      onChange={(e) =>
                        changeItem(item.id, {
                          maxFiles: Number(e.target.value),
                        })
                      }
                    />{" "}
                    arquivos
                  </label>
                </div>
              </div>
            </section>
          ))}
        </div>
        <Button
          variant="secondary"
          type="button"
          disabled={items.length >= 30}
          onClick={() => setItems((prev) => [...prev, newItem()])}
        >
          <Plus size={16} />
          Adicionar documento
        </Button>
        <p className="wp-editor-note">
          Alterações no modelo valem para novas solicitações. As solicitações já
          criadas mantêm sua lista original.
        </p>
        {error && (
          <p className="inline-error" role="alert">
            {error}
          </p>
        )}
        <div className="form-actions">
          <Button variant="secondary" type="button" onClick={onClose}>
            Cancelar
          </Button>
          <Button loading={saving} type="submit">
            {template ? "Salvar modelo" : "Criar modelo"}
          </Button>
        </div>
      </form>
    </Modal>
  );
}

export function TemplatesPage() {
  usePageMeta({ title: "Modelos — DocPing" });
  const query = useQuery({
    queryKey: ["templates"],
    queryFn: () => api<Template[]>("/templates"),
  });
  const [editor, setEditor] = useState<Template | "new" | null>(null);
  const [search, setSearch] = useState("");
  const [filter, setFilter] = useState<"active" | "archived" | "all">("active");
  const [archiving, setArchiving] = useState<Template | null>(null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const queryClient = useQueryClient();
  const toast = useToast();
  const all = query.data ?? [];
  const templates = all.filter(
    (template) =>
      (filter === "all" || template.active === (filter === "active")) &&
      `${template.name} ${template.description}`
        .toLowerCase()
        .includes(search.toLowerCase()),
  );
  async function setArchived(template: Template, archived: boolean) {
    setSaving(true);
    setError("");
    try {
      await put(`/templates/${template.id}`, { active: !archived });
      await queryClient.invalidateQueries({ queryKey: ["templates"] });
      toast(archived ? "Modelo arquivado." : "Modelo reativado.");
      setArchiving(null);
    } catch (err) {
      setError(message(err));
    } finally {
      setSaving(false);
    }
  }
  return (
    <div className="wp-page">
      <PageHeading
        eyebrow="MENOS TRABALHO REPETIDO"
        title="Modelos de documentos"
        description="Uma boa lista hoje. Um fechamento mais simples todo mês."
        action={
          <Button onClick={() => setEditor("new")}>
            <Plus size={17} />
            Novo modelo
          </Button>
        }
      />
      <div className="wp-template-toolbar">
        <div className="wp-tabs" aria-label="Filtrar modelos">
          {(
            [
              ["active", "Ativos"],
              ["archived", "Arquivados"],
              ["all", "Todos"],
            ] as const
          ).map(([value, label]) => (
            <button
              type="button"
              className={filter === value ? "wp-tab wp-tab-active" : "wp-tab"}
              key={value}
              aria-pressed={filter === value}
              onClick={() => setFilter(value)}
            >
              {label}
              <span>
                {
                  all.filter(
                    (template) =>
                      value === "all" ||
                      template.active === (value === "active"),
                  ).length
                }
              </span>
            </button>
          ))}
        </div>
        <label className="wp-search">
          <Search size={16} />
          <input
            aria-label="Buscar modelos"
            placeholder="Buscar modelo"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
        </label>
      </div>
      {query.isPending ? (
        <LoadingRows />
      ) : query.isError ? (
        <LoadError error={query.error} retry={() => void query.refetch()} />
      ) : !templates.length ? (
        <div className="panel">
          <EmptyState
            title={
              search
                ? "Nenhum modelo encontrado"
                : filter === "archived"
                  ? "Nenhum modelo arquivado"
                  : "Sua primeira lista de documentos"
            }
            description={
              search
                ? "Tente outro nome para encontrar seu modelo."
                : filter === "archived"
                  ? "Modelos arquivados ficam aqui e podem ser reativados quando precisar."
                  : "Crie um modelo para não precisar escrever a mesma lista todos os meses."
            }
            action={
              !search && filter !== "archived" ? (
                <Button onClick={() => setEditor("new")}>
                  <Plus size={16} />
                  Criar modelo
                </Button>
              ) : undefined
            }
          />
        </div>
      ) : (
        <div className="wp-template-grid">
          {templates.map((template, index) => (
            <article
              className={`panel wp-template-card ${template.active ? "" : "wp-template-archived"}`}
              key={template.id}
            >
              <div className="wp-template-card-top">
                <span className="wp-document-icon">
                  <FileText size={22} strokeWidth={1.5} />
                </span>
                <div className="wp-template-card-tools">
                  <span className="wp-model-index mono">
                    {template.active
                      ? `MODELO ${String(index + 1).padStart(2, "0")}`
                      : "ARQUIVADO"}
                  </span>
                  <Button
                    variant="ghost"
                    aria-label={`${template.active ? "Arquivar" : "Reativar"} modelo ${template.name}`}
                    title={
                      template.active ? "Arquivar modelo" : "Reativar modelo"
                    }
                    disabled={saving}
                    onClick={() => {
                      setError("");
                      if (template.active) setArchiving(template);
                      else void setArchived(template, false);
                    }}
                  >
                    {template.active ? (
                      <Archive size={14} />
                    ) : (
                      <RotateCcw size={14} />
                    )}
                  </Button>
                </div>
              </div>
              <h2>{template.name}</h2>
              <p className="wp-template-description">
                {template.description ||
                  "Uma lista pronta para suas próximas solicitações."}
              </p>
              <div className="wp-template-preview">
                {template.items.slice(0, 4).map((item) => (
                  <div key={item.id}>
                    <Check size={14} />
                    <span>{item.name}</span>
                    {!item.required && <small>opcional</small>}
                  </div>
                ))}
                {template.items.length > 4 && (
                  <p>
                    + {template.items.length - 4}{" "}
                    {template.items.length - 4 === 1
                      ? "documento"
                      : "documentos"}
                  </p>
                )}
              </div>
              <div className="wp-template-card-footer">
                <span>
                  {template.items.length}{" "}
                  {template.items.length === 1 ? "documento" : "documentos"}
                </span>
                <Button variant="ghost" onClick={() => setEditor(template)}>
                  Editar modelo
                  <ArrowRight size={15} />
                </Button>
              </div>
            </article>
          ))}
        </div>
      )}
      <div className="wp-context-card">
        <CalendarDays size={22} strokeWidth={1.5} />
        <div>
          <strong>Todo mês, sem começar do zero.</strong>
          <p>
            Combine um modelo com uma recorrência para criar as solicitações do
            próximo fechamento.
          </p>
        </div>
        <Link to="/app/recorrencias">
          Ver recorrências
          <ArrowRight size={16} />
        </Link>
      </div>
      {editor && (
        <TemplateEditor
          template={editor === "new" ? null : editor}
          onClose={() => setEditor(null)}
        />
      )}
      {archiving && (
        <Modal
          title="Arquivar modelo?"
          onClose={saving ? () => undefined : () => setArchiving(null)}
        >
          <p className="wp-dialog-copy">
            <strong>{archiving.name}</strong> sai da lista de modelos ativos. As
            solicitações existentes manterão seus documentos. Recorrências com
            este modelo precisarão de um novo modelo ativo. Você pode reativá-lo
            depois na aba Arquivados.
          </p>
          {error && (
            <p className="inline-error" role="alert">
              {error}
            </p>
          )}
          <div className="form-actions">
            <Button
              variant="secondary"
              disabled={saving}
              onClick={() => setArchiving(null)}
            >
              Cancelar
            </Button>
            <Button
              loading={saving}
              onClick={() => void setArchived(archiving, true)}
            >
              Arquivar modelo
            </Button>
          </div>
        </Modal>
      )}
    </div>
  );
}

type ScheduleRun = {
  id: string;
  clientId: string;
  competence: string;
  requestId: string;
  createdAt: string;
};
function ScheduleHistory({
  schedule,
  clients,
  onClose,
}: {
  schedule: Schedule;
  clients: Client[];
  onClose: () => void;
}) {
  const query = useQuery({
    queryKey: ["schedules", schedule.id, "runs"],
    queryFn: () => api<ScheduleRun[]>(`/schedules/${schedule.id}/runs`),
  });
  return (
    <Modal
      title="Histórico de execução"
      description={schedule.name}
      onClose={onClose}
      wide
    >
      {query.isPending ? (
        <LoadingRows />
      ) : query.isError ? (
        <LoadError error={query.error} retry={() => void query.refetch()} />
      ) : !query.data.length ? (
        <EmptyState
          title="A primeira execução ainda está por vir"
          description="As solicitações criadas automaticamente por esta recorrência aparecerão aqui."
        />
      ) : (
        <div className="wp-table-scroll">
          <table className="wp-table">
            <thead>
              <tr>
                <th>Cliente</th>
                <th>Competência</th>
                <th>Criada em</th>
                <th>
                  <span className="wp-sr-only">Solicitação</span>
                </th>
              </tr>
            </thead>
            <tbody>
              {[...query.data]
                .sort((a, b) => b.createdAt.localeCompare(a.createdAt))
                .map((run) => (
                  <tr key={run.id}>
                    <td>
                      {clients.find((client) => client.id === run.clientId)
                        ?.name || "Cliente arquivado"}
                    </td>
                    <td>{shortMonth(run.competence)}</td>
                    <td>{dateLabel(run.createdAt)}</td>
                    <td>
                      <Link
                        className="wp-history-link"
                        to={`/app/solicitacoes/${run.requestId}`}
                      >
                        Ver solicitação
                        <ArrowRight size={14} />
                      </Link>
                    </td>
                  </tr>
                ))}
            </tbody>
          </table>
        </div>
      )}
    </Modal>
  );
}

type ScheduleDraft = Omit<Schedule, "id" | "lastRun" | "nextRun">;
const scheduleFields = (schedule: ScheduleDraft): ScheduleDraft => ({
  name: schedule.name,
  templateId: schedule.templateId,
  clientIds: [...schedule.clientIds],
  dayOfMonth: schedule.dayOfMonth,
  dueDays: schedule.dueDays,
  active: schedule.active,
});
function ScheduleEditor({
  schedule,
  templates,
  clients,
  onClose,
}: {
  schedule: Schedule | null;
  templates: Template[];
  clients: Client[];
  onClose: () => void;
}) {
  const [draft, setDraft] = useState<ScheduleDraft>(
    schedule
      ? {
          ...scheduleFields(schedule),
          templateId: templates.some(
            (template) => template.id === schedule.templateId,
          )
            ? schedule.templateId
            : "",
          clientIds: schedule.clientIds.filter((id) =>
            clients.some((client) => client.id === id && client.active),
          ),
        }
      : {
          name: "",
          templateId: templates[0]?.id ?? "",
          clientIds: [],
          dayOfMonth: 5,
          dueDays: 10,
          active: true,
        },
  );
  const [search, setSearch] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const queryClient = useQueryClient();
  const toast = useToast();
  const activeClients = clients.filter((client) => client.active);
  const selectableClients = activeClients.slice(0, 100);
  const allSelected =
    selectableClients.length > 0 &&
    selectableClients.every((client) => draft.clientIds.includes(client.id));
  const shown = activeClients.filter((client) =>
    `${client.name} ${client.cnpj}`
      .toLowerCase()
      .includes(search.toLowerCase()),
  );
  const change = (change: Partial<ScheduleDraft>) =>
    setDraft((prev) => ({ ...prev, ...change }));
  async function submit(event: FormEvent) {
    event.preventDefault();
    setError("");
    if (!draft.clientIds.length)
      return setError("Selecione pelo menos um cliente.");
    if (draft.clientIds.length > 100)
      return setError("Selecione até 100 clientes por recorrência.");
    setSaving(true);
    const body = { ...draft, name: draft.name.trim() };
    try {
      if (schedule) await put(`/schedules/${schedule.id}`, body);
      else await post("/schedules", body);
      await queryClient.invalidateQueries({ queryKey: ["schedules"] });
      toast(schedule ? "Recorrência atualizada." : "Recorrência criada.");
      onClose();
    } catch (err) {
      setError(message(err));
    } finally {
      setSaving(false);
    }
  }
  return (
    <Modal
      title={schedule ? "Editar recorrência" : "Nova recorrência mensal"}
      description="Defina uma vez. As próximas solicitações nascem no dia certo."
      onClose={onClose}
      wide
    >
      <form className="wp-form" onSubmit={submit}>
        <Field label="Nome da recorrência">
          <input
            className="input"
            autoFocus
            required
            maxLength={120}
            value={draft.name}
            onChange={(e) => change({ name: e.target.value })}
            placeholder="Ex.: Fechamento mensal — carteira geral"
          />
        </Field>
        <Field label="Modelo de documentos">
          <select
            className="select"
            required
            value={draft.templateId}
            onChange={(e) => change({ templateId: e.target.value })}
          >
            <option value="" disabled>
              Selecione um modelo ativo
            </option>
            {templates.map((template) => (
              <option key={template.id} value={template.id}>
                {template.name} · {template.items.length} documentos
              </option>
            ))}
          </select>
        </Field>
        <div className="field-row">
          <Field label="Criar todo dia">
            <select
              className="select"
              value={draft.dayOfMonth}
              onChange={(e) => change({ dayOfMonth: Number(e.target.value) })}
            >
              {Array.from({ length: 31 }, (_, index) => (
                <option key={index + 1} value={index + 1}>
                  {index + 1} de cada mês
                </option>
              ))}
            </select>
          </Field>
          <Field
            label="Prazo para envio"
            hint="Contado a partir da criação da solicitação."
          >
            <div className="wp-input-suffix">
              <input
                className="input"
                type="number"
                required
                min={1}
                max={90}
                value={draft.dueDays}
                onChange={(e) => change({ dueDays: Number(e.target.value) })}
              />
              <span>dias</span>
            </div>
          </Field>
        </div>
        <div className="wp-schedule-explainer">
          <CalendarDays size={18} />
          <p>
            No dia <strong>{draft.dayOfMonth}</strong>, criaremos solicitações
            da <strong>competência do mês anterior</strong>. Quando o mês não
            tiver esse dia, usaremos o último dia. Fuso: Brasília.
          </p>
        </div>
        <div className="wp-document-heading">
          <strong>Clientes participantes</strong>
          <span className="wp-count-label">
            {draft.clientIds.length} selecionados
          </span>
        </div>
        {schedule?.clientIds.some(
          (id) => !activeClients.some((client) => client.id === id),
        ) && (
          <p className="small muted">
            Clientes arquivados foram removidos desta seleção. O histórico
            anterior está preservado.
          </p>
        )}
        <div className="wp-client-select">
          <div className="wp-select-toolbar">
            <label className="wp-search">
              <Search size={15} />
              <input
                aria-label="Buscar participantes"
                placeholder="Buscar cliente"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
              />
            </label>
            <button
              type="button"
              onClick={() =>
                change({
                  clientIds: allSelected
                    ? []
                    : selectableClients.map((client) => client.id),
                })
              }
            >
              {allSelected
                ? "Limpar seleção"
                : activeClients.length > 100
                  ? "Selecionar os 100 primeiros"
                  : "Selecionar todos"}
            </button>
          </div>
          <div className="wp-select-list">
            {shown.map((client) => (
              <label className="wp-client-choice" key={client.id}>
                <input
                  type="checkbox"
                  checked={draft.clientIds.includes(client.id)}
                  disabled={
                    draft.clientIds.length >= 100 &&
                    !draft.clientIds.includes(client.id)
                  }
                  onChange={(e) =>
                    change({
                      clientIds: e.target.checked
                        ? [...draft.clientIds, client.id]
                        : draft.clientIds.filter((id) => id !== client.id),
                    })
                  }
                />
                <Avatar name={client.name} />
                <span>
                  <strong>{client.name}</strong>
                  <small>
                    {client.email ||
                      (client.cnpj
                        ? cnpjLabel(client.cnpj)
                        : "Sem e-mail cadastrado")}
                  </small>
                </span>
              </label>
            ))}
            {!shown.length && (
              <p className="wp-select-empty muted">
                Nenhum cliente encontrado.
              </p>
            )}
          </div>
        </div>
        {activeClients.length > 100 && (
          <p className="small muted">
            Até 100 clientes por recorrência. Crie outra rotina para os demais
            clientes.
          </p>
        )}
        <label className="wp-check">
          <input
            type="checkbox"
            checked={draft.active}
            onChange={(e) => change({ active: e.target.checked })}
          />
          Recorrência ativa
        </label>
        {error && (
          <p className="inline-error" role="alert">
            {error}
          </p>
        )}
        <div className="form-actions">
          <Button variant="secondary" type="button" onClick={onClose}>
            Cancelar
          </Button>
          <Button loading={saving} type="submit">
            {schedule ? "Salvar alterações" : "Criar recorrência"}
          </Button>
        </div>
      </form>
    </Modal>
  );
}

export function SchedulesPage() {
  usePageMeta({ title: "Recorrências — DocPing" });
  const schedules = useQuery({
    queryKey: ["schedules"],
    queryFn: () => api<Schedule[]>("/schedules"),
  });
  const templates = useQuery({
    queryKey: ["templates"],
    queryFn: () => api<Template[]>("/templates"),
  });
  const clients = useQuery({
    queryKey: ["clients"],
    queryFn: () => api<Client[]>("/clients"),
  });
  const [editor, setEditor] = useState<Schedule | "new" | null>(null);
  const [toggling, setToggling] = useState("");
  const [history, setHistory] = useState<Schedule | null>(null);
  const queryClient = useQueryClient();
  const toast = useToast();
  // The models list includes archived ones so they stay reactivatable; a
  // recurrence can only be built on an active model.
  const activeTemplates = (templates.data ?? []).filter(
    (template) => template.active,
  );
  const canCreate =
    !!activeTemplates.length && !!clients.data?.some((client) => client.active);
  async function toggle(schedule: Schedule) {
    setToggling(schedule.id);
    try {
      await put(`/schedules/${schedule.id}`, {
        ...scheduleFields(schedule),
        active: !schedule.active,
      });
      await queryClient.invalidateQueries({ queryKey: ["schedules"] });
      toast(
        schedule.active ? "Recorrência pausada." : "Recorrência reativada.",
      );
    } catch (err) {
      toast(message(err), "error");
    } finally {
      setToggling("");
    }
  }
  return (
    <div className="wp-page">
      <PageHeading
        eyebrow="SEU PRÓXIMO FECHAMENTO"
        title="Recorrências mensais"
        description="As mesmas listas, para os mesmos clientes. Sem refazer o trabalho."
        action={
          <Button onClick={() => setEditor("new")} disabled={!canCreate}>
            <Plus size={17} />
            Nova recorrência
          </Button>
        }
      />
      <div className="wp-recurrence-banner">
        <div className="wp-recurrence-icon">
          <CalendarDays size={27} strokeWidth={1.5} />
        </div>
        <div>
          <h2>Uma rotina que anda sozinha.</h2>
          <p>
            As solicitações são criadas automaticamente no dia escolhido, com o
            modelo e os clientes que você definir.
          </p>
        </div>
        <span className="wp-timezone mono">America/Sao_Paulo</span>
      </div>
      {!templates.isPending &&
        !clients.isPending &&
        !canCreate &&
        !templates.isError &&
        !clients.isError && (
          <div className="wp-prerequisite">
            Para criar uma recorrência, cadastre pelo menos um cliente ativo e
            um modelo.{" "}
            <Link
              to={
                !clients.data?.some((client) => client.active)
                  ? "/app/clientes"
                  : "/app/modelos"
              }
            >
              Preparar minha carteira
              <ArrowRight size={15} />
            </Link>
          </div>
        )}
      {schedules.isPending || clients.isPending || templates.isPending ? (
        <LoadingRows />
      ) : schedules.isError || clients.isError || templates.isError ? (
        <LoadError
          error={schedules.error || clients.error || templates.error}
          retry={() => {
            void schedules.refetch();
            void clients.refetch();
            void templates.refetch();
          }}
        />
      ) : !schedules.data?.length ? (
        <div className="panel">
          <EmptyState
            title="Seu próximo mês já pode ficar pronto"
            description="Crie a primeira recorrência para repetir sua rotina de coleta sem recadastrar os documentos."
            action={
              canCreate ? (
                <Button onClick={() => setEditor("new")}>
                  <Plus size={16} />
                  Criar recorrência
                </Button>
              ) : undefined
            }
          />
        </div>
      ) : (
        <div className="panel wp-schedule-list">
          <div className="wp-list-heading">
            <h2>Suas rotinas</h2>
            <span>
              {schedules.data.filter((schedule) => schedule.active).length}{" "}
              ativas · {schedules.data.length} no total
            </span>
          </div>
          {schedules.data.map((schedule) => (
            <article className="wp-schedule-row" key={schedule.id}>
              <div className="wp-day-stamp">
                <span>TODO DIA</span>
                <strong>{String(schedule.dayOfMonth).padStart(2, "0")}</strong>
              </div>
              <div className="wp-schedule-main">
                <div className="wp-schedule-title">
                  <h3>{schedule.name}</h3>
                  <span
                    className={
                      schedule.active
                        ? "wp-active-status"
                        : "wp-archived-status"
                    }
                  >
                    <i />
                    {schedule.active ? "Ativa" : "Pausada"}
                  </span>
                </div>
                <p>
                  {(() => {
                    const model = templates.data?.find(
                      (template) => template.id === schedule.templateId,
                    );
                    if (!model) return "Modelo removido — selecione outro";
                    return model.active
                      ? model.name
                      : `${model.name} — arquivado, selecione outro`;
                  })()}
                  <span>·</span>
                  {schedule.clientIds.length}{" "}
                  {schedule.clientIds.length === 1 ? "cliente" : "clientes"}
                  <span>·</span>Prazo de {schedule.dueDays} dias
                </p>
                <div className="wp-run-meta">
                  {schedule.nextRun && schedule.active ? (
                    <>
                      Próxima criação:{" "}
                      <strong>{dateLabel(schedule.nextRun)}</strong>
                    </>
                  ) : schedule.active ? (
                    "Aguardando a próxima execução programada"
                  ) : (
                    "Novas solicitações pausadas"
                  )}
                  {schedule.lastRun && (
                    <span>
                      Última execução em {dateLabel(schedule.lastRun)}
                    </span>
                  )}
                </div>
              </div>
              <div className="wp-schedule-actions">
                <Button
                  variant="secondary"
                  loading={toggling === schedule.id}
                  disabled={!!toggling}
                  onClick={() => void toggle(schedule)}
                >
                  {schedule.active ? <Pause size={15} /> : <Play size={15} />}
                  {schedule.active ? "Pausar" : "Reativar"}
                </Button>
                <Button
                  variant="ghost"
                  aria-label={`Histórico de ${schedule.name}`}
                  title="Histórico de execução"
                  onClick={() => setHistory(schedule)}
                >
                  <History size={16} />
                </Button>
                <Button
                  variant="ghost"
                  aria-label={`Editar ${schedule.name}`}
                  title="Editar recorrência"
                  onClick={() => setEditor(schedule)}
                >
                  <Pencil size={16} />
                </Button>
              </div>
            </article>
          ))}
        </div>
      )}
      <p className="wp-page-note">
        <Mail size={16} />
        <span>
          E-mails dependem do provedor configurado. O compartilhamento pelo
          WhatsApp continua sendo manual.
        </span>
      </p>
      {editor && activeTemplates.length > 0 && clients.data && (
        <ScheduleEditor
          schedule={editor === "new" ? null : editor}
          templates={activeTemplates}
          clients={clients.data}
          onClose={() => setEditor(null)}
        />
      )}
      {history && (
        <ScheduleHistory
          schedule={history}
          clients={clients.data ?? []}
          onClose={() => setHistory(null)}
        />
      )}
    </div>
  );
}

export function SettingsPage() {
  usePageMeta({ title: "Configurações — DocPing" });
  const { session } = useSession();
  const organization = session.organization;
  const [draft, setDraft] = useState<Organization>({ ...organization });
  const [reminderText, setReminderText] = useState(
    (organization.reminderDays ?? [3, 6]).join(", "),
  );
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [billingPending, setBillingPending] = useState(false);
  const [billingError, setBillingError] = useState("");
  const billing = session.billing;
  const queryClient = useQueryClient();
  const toast = useToast();
  useEffect(() => {
    setDraft({ ...organization });
    setReminderText((organization.reminderDays ?? [3, 6]).join(", "));
  }, [organization]);
  async function submit(event: FormEvent) {
    event.preventDefault();
    setError("");
    const reminderDays = reminderText
      .split(",")
      .map((day) => day.trim())
      .filter(Boolean)
      .map(Number);
    if (
      draft.remindersEnabled &&
      (!reminderDays.length ||
        reminderDays.length > 5 ||
        reminderDays.some(
          (day) => !Number.isInteger(day) || day < 1 || day > 60,
        ) ||
        new Set(reminderDays).size !== reminderDays.length)
    )
      return setError(
        "Defina de 1 a 5 lembretes diferentes, entre 1 e 60 dias, separados por vírgula.",
      );
    setSaving(true);
    try {
      await put("/organization", {
        name: draft.name.trim(),
        email: (draft.email ?? "").trim(),
        remindersEnabled: !!draft.remindersEnabled,
        reminderDays: draft.remindersEnabled
          ? reminderDays.sort((a, b) => a - b)
          : (organization.reminderDays ?? [3, 6]),
        retentionDays: draft.retentionDays ?? 365,
        aiReview: draft.aiReview ?? "suggest",
      });
      await queryClient.invalidateQueries({ queryKey: ["session"] });
      toast("Configurações do escritório salvas.");
    } catch (err) {
      setError(message(err));
    } finally {
      setSaving(false);
    }
  }
  return (
    <div className="wp-page">
      <PageHeading
        eyebrow="DO SEU JEITO"
        title="Configurações"
        description="As informações e preferências do seu escritório, em um lugar só."
      />
      <form className="wp-settings-layout" onSubmit={submit}>
        <div className="wp-settings-main">
          <section className="panel wp-settings-section">
            <div className="wp-settings-section-title">
              <span className="wp-section-number mono">01</span>
              <div>
                <h2>Seu escritório</h2>
                <p>
                  É assim que os clientes reconhecem quem está pedindo os
                  documentos.
                </p>
              </div>
            </div>
            <Field label="Nome do escritório">
              <input
                className="input"
                required
                maxLength={160}
                value={draft.name}
                onChange={(e) =>
                  setDraft((prev) => ({ ...prev, name: e.target.value }))
                }
              />
            </Field>
            <Field
              label="E-mail de contato"
              hint="Contato do escritório exibido nas suas comunicações."
            >
              <input
                className="input"
                type="email"
                maxLength={254}
                value={draft.email ?? ""}
                onChange={(e) =>
                  setDraft((prev) => ({ ...prev, email: e.target.value }))
                }
                placeholder="contato@escritorio.com.br"
              />
            </Field>
          </section>
          <section className="panel wp-settings-section">
            <div className="wp-settings-section-title">
              <span className="wp-section-number mono">02</span>
              <div>
                <h2>Lembretes por e-mail</h2>
                <p>Uma régua curta para ajudar seus clientes a não esquecer.</p>
              </div>
            </div>
            <label className="wp-toggle-row">
              <div>
                <strong>Lembrar clientes com pendências</strong>
                <span>
                  Solicitações concluídas deixam de receber lembretes.
                </span>
              </div>
              <input
                type="checkbox"
                role="switch"
                aria-label="Lembrar clientes com pendências"
                checked={!!draft.remindersEnabled}
                onChange={(e) =>
                  setDraft((prev) => ({
                    ...prev,
                    remindersEnabled: e.target.checked,
                  }))
                }
              />
            </label>
            <Field
              label="Dias após a criação"
              hint="Separe por vírgula. Ex.: 3, 6 para lembrar após 3 e 6 dias."
            >
              <input
                className="input wp-short-input"
                disabled={!draft.remindersEnabled}
                value={reminderText}
                onChange={(e) => setReminderText(e.target.value)}
                placeholder="3, 6"
              />
            </Field>
            <p className="wp-settings-help">
              O resultado de cada tentativa de envio fica no histórico da
              solicitação. O envio depende da configuração do provedor de
              e-mail.
            </p>
          </section>
          <section className="panel wp-settings-section">
            <div className="wp-settings-section-title">
              <span className="wp-section-number mono">03</span>
              <div>
                <h2>Análise automática de documentos</h2>
                <p>
                  Cada arquivo enviado é lido e conferido contra o que foi
                  solicitado antes de chegar à sua revisão.
                </p>
              </div>
            </div>
            <div className="wp-choice-group" role="radiogroup" aria-label="Análise automática de documentos">
              {(
                [
                  {
                    value: "auto",
                    title: "Aprovar e reprovar automaticamente",
                    hint: "Documentos com todos os critérios conferidos são aprovados sozinhos; os reprovados voltam ao cliente com o motivo. Qualquer dúvida continua vindo para você.",
                  },
                  {
                    value: "suggest",
                    title: "Apenas sugerir",
                    hint: "O resultado aparece na tela de revisão, mas a decisão continua sendo sua.",
                  },
                  {
                    value: "off",
                    title: "Desativada",
                    hint: "Nenhum documento é enviado para análise.",
                  },
                ] as const
              ).map((option) => (
                <label
                  key={option.value}
                  className={
                    (draft.aiReview ?? "suggest") === option.value
                      ? "wp-choice active"
                      : "wp-choice"
                  }
                >
                  <input
                    type="radio"
                    name="aiReview"
                    value={option.value}
                    checked={(draft.aiReview ?? "suggest") === option.value}
                    onChange={() =>
                      setDraft((prev) => ({ ...prev, aiReview: option.value }))
                    }
                  />
                  <div>
                    <strong>{option.title}</strong>
                    <span>{option.hint}</span>
                  </div>
                </label>
              ))}
            </div>
            <p className="wp-settings-help">
              A leitura usa Docling, com PaddleOCR para fotos e digitalizações,
              e a conferência usa o modelo Jev. Se a análise falhar, o
              documento segue para revisão manual — nada é aprovado sem
              conferência.
            </p>
          </section>
          <section className="panel wp-settings-section">
            <div className="wp-settings-section-title">
              <span className="wp-section-number mono">04</span>
              <div>
                <h2>Retenção de documentos</h2>
                <p>
                  Defina por quanto tempo o escritório precisa manter os
                  arquivos.
                </p>
              </div>
            </div>
            <Field
              label="Período de retenção"
              hint="A política deve considerar as obrigações e acordos do seu escritório."
            >
              <select
                className="select wp-retention-select"
                value={draft.retentionDays ?? 365}
                onChange={(e) =>
                  setDraft((prev) => ({
                    ...prev,
                    retentionDays: Number(e.target.value),
                  }))
                }
              >
                {![90, 180, 365, 730, 1825].includes(
                  draft.retentionDays ?? 365,
                ) && (
                  <option value={draft.retentionDays}>
                    {draft.retentionDays} dias
                  </option>
                )}
                <option value={90}>90 dias</option>
                <option value={180}>180 dias</option>
                <option value={365}>1 ano</option>
                <option value={730}>2 anos</option>
                <option value={1825}>5 anos</option>
              </select>
            </Field>
          </section>
          {billing?.enabled && (
            <section className="panel wp-settings-section">
              <div className="wp-settings-section-title">
                <span className="wp-section-number mono">05</span>
                <div>
                  <h2>Plano e cobrança</h2>
                  <p>
                    O pagamento é processado pela Stripe. Trocar de plano,
                    mudar o cartão ou cancelar acontece na área de cobrança
                    dela.
                  </p>
                </div>
              </div>
              <dl className="wp-billing-facts">
                <div>
                  <dt>Plano</dt>
                  <dd>{planName(billing.plan)}</dd>
                </div>
                <div>
                  <dt>Assinatura</dt>
                  <dd>
                    <span
                      className={`wp-billing-dot wp-billing-dot-${billing.entitled ? "ok" : "off"}`}
                      aria-hidden
                    />
                    {subscriptionLabel(billing.status)}
                  </dd>
                </div>
                <div>
                  <dt>Solicitações ativas</dt>
                  <dd>
                    {billing.activeRequests}
                    {billing.limits.activeRequests > 0
                      ? ` de ${billing.limits.activeRequests}`
                      : ""}
                  </dd>
                </div>
                {billing.periodEnd && (
                  <div>
                    <dt>
                      {billing.cancelAtPeriodEnd ? "Encerra em" : "Renova em"}
                    </dt>
                    <dd>{dateLabel(billing.periodEnd)}</dd>
                  </div>
                )}
              </dl>
              {billing.cancelAtPeriodEnd && (
                <p className="wp-billing-note" role="status">
                  A assinatura está marcada para não renovar. O escritório
                  continua funcionando até o fim do período pago.
                </p>
              )}
              {billingError && (
                <p className="inline-error" role="alert">
                  {billingError}
                </p>
              )}
              <div className="wp-billing-actions">
                {billing.manageable ? (
                  <Button
                    type="button"
                    loading={billingPending}
                    onClick={async () => {
                      setBillingError("");
                      setBillingPending(true);
                      try {
                        const { url } = await openBillingPortal();
                        window.location.assign(url);
                      } catch (err) {
                        setBillingError(message(err));
                        setBillingPending(false);
                      }
                    }}
                  >
                    Abrir área de cobrança
                  </Button>
                ) : (
                  <Link to="/assinar">
                    <Button type="button">Escolher um plano</Button>
                  </Link>
                )}
              </div>
            </section>
          )}
          {error && (
            <p className="inline-error" role="alert">
              {error}
            </p>
          )}
          <div className="wp-settings-save">
            <span>As alterações valem para este escritório.</span>
            <Button loading={saving} type="submit">
              Salvar configurações
            </Button>
          </div>
        </div>
        <aside className="wp-settings-aside">
          <div className="panel wp-account-card">
            <div className="wp-account-avatar">
              <Avatar name={session.user.name} />
            </div>
            <span className="eyebrow">SUA CONTA</span>
            <h3>{session.user.name}</h3>
            <p>{session.user.email}</p>
            <div className="wp-plan-info">
              <span>Plano atual</span>
              <strong>{organization.plan || "Piloto"}</strong>
            </div>
            <div className="wp-plan-info">
              <span>Limite de clientes ativos</span>
              <strong>{organization.clientLimit}</strong>
            </div>
            {organization.demo && (
              <span className="wp-demo-note">Ambiente de demonstração</span>
            )}
          </div>
          <p className="wp-aside-note">
            As configurações são compartilhadas por todos os usuários do
            escritório.
          </p>
        </aside>
      </form>
    </div>
  );
}
