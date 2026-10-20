import { useState } from "react";
import { useNavigate, Link } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import {
  Plus,
  ArrowRight,
  ArrowUpRight,
  CalendarDays,
  ChevronDown,
  Users,
  FileCheck2,
  CheckCheck,
  Clock3,
  Check,
  Files,
  ChevronRight,
  ArrowDownToLine,
  Repeat2,
  Search,
  SlidersHorizontal,
} from "lucide-react";
import { useSession } from "@/lib/session";
import { api } from "@/lib/api";
import type { DocumentRequest, Client } from "@/lib/types";
import {
  Button,
  Avatar,
  StatusBadge,
  PageHeading,
  Loading,
  ErrorState,
  EmptyState,
} from "@/components/shared";
import { AnimatedTabs } from "@/components/ui/animated-tabs";
import {
  dateLabel,
  daysLate,
  monthName,
  overdue,
  progress,
  relativeDate,
  shortMonth,
  statusLabels,
  today,
} from "@/lib/format";
import { CommunicationAlert } from "@/components/CommunicationAlert";
import "./dashboard.css";

export function Dashboard() {
  const { session } = useSession();
  const navigate = useNavigate();
  const {
    data: requests = [],
    isLoading,
    error,
    refetch,
  } = useQuery({
    queryKey: ["requests"],
    queryFn: () => api<DocumentRequest[]>("/requests"),
  });
  const [month, setMonth] = useState("");
  const months = [...new Set(requests.map((r) => r.competence))]
    .sort()
    .reverse();
  const activeMonth = month || months[0];
  const selected = activeMonth
    ? requests.filter((r) => r.competence === activeMonth)
    : requests;
  const review = selected.reduce(
    (a, r) => a + r.items.filter((i) => i.status === "uploaded").length,
    0,
  );
  const pending = selected.filter(
    (r) => r.status !== "completed" && r.status !== "cancelled",
  );
  const late = pending.filter(overdue);
  const completed = selected.filter((r) => r.status === "completed");
  const deadlines = pending
    .map((r) => r.dueDate)
    .filter((d) => d >= today())
    .sort();
  const attention = [...pending]
    .sort((a, b) => {
      const priority = (r: DocumentRequest) =>
        r.items.some((i) => i.status === "uploaded")
          ? 0
          : overdue(r)
            ? 1
            : r.status === "correction_required"
              ? 2
              : 3;
      return priority(a) - priority(b);
    })
    .slice(0, 3);
  const hour = Number(
    new Intl.DateTimeFormat("pt-BR", {
      hour: "numeric",
      hour12: false,
      timeZone: "America/Sao_Paulo",
    }).format(new Date()),
  );
  const greeting =
    hour < 12 ? "Bom dia" : hour < 18 ? "Boa tarde" : "Boa noite";
  const date = new Intl.DateTimeFormat("pt-BR", {
    weekday: "long",
    day: "numeric",
    month: "long",
    timeZone: "America/Sao_Paulo",
  }).format(new Date());
  const pct = selected.length
    ? Math.round((completed.length / selected.length) * 100)
    : 0;
  const [tableFilter, setTableFilter] = useState("all");
  const recent = [...selected].sort((a, b) =>
    b.updatedAt.localeCompare(a.updatedAt),
  );
  const table = recent
    .filter(
      (r) =>
        tableFilter === "all" ||
        (tableFilter === "review" &&
          r.items.some((i) => i.status === "uploaded")) ||
        (tableFilter === "late" && overdue(r)),
    )
    .slice(0, 5);
  if (isLoading) return <Loading />;
  if (error)
    return (
      <ErrorState message={error.message} onRetry={() => void refetch()} />
    );
  return (
    <div className="dashboard">
      <PageHeading
        eyebrow="UM POUCO MAIS DE CLAREZA. UM DIA MAIS LEVE."
        title={`${greeting}, ${session.user.name.split(" ")[0]}.`}
        description={
          requests.length
            ? `Você tem ${review} documentos para revisar e ${pending.length} clientes com pendências.`
            : "Tudo começa com a sua primeira solicitação."
        }
        action={
          <>
            <span className="today-label">
              <CalendarDays size={14} />
              {date.charAt(0).toUpperCase() +
                date.slice(1).replace("-feira", "")}
            </span>
            <Button onClick={() => navigate("/app/solicitacoes/nova")}>
              <Plus size={16} />
              Nova solicitação
            </Button>
          </>
        }
      />
      <CommunicationAlert />
      <div className="dashboard-section-label">
        <span>Visão do seu escritório</span>
        <div className="month-select">
          <CalendarDays size={13} />
          <select
            aria-label="Competência do painel"
            value={activeMonth || ""}
            onChange={(e) => setMonth(e.target.value)}
          >
            {months.length ? (
              months.map((m) => (
                <option key={m} value={m}>
                  {monthName(m)}
                </option>
              ))
            ) : (
              <option value="">Este mês</option>
            )}
          </select>
          <ChevronDown size={12} />
        </div>
      </div>
      <section className="metrics-grid" aria-label="Resumo do fechamento">
        <Metric
          icon={Users}
          label="Clientes com pendências"
          value={String(pending.length).padStart(2, "0")}
          detail={
            late.length
              ? `${late.length} com prazo vencido`
              : "Todos dentro do prazo"
          }
          tone={late.length ? "amber" : "normal"}
          onClick={() => navigate("/app/solicitacoes?filtro=pending")}
        />
        <Metric
          icon={FileCheck2}
          label="Aguardando revisão"
          value={String(review).padStart(2, "0")}
          detail="Documentos prontos para conferir"
          tone="blue"
          onClick={() => navigate("/app/solicitacoes?filtro=review")}
        />
        <Metric
          icon={CheckCheck}
          label="Solicitações concluídas"
          value={String(completed.length).padStart(2, "0")}
          detail={
            selected.length
              ? `${completed.length} de ${selected.length} neste fechamento`
              : "Ainda não há solicitações"
          }
          tone="green"
          onClick={() => navigate("/app/solicitacoes?filtro=completed")}
        />
        <Metric
          icon={Clock3}
          label="Próximo prazo"
          value={deadlines.length ? dateLabel(deadlines[0]) : "—"}
          detail={
            deadlines.length
              ? `${pending.filter((r) => r.dueDate === deadlines[0]).length} solicitações para esta data`
              : "Nenhuma entrega programada"
          }
          tone="normal"
          onClick={() => navigate("/app/solicitacoes")}
        />
      </section>
      <div className="dashboard-work-grid">
        <section className="panel attention-panel">
          <div className="panel-title">
            <div>
              <h2>
                Precisam da sua atenção
                <span className="attention-number">{pending.length}</span>
              </h2>
              <p>O que importa agora, em um só lugar.</p>
            </div>
            <span className="attention-heading-dot">
              <span />
              PRIORIDADES
            </span>
          </div>
          <div className="attention-list">
            {attention.map((r) => {
              const count = r.items.filter(
                (i) => i.status === "uploaded",
              ).length;
              const correction = r.items.filter(
                (i) => i.status === "correction_required",
              ).length;
              return (
                <div className="attention-row" key={r.id}>
                  <Avatar name={r.clientName} />
                  <div className="attention-row-info">
                    <h3>{r.clientName}</h3>
                    <p>
                      {count ? (
                        <>
                          <span className="activity-indicator blue" />
                          {count}{" "}
                          {count === 1
                            ? "documento recebido"
                            : "documentos recebidos"}
                          <span className="dot-divider">·</span>Prontos para
                          revisão
                        </>
                      ) : overdue(r) ? (
                        <>
                          <span className="activity-indicator amber" />
                          Prazo vencido há {daysLate(r)} dias
                          <span className="dot-divider">·</span>
                          {
                            r.items.filter((i) => i.status === "pending").length
                          }{" "}
                          pendentes
                        </>
                      ) : correction ? (
                        <>
                          <span className="activity-indicator amber" />
                          {correction}{" "}
                          {correction === 1
                            ? "documento precisa"
                            : "documentos precisam"}{" "}
                          de correção
                        </>
                      ) : (
                        <>
                          <span className="activity-indicator" />
                          Aguardando envio<span className="dot-divider">·</span>
                          Até {dateLabel(r.dueDate)}
                        </>
                      )}
                    </p>
                  </div>
                  <button
                    className={`attention-action ${count ? "review" : ""}`}
                    onClick={() => navigate(`/app/solicitacoes/${r.id}`)}
                  >
                    {count
                      ? "Revisar"
                      : overdue(r)
                        ? "Lembrar cliente"
                        : "Ver solicitação"}
                    <ArrowUpRight size={13} />
                  </button>
                </div>
              );
            })}
            {!attention.length && (
              <div className="all-clear">
                <Check size={23} />
                <div>
                  <h3>Tudo em dia. Pode respirar.</h3>
                  <p>Novos envios e pendências aparecerão aqui.</p>
                </div>
              </div>
            )}
          </div>
          <Link className="attention-footer" to="/app/solicitacoes">
            Ver todas as pendências
            <ArrowRight size={13} />
          </Link>
        </section>
        <section className="closing-panel">
          <div className="closing-header">
            <span className="closing-eyebrow">
              <span />
              VISÃO DO FECHAMENTO
            </span>
            <Repeat2 size={16} />
          </div>
          <div className="closing-summary">
            <div>
              <h2>
                {activeMonth
                  ? monthName(activeMonth).replace(" de ", " ")
                  : "Seu próximo mês"}
              </h2>
              <p>
                Um passo de cada vez.
                <br />
                Tudo caminhando para fechar.
              </p>
            </div>
            <div className="closing-ring">
              <svg viewBox="0 0 76 76" aria-hidden="true">
                <circle
                  cx="38"
                  cy="38"
                  r="32"
                  fill="none"
                  stroke="#d7e3cf"
                  strokeWidth="5"
                />
                <circle
                  cx="38"
                  cy="38"
                  r="32"
                  fill="none"
                  stroke="#6f9260"
                  strokeWidth="5"
                  strokeLinecap="round"
                  strokeDasharray={`${pct * 2.01} 201`}
                  transform="rotate(-90 38 38)"
                />
              </svg>
              <span className="mono">
                {pct}
                <small>%</small>
              </span>
            </div>
          </div>
          <div className="closing-legend">
            <div>
              <span className="legend-dot complete" />
              Concluídas
              <strong className="mono">
                {String(completed.length).padStart(2, "0")}
              </strong>
            </div>
            <div>
              <span className="legend-dot review" />
              Em revisão
              <strong className="mono">
                {String(
                  selected.filter((r) =>
                    r.items.some((i) => i.status === "uploaded"),
                  ).length,
                ).padStart(2, "0")}
              </strong>
            </div>
            <div>
              <span className="legend-dot pending" />
              Aguardando cliente
              <strong className="mono">
                {String(
                  pending.filter(
                    (r) => !r.items.some((i) => i.status === "uploaded"),
                  ).length,
                ).padStart(2, "0")}
              </strong>
            </div>
          </div>
          <button
            className="closing-footer"
            onClick={() => navigate("/app/solicitacoes?filtro=completed")}
          >
            <CheckCheck size={14} />
            Acompanhar o fechamento
            <ArrowUpRight size={13} />
          </button>
        </section>
      </div>
      <section className="panel recent-panel">
        <div className="panel-title">
          <div>
            <h2>Solicitações recentes</h2>
            <p>Do primeiro pedido ao último documento aprovado.</p>
          </div>
          <button
            className="text-link"
            onClick={() => navigate("/app/solicitacoes")}
          >
            Ver todas
            <ArrowRight size={13} />
          </button>
        </div>
        <div className="recent-toolbar">
          <AnimatedTabs
            tabs={[
              {
                id: "all",
                label: "Todas",
                badge: selected.length || undefined,
              },
              {
                id: "review",
                label: "Para revisar",
                badge: review || undefined,
              },
              {
                id: "late",
                label: "Atrasadas",
                badge: late.length || undefined,
              },
            ]}
            activeTab={tableFilter}
            onChange={setTableFilter}
            renderContent={false}
            className="request-tabs"
          />
          <span className="table-updated">
            Atualizado com os últimos envios
          </span>
        </div>
        {!table.length ? (
          <EmptyState
            title={
              requests.length
                ? "Nenhuma solicitação neste filtro"
                : "Seu primeiro fechamento começa aqui"
            }
            description={
              requests.length
                ? "Seus clientes estão em dia nesta categoria."
                : "Cadastre um cliente e escolha um modelo para começar."
            }
            action={
              !requests.length ? (
                <Button onClick={() => navigate("/app/solicitacoes/nova")}>
                  <Plus size={15} />
                  Criar solicitação
                </Button>
              ) : undefined
            }
          />
        ) : (
          <RequestTable requests={table} />
        )}
        <div className="table-bottom">
          <span>
            {table.length
              ? `Mostrando ${table.length} de ${selected.length} solicitações`
              : "Nenhuma solicitação"}
          </span>
          <span className="subtle-safe">
            <span />
            Organizado. Do início ao fim.
          </span>
        </div>
      </section>
      <div className="dashboard-bottom-note">
        <span className="mini-spark">
          <Check size={12} />
        </span>
        <span>
          Mais tempo para o que importa.
          <strong> Menos tempo procurando documentos.</strong>
        </span>
        <Link to="/app/modelos">
          Explorar modelos
          <ArrowUpRight size={12} />
        </Link>
      </div>
    </div>
  );
}

function Metric({
  icon: Icon,
  label,
  value,
  detail,
  tone,
  onClick,
}: {
  icon: typeof Users;
  label: string;
  value: string;
  detail: string;
  tone: string;
  onClick: () => void;
}) {
  return (
    <button className={`metric ${tone}`} onClick={onClick}>
      <div className="metric-label">
        {label}
        <Icon size={15} strokeWidth={1.5} />
      </div>
      <div
        className={`metric-value ${value.includes(" ") ? "metric-date" : ""}`}
      >
        {value}
      </div>
      <p>
        {tone === "amber" && <span className="metric-detail-dot" />}
        {detail}
      </p>
    </button>
  );
}

export function RequestTable({ requests }: { requests: DocumentRequest[] }) {
  const navigate = useNavigate();
  return (
    <div className="table-scroll">
      <table className="request-table">
        <thead>
          <tr>
            <th>Cliente</th>
            <th>Competência</th>
            <th>Progresso</th>
            <th>Prazo</th>
            <th>Status</th>
            <th className="activity-col">Última atividade</th>
            <th>
              <span className="sr-only">Abrir</span>
            </th>
          </tr>
        </thead>
        <tbody>
          {requests.map((r) => {
            const p = progress(r);
            return (
              <tr key={r.id}>
                <td>
                  <Link
                    to={`/app/solicitacoes/${r.id}`}
                    className="table-client"
                  >
                    <Avatar name={r.clientName} />
                    <span>
                      <strong>{r.clientName}</strong>
                      <small>{r.title}</small>
                    </span>
                  </Link>
                </td>
                <td>
                  <span className="competence-text">
                    {shortMonth(r.competence)}
                  </span>
                  {r.recurring && (
                    <Repeat2
                      size={10}
                      className="recurring-icon"
                      aria-label="Recorrente"
                    />
                  )}
                </td>
                <td>
                  <div
                    className={`progress-cell ${p.percent === 100 ? "done" : ""}`}
                  >
                    <div className="mini-progress">
                      <span style={{ width: `${p.percent}%` }} />
                    </div>
                    <span className="mono">
                      {p.approved}
                      <small>/{p.total}</small>
                    </span>
                  </div>
                </td>
                <td>
                  <div className={`deadline-cell ${overdue(r) ? "late" : ""}`}>
                    {overdue(r) && <span />}
                    {dateLabel(r.dueDate)}
                    {overdue(r) && <small>{daysLate(r)}d atraso</small>}
                  </div>
                </td>
                <td>
                  <StatusBadge status={r.status} />
                </td>
                <td className="activity-col">
                  <span className="last-activity">
                    {relativeDate(r.updatedAt)}
                  </span>
                </td>
                <td>
                  <button
                    className="table-open icon-button"
                    onClick={() => navigate(`/app/solicitacoes/${r.id}`)}
                    aria-label={`Abrir solicitação de ${r.clientName}`}
                  >
                    <ChevronRight size={15} />
                  </button>
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

export function RequestsPage() {
  const navigate = useNavigate();
  const [params, setParams] = useState(
    () => new URLSearchParams(window.location.search),
  );
  const [q, setQ] = useState("");
  const [filter, setFilter] = useState(params.get("filtro") || "all");
  const [sort, setSort] = useState("activity");
  const [month, setMonth] = useState("all");
  const {
    data: requests = [],
    isLoading,
    error,
    refetch,
  } = useQuery({
    queryKey: ["requests"],
    queryFn: () => api<DocumentRequest[]>("/requests"),
  });
  const { data: clients = [] } = useQuery({
    queryKey: ["clients"],
    queryFn: () => api<Client[]>("/clients"),
  });
  const filtered = requests
    .filter(
      (r) =>
        (`${r.clientName} ${r.title}`.toLowerCase().includes(q.toLowerCase()) ||
          (
            clients.find((c) => c.id === r.clientId)?.cnpj.toLowerCase() || ""
          ).includes(q.toLowerCase().replace(/[.\/-]/g, ""))) &&
        (!params.get("client") || r.clientId === params.get("client")) &&
        (month === "all" || r.competence === month) &&
        (filter === "all" ||
          (filter === "pending" &&
            ["pending", "in_progress"].includes(r.status)) ||
          (filter === "review" &&
            r.items.some((i) => i.status === "uploaded")) ||
          (filter === "correction" && r.status === "correction_required") ||
          (filter === "late" && overdue(r)) ||
          (filter === "completed" && r.status === "completed")),
    )
    .sort((a, b) =>
      sort === "deadline"
        ? a.dueDate.localeCompare(b.dueDate)
        : b.updatedAt.localeCompare(a.updatedAt),
    );
  const months = [...new Set(requests.map((r) => r.competence))]
    .sort()
    .reverse();
  if (isLoading) return <Loading />;
  if (error)
    return (
      <ErrorState message={error.message} onRetry={() => void refetch()} />
    );
  return (
    <div className="requests-page">
      <PageHeading
        eyebrow="SOLICITAR. RECEBER. CONFERIR. CONCLUIR."
        title="Solicitações"
        description="Cada cliente, cada documento, cada próximo passo."
        action={
          <Button onClick={() => navigate("/app/solicitacoes/nova")}>
            <Plus size={16} />
            Nova solicitação
          </Button>
        }
      />
      <section className="panel">
        <div className="requests-toolbar">
          <div className="search-field">
            <Search size={16} />
            <input
              aria-label="Buscar solicitações"
              placeholder="Buscar por cliente ou solicitação…"
              value={q}
              onChange={(e) => setQ(e.target.value)}
            />
            {q && (
              <button onClick={() => setQ("")} aria-label="Limpar busca">
                ×
              </button>
            )}
          </div>
          <div className="requests-toolbar-right">
            <label className="small-select">
              <CalendarDays size={14} />
              <select
                aria-label="Filtrar competência"
                value={month}
                onChange={(e) => setMonth(e.target.value)}
              >
                <option value="all">Todas as competências</option>
                {months.map((m) => (
                  <option key={m} value={m}>
                    {monthName(m)}
                  </option>
                ))}
              </select>
              <ChevronDown size={12} />
            </label>
            <label className="small-select">
              <SlidersHorizontal size={14} />
              <select
                aria-label="Ordenar solicitações"
                value={sort}
                onChange={(e) => setSort(e.target.value)}
              >
                <option value="activity">Última atividade</option>
                <option value="deadline">Prazo mais próximo</option>
              </select>
              <ChevronDown size={12} />
            </label>
          </div>
        </div>
        <div className="requests-filter-row">
          <AnimatedTabs
            tabs={[
              {
                id: "all",
                label: "Todas",
                badge: requests.length || undefined,
              },
              { id: "pending", label: "Aguardando cliente" },
              { id: "review", label: "Para revisar" },
              { id: "correction", label: "Com correção" },
              { id: "late", label: "Atrasadas" },
              { id: "completed", label: "Concluídas" },
            ]}
            activeTab={filter}
            onChange={setFilter}
            renderContent={false}
            className="request-tabs"
          />
        </div>
        {params.get("client") && (
          <div className="client-filter-active">
            Filtrando por cliente{" "}
            <button
              onClick={() => {
                setParams(new URLSearchParams());
                window.history.replaceState({}, "", "/app/solicitacoes");
              }}
            >
              Limpar filtro ×
            </button>
          </div>
        )}
        {filtered.length ? (
          <RequestTable requests={filtered} />
        ) : (
          <EmptyState
            title={
              q
                ? "Não encontramos essa solicitação"
                : "Tudo em dia neste filtro"
            }
            description={
              q
                ? "Tente buscar pelo nome da empresa."
                : "As solicitações desta categoria aparecerão aqui."
            }
            action={
              <Button
                variant="secondary"
                onClick={() => {
                  setQ("");
                  setFilter("all");
                  setMonth("all");
                }}
              >
                Ver todas
              </Button>
            }
          />
        )}
        <div className="table-bottom">
          <span>
            {filtered.length} solicitações{q ? " encontradas" : ""}
          </span>
          <span className="subtle-safe">Dados do seu escritório</span>
        </div>
      </section>
      <div className="requests-howto">
        <span className="requests-howto-icon">
          <Files size={19} />
        </span>
        <div>
          <h3>O próximo mês já pode ficar pronto.</h3>
          <p>
            Use uma recorrência para repetir os pedidos sem recadastrar os
            documentos.
          </p>
        </div>
        <Button
          variant="secondary"
          onClick={() => navigate("/app/recorrencias")}
        >
          Configurar recorrência
          <ArrowUpRight size={13} />
        </Button>
      </div>
    </div>
  );
}
