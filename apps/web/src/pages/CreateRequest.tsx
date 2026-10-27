import { useEffect, useRef, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  ArrowLeft,
  ArrowRight,
  Check,
  Plus,
  Search,
  FileText,
  CalendarDays,
  Link2,
  Copy,
  Mail,
  MessageCircle,
  ShieldCheck,
  ChevronUp,
  ChevronDown,
  Trash2,
  Repeat2,
  Send,
} from "lucide-react";
import { api, post } from "@/lib/api";
import { usePageMeta } from "@/lib/seo";
import type {
  Client,
  Template,
  TemplateItem,
  DocumentRequest,
} from "@/lib/types";
import {
  Avatar,
  Button,
  Field,
  PageHeading,
  Loading,
  ErrorState,
} from "@/components/shared";
import { ClientEditor } from "./WorkspacePages";
import { monthName, today } from "@/lib/format";
import { useToast } from "@/lib/toast";
import "./request-flow.css";

export function CreateRequest() {
  usePageMeta({ title: "Nova solicitação — DocPing" });
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const toast = useToast();
  const [step, setStep] = useState(1);
  const [chosen, setChosen] = useState<string[]>([]);
  // Arriving from a client that was just registered: that client is already
  // the answer to step one, so the flow opens on the documents instead of
  // asking the office to find in a list the name it typed a second ago.
  const [searchParams] = useSearchParams();
  const fromClient = searchParams.get("cliente");
  const [q, setQ] = useState("");
  const [adding, setAdding] = useState(false);
  const [templateId, setTemplateId] = useState("");
  const [items, setItems] = useState<TemplateItem[]>([]);
  const [title, setTitle] = useState("Fechamento mensal");
  const current = new Date(`${today()}T12:00:00Z`);
  const prev = new Date(
    Date.UTC(current.getUTCFullYear(), current.getUTCMonth() - 1, 1),
  );
  const [competence, setCompetence] = useState(prev.toISOString().slice(0, 7));
  const defaultDue = new Date(current.getTime() + 7 * 86400000)
    .toISOString()
    .slice(0, 10);
  const [dueDate, setDueDate] = useState(defaultDue);
  const [channel, setChannel] = useState("link");
  const [sensitive, setSensitive] = useState(false);
  const [created, setCreated] = useState<DocumentRequest[]>([]);
  const [error, setError] = useState("");
  const [links, setLinks] = useState<Record<string, string>>({});
  const [sharing, setSharing] = useState("");
  // Which requests already had their e-mail accepted by the provider, so the
  // card says so instead of offering a send that the one-minute guard on the
  // API would refuse anyway.
  const [emailed, setEmailed] = useState<string[]>([]);
  const clientsQuery = useQuery({
    queryKey: ["clients"],
    queryFn: () => api<Client[]>("/clients"),
  });
  const templatesQuery = useQuery({
    queryKey: ["templates"],
    queryFn: () => api<Template[]>("/templates"),
  });
  const clients = clientsQuery.data || [];
  // Only once, and only for a client that really belongs to this workspace
  // and is active -- the id travels in the URL, so it is a request, not a
  // fact. Going back to step one afterwards has to keep working, which is
  // what the ref guards.
  const jumped = useRef(false);
  useEffect(() => {
    if (jumped.current || !fromClient || clientsQuery.isLoading) return;
    jumped.current = true;
    const client = (clientsQuery.data || []).find(
      (c) => c.id === fromClient && c.active,
    );
    if (!client) return;
    setChosen([client.id]);
    setStep(2);
  }, [fromClient, clientsQuery.data, clientsQuery.isLoading]);
  // The list now carries archived models so they can be reactivated; only
  // active ones may start a new request.
  const templates = (templatesQuery.data || []).filter((t) => t.active);
  const selectedClients = clients.filter((c) => chosen.includes(c.id));
  const create = useMutation({
    mutationFn: () =>
      post<DocumentRequest[]>("/requests", {
        clientIds: chosen,
        templateId,
        title,
        competence,
        dueDate,
        items,
        sensitive,
      }),
    onSuccess: async (requests) => {
      setCreated(requests);
      await queryClient.invalidateQueries({ queryKey: ["requests"] });
      if (channel === "email") {
        for (const r of requests) {
          try {
            const delivery = await post<{ status: string; error?: string }>(
              `/requests/${r.id}/notify`,
              { channel: "email" },
            );
            if (delivery.status !== "accepted")
              throw new Error(
                delivery.error || "O e-mail não foi aceito pelo provedor.",
              );
            setEmailed((p) => [...p, r.id]);
            toast(`E-mail de ${r.clientName} aceito pelo provedor.`);
          } catch (e) {
            toast(
              e instanceof Error
                ? e.message
                : "Não foi possível enviar o e-mail.",
              "error",
            );
          }
        }
      }
    },
    onError: (e) => setError(e.message),
  });
  function chooseTemplate(t: Template) {
    setTemplateId(t.id);
    setTitle(t.name);
    setItems(t.items.map((i) => ({ ...i })));
  }
  function next() {
    setError("");
    if (step === 1 && !chosen.length) {
      setError("Selecione ao menos um cliente para continuar.");
      return;
    }
    if (
      step === 2 &&
      (!items.length ||
        items.some((i) => !i.name.trim()) ||
        !items.some((i) => i.required))
    ) {
      setError("Inclua ao menos um documento obrigatório e preencha os nomes.");
      return;
    }
    setStep((s) => s + 1);
  }
  function move(index: number, direction: number) {
    const copy = [...items];
    [copy[index], copy[index + direction]] = [
      copy[index + direction],
      copy[index],
    ];
    setItems(copy);
  }
  async function share(r: DocumentRequest, mode: string) {
    setSharing(r.id);
    try {
      let url = links[r.id];
      if (!url) {
        const result = await post<{ url: string; token: string }>(
          `/requests/${r.id}/share`,
        );
        url = result.url;
        setLinks((p) => ({ ...p, [r.id]: url }));
      }
      if (mode === "email") {
        const delivery = await post<{ status: string; error?: string }>(
          `/requests/${r.id}/notify`,
          { channel: "email" },
        );
        if (delivery.status !== "accepted")
          throw new Error(
            delivery.error || "O e-mail não foi aceito pelo provedor.",
          );
        setEmailed((p) => [...p, r.id]);
        toast(`E-mail de ${r.clientName} aceito pelo provedor.`);
      } else if (mode === "copy") {
        await navigator.clipboard.writeText(url);
        toast("Link copiado. Pronto para compartilhar.");
      } else if (mode === "portal") {
        window.open(url, "_blank", "noopener,noreferrer");
      } else {
        const client = clients.find((c) => c.id === r.clientId);
        const phone = client?.phone.replace(/\D/g, "") || "";
        const fullPhone =
          phone.length === 10 || phone.length === 11 ? `55${phone}` : phone;
        const href = `https://wa.me/${fullPhone}?text=${encodeURIComponent(`Olá${client?.contactName ? ", " + client.contactName : ""}! ${r.title}: envie os documentos até ${r.dueDate.split("-").reverse().join("/")} pelo link ${url}`)}`;
        window.open(href, "_blank", "noopener,noreferrer");
        await post(`/requests/${r.id}/notify`, { channel: "whatsapp_opened" });
        toast("WhatsApp aberto. Confirme o envio da mensagem por lá.");
      }
    } catch (e) {
      toast(
        e instanceof Error ? e.message : "Não foi possível compartilhar.",
        "error",
      );
    } finally {
      setSharing("");
    }
  }
  if (clientsQuery.isLoading || templatesQuery.isLoading) return <Loading />;
  if (clientsQuery.error || templatesQuery.error)
    return (
      <ErrorState
        message={(clientsQuery.error || templatesQuery.error)!.message}
        onRetry={() => {
          void clientsQuery.refetch();
          void templatesQuery.refetch();
        }}
      />
    );
  if (created.length)
    return (
      <div className="created-screen">
        <div className="created-check">
          <Check size={32} strokeWidth={1.5} />
        </div>
        <p className="eyebrow">TUDO CERTO POR AQUI</p>
        <h1>
          {created.length === 1
            ? "Sua solicitação está pronta."
            : `${created.length} solicitações prontas.`}
        </h1>
        <p>
          Agora é só compartilhar. Seus clientes enviam pelo celular,
          <br />
          sem criar conta ou instalar nada.
        </p>
        <div className="created-list">
          {created.map((r) => (
            <div className="created-card" key={r.id}>
              <div className="created-card-heading">
                <Avatar name={r.clientName} />
                <div>
                  <strong>{r.clientName}</strong>
                  <span>
                    {r.title} · {items.length} documentos
                  </span>
                </div>
                <span className="created-ready">
                  <span />
                  Pronta
                </span>
              </div>
              <div className="created-card-actions">
                <Button
                  variant="secondary"
                  loading={sharing === r.id}
                  onClick={() => void share(r, "copy")}
                >
                  <Copy size={14} />
                  Copiar link
                </Button>
                {emailed.includes(r.id) ? (
                  <span className="created-sent">
                    <Check size={13} />
                    E-mail enviado
                  </span>
                ) : (
                  <Button
                    variant="secondary"
                    disabled={!clients.find((c) => c.id === r.clientId)?.email}
                    onClick={() => void share(r, "email")}
                  >
                    <Mail size={14} />
                    Enviar e-mail
                  </Button>
                )}
                <Button
                  variant="secondary"
                  disabled={!clients.find((c) => c.id === r.clientId)?.phone}
                  onClick={() => void share(r, "whatsapp")}
                >
                  <MessageCircle size={14} />
                  WhatsApp
                </Button>
                <button
                  className="text-link"
                  onClick={() => void share(r, "portal")}
                >
                  Abrir portal
                  <ArrowRight size={13} />
                </button>
              </div>
              {links[r.id] && (
                <div className="created-link">
                  <Link2 size={13} />
                  <input
                    readOnly
                    value={links[r.id]}
                    aria-label={`Link de ${r.clientName}`}
                  />
                </div>
              )}
            </div>
          ))}
        </div>
        <div className="created-bottom-actions">
          <Button
            onClick={() => navigate(`/app/solicitacoes/${created[0].id}`)}
          >
            Acompanhar solicitação
            <ArrowRight size={15} />
          </Button>
          <Button
            variant="ghost"
            onClick={() => {
              setCreated([]);
              setChosen([]);
              setLinks({});
              setStep(1);
            }}
          >
            Criar outra solicitação
          </Button>
        </div>
        <p className="created-safe">
          <ShieldCheck size={13} />
          Cada cliente acessa apenas os seus documentos.
        </p>
      </div>
    );
  return (
    <div className="create-request">
      <Link to="/app/solicitacoes" className="back-link">
        <ArrowLeft size={14} />
        Voltar para solicitações
      </Link>
      <PageHeading
        title="Nova solicitação"
        description="Peça uma vez. Acompanhe tudo por aqui."
      />
      <div className="request-stepper" aria-label={`Etapa ${step} de 3`}>
        {[
          ["Para quem?", "Selecione seus clientes"],
          ["O que precisamos?", "Escolha os documentos"],
          ["Pronto para enviar", "Revise e compartilhe"],
        ].map(([label, desc], i) => (
          <div
            key={label}
            className={`request-step ${step === i + 1 ? "current" : step > i + 1 ? "done" : ""}`}
          >
            <span>{step > i + 1 ? <Check size={14} /> : i + 1}</span>
            <section>
              <strong>{label}</strong>
              <small>{desc}</small>
            </section>
            {i < 2 && <div className="step-line" />}
          </div>
        ))}
      </div>
      <div className="create-request-grid">
        <section className="panel create-main">
          <div className="create-main-header">
            <span className="step-kicker">ETAPA 0{step}</span>
            <h2>
              {step === 1
                ? "Quem vai enviar os documentos?"
                : step === 2
                  ? "O que você precisa receber?"
                  : "Vamos conferir os detalhes?"}
            </h2>
            <p>
              {step === 1
                ? "Escolha um ou mais clientes. Cada um receberá sua própria solicitação."
                : step === 2
                  ? "Comece com um modelo e ajuste apenas o que precisar."
                  : "Defina a competência, o prazo e como compartilhar."}
            </p>
          </div>
          {step === 1 && (
            <>
              <div className="client-selection-tools">
                <div className="search-field">
                  <Search size={15} />
                  <input
                    aria-label="Buscar cliente"
                    placeholder="Buscar por nome ou CNPJ…"
                    value={q}
                    onChange={(e) => setQ(e.target.value)}
                  />
                </div>
                <Button variant="secondary" onClick={() => setAdding(true)}>
                  <Plus size={14} />
                  Novo cliente
                </Button>
              </div>
              <div className="selection-label">
                <span>
                  {clients.filter((c) => c.active).length} clientes ativos
                </span>
                <button
                  onClick={() =>
                    setChosen(
                      chosen.length === clients.filter((c) => c.active).length
                        ? []
                        : clients.filter((c) => c.active).map((c) => c.id),
                    )
                  }
                >
                  {chosen.length ? "Limpar seleção" : "Selecionar todos"}
                </button>
              </div>
              <div className="client-selection-list">
                {clients
                  .filter(
                    (c) =>
                      c.active &&
                      `${c.name} ${c.cnpj}`
                        .toLowerCase()
                        .includes(q.toLowerCase()),
                  )
                  .map((c) => (
                    <label
                      key={c.id}
                      className={`client-selection-item ${chosen.includes(c.id) ? "selected" : ""}`}
                    >
                      <input
                        type="checkbox"
                        checked={chosen.includes(c.id)}
                        onChange={() =>
                          setChosen((p) =>
                            p.includes(c.id)
                              ? p.filter((id) => id !== c.id)
                              : [...p, c.id],
                          )
                        }
                      />
                      <Avatar name={c.name} />
                      <span>
                        <strong>{c.name}</strong>
                        <small>{c.contactName || c.cnpj || c.email}</small>
                      </span>
                      {chosen.includes(c.id) && <Check size={15} />}
                    </label>
                  ))}
                {!clients.some((c) => c.active) && (
                  <div className="new-client-empty">
                    <span>
                      <UsersIcon />
                    </span>
                    <h3>Seu primeiro cliente, seu primeiro passo.</h3>
                    <p>Só precisamos do nome da empresa e dos contatos.</p>
                    <Button onClick={() => setAdding(true)}>
                      <Plus size={15} />
                      Cadastrar primeiro cliente
                    </Button>
                  </div>
                )}
                {clients.some((c) => c.active) &&
                  !clients.some(
                    (c) =>
                      c.active &&
                      `${c.name} ${c.cnpj}`
                        .toLowerCase()
                        .includes(q.toLowerCase()),
                  ) && (
                    <p className="selection-empty">
                      Nenhum cliente encontrado para “{q}”.
                    </p>
                  )}
              </div>
            </>
          )}
          {step === 2 && (
            <>
              <div className="template-picks">
                {templates.map((t) => (
                  <button
                    key={t.id}
                    className={`template-pick ${t.id === templateId ? "selected" : ""}`}
                    onClick={() => chooseTemplate(t)}
                  >
                    <FileText size={20} strokeWidth={1.4} />
                    <strong>{t.name}</strong>
                    <small>{t.items.length} documentos</small>
                    {t.id === templateId && (
                      <span className="template-check">
                        <Check size={12} />
                      </span>
                    )}
                  </button>
                ))}
              </div>
              <div className="request-items-editor">
                <div className="editor-label">
                  <span>DOCUMENTOS SOLICITADOS</span>
                  <span>{items.length} itens</span>
                </div>
                {items.map((item, i) => (
                  <div className="request-edit-item" key={item.id}>
                    <span className="item-order mono">
                      {String(i + 1).padStart(2, "0")}
                    </span>
                    <div className="request-edit-fields">
                      <input
                        className="input"
                        aria-label={`Nome do documento ${i + 1}`}
                        value={item.name}
                        placeholder="Nome do documento"
                        maxLength={180}
                        onChange={(e) =>
                          setItems((p) =>
                            p.map((a, n) =>
                              n === i ? { ...a, name: e.target.value } : a,
                            ),
                          )
                        }
                      />
                      <input
                        className="item-instruction"
                        aria-label={`Instrução do documento ${i + 1}`}
                        value={item.instruction}
                        placeholder="Adicionar uma orientação para o cliente (opcional)"
                        maxLength={1000}
                        onChange={(e) =>
                          setItems((p) =>
                            p.map((a, n) =>
                              n === i
                                ? { ...a, instruction: e.target.value }
                                : a,
                            ),
                          )
                        }
                      />
                      <label className="item-required">
                        <input
                          type="checkbox"
                          checked={item.required}
                          onChange={(e) =>
                            setItems((p) =>
                              p.map((a, n) =>
                                n === i
                                  ? { ...a, required: e.target.checked }
                                  : a,
                              ),
                            )
                          }
                        />
                        Obrigatório
                      </label>
                    </div>
                    <div className="item-tools">
                      <button
                        disabled={i === 0}
                        aria-label={`Mover ${item.name} para cima`}
                        onClick={() => move(i, -1)}
                      >
                        <ChevronUp size={13} />
                      </button>
                      <button
                        disabled={i === items.length - 1}
                        aria-label={`Mover ${item.name} para baixo`}
                        onClick={() => move(i, 1)}
                      >
                        <ChevronDown size={13} />
                      </button>
                      <button
                        aria-label={`Remover ${item.name}`}
                        onClick={() =>
                          setItems((p) => p.filter((_, n) => n !== i))
                        }
                      >
                        <Trash2 size={13} />
                      </button>
                    </div>
                  </div>
                ))}
                <button
                  className="add-document"
                  onClick={() =>
                    setItems((p) => [
                      ...p,
                      {
                        id: crypto.randomUUID(),
                        name: "",
                        instruction: "",
                        required: true,
                        maxFiles: 5,
                      },
                    ])
                  }
                >
                  <Plus size={14} />
                  Adicionar documento
                </button>
              </div>
            </>
          )}
          {step === 3 && (
            <div className="send-details">
              <Field label="Nome da solicitação">
                <input
                  className="input"
                  value={title}
                  maxLength={180}
                  onChange={(e) => setTitle(e.target.value)}
                  required
                />
              </Field>
              <div className="field-row">
                <Field label="Competência">
                  <input
                    className="input"
                    type="month"
                    value={competence}
                    onChange={(e) => setCompetence(e.target.value)}
                    required
                  />
                </Field>
                <Field label="Data-limite">
                  <input
                    className="input"
                    type="date"
                    value={dueDate}
                    onChange={(e) => setDueDate(e.target.value)}
                    required
                  />
                </Field>
              </div>
              <div className="channel-label">
                <strong>Como você prefere compartilhar?</strong>
                <span>Você pode usar outros canais depois.</span>
              </div>
              <div className="channel-options">
                {[
                  {
                    id: "link",
                    icon: Link2,
                    title: "Copiar link",
                    text: "Compartilhe por onde preferir.",
                  },
                  {
                    id: "whatsapp",
                    icon: MessageCircle,
                    title: "WhatsApp",
                    text: "Você confirma o envio no aplicativo.",
                  },
                  {
                    id: "email",
                    icon: Mail,
                    title: "E-mail",
                    text: "Enviamos pelo provedor configurado.",
                  },
                ].map((c) => (
                  <label
                    className={`channel-option ${channel === c.id ? "selected" : ""}`}
                    key={c.id}
                  >
                    <input
                      type="radio"
                      name="channel"
                      value={c.id}
                      checked={channel === c.id}
                      onChange={() => setChannel(c.id)}
                    />
                    <c.icon size={18} />
                    <span>
                      <strong>{c.title}</strong>
                      <small>{c.text}</small>
                    </span>
                  </label>
                ))}
              </div>
              <label className="sensitive-option">
                <input
                  type="checkbox"
                  checked={sensitive}
                  onChange={(e) => setSensitive(e.target.checked)}
                />
                <span>
                  <strong>Exigir código de acesso por e-mail</strong>
                  <small>
                    Para documentos pessoais ou sensíveis. O responsável
                    confirma a identidade antes de acessar.
                  </small>
                </span>
              </label>
              <div className="send-note">
                <ShieldCheck size={17} />
                <p>
                  O link é individual e temporário. Seu cliente acessa sem
                  cadastro e pode continuar um envio depois.
                </p>
              </div>
            </div>
          )}
          {error && (
            <div className="create-inline-error inline-error" role="alert">
              {error}
            </div>
          )}
          <div className="create-main-footer">
            <span>
              {step === 1
                ? `${chosen.length} ${chosen.length === 1 ? "cliente selecionado" : "clientes selecionados"}`
                : step === 2
                  ? `${items.filter((i) => i.required).length} documentos obrigatórios`
                  : "Tudo conferido? Pode criar."}
            </span>
            <div>
              {step > 1 && (
                <Button
                  variant="ghost"
                  onClick={() => {
                    setStep((s) => s - 1);
                    setError("");
                  }}
                >
                  <ArrowLeft size={14} />
                  Voltar
                </Button>
              )}
              {step < 3 ? (
                <Button onClick={next}>
                  Continuar
                  <ArrowRight size={14} />
                </Button>
              ) : (
                <Button
                  loading={create.isPending}
                  onClick={() => {
                    if (!title.trim() || !dueDate || !competence) {
                      setError("Preencha o nome, a competência e o prazo.");
                      return;
                    }
                    setError("");
                    create.mutate();
                  }}
                >
                  <Send size={14} />
                  Criar solicitação{chosen.length > 1 ? "s" : ""}
                </Button>
              )}
            </div>
          </div>
        </section>
        <aside className="create-summary">
          <span className="eyebrow">SUA SOLICITAÇÃO</span>
          <h3>
            Um pedido organizado.
            <br />
            Um fechamento mais leve.
          </h3>
          <div className="summary-preview">
            <div>
              <UsersIcon />
              <span>Clientes</span>
              <strong>
                {chosen.length ? String(chosen.length).padStart(2, "0") : "—"}
              </strong>
            </div>
            <div>
              <FileText size={15} />
              <span>Documentos por cliente</span>
              <strong>
                {items.length ? String(items.length).padStart(2, "0") : "—"}
              </strong>
            </div>
            <div>
              <CalendarDays size={15} />
              <span>Competência</span>
              <strong className="summary-month">{monthName(competence)}</strong>
            </div>
          </div>
          {selectedClients.length > 0 && (
            <div className="summary-clients">
              {selectedClients.slice(0, 4).map((c) => (
                <span key={c.id}>
                  <Avatar name={c.name} />
                  {c.name}
                </span>
              ))}
              {selectedClients.length > 4 && (
                <small>+ {selectedClients.length - 4} clientes</small>
              )}
            </div>
          )}
          <div className="summary-tip">
            <Repeat2 size={16} />
            <p>
              Você poderá repetir este pedido todo mês usando uma recorrência.
            </p>
          </div>
        </aside>
      </div>
      {adding && (
        <ClientEditor
          client={null}
          onClose={() => setAdding(false)}
          onSaved={(c) => {
            setChosen((p) => [...p, c.id]);
            setAdding(false);
          }}
        />
      )}
    </div>
  );
}
function UsersIcon() {
  return (
    <svg
      width="16"
      height="16"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
    >
      <path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2m20 0v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75" />
      <circle cx="9" cy="7" r="4" />
    </svg>
  );
}
