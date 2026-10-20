import { useEffect, useState } from "react";
import { NavLink, Outlet, useLocation, useNavigate } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import {
  Home,
  Files,
  Users,
  Layers,
  Settings2,
  Repeat2,
  ChevronsUpDown,
  Search,
  Bell,
  HelpCircle,
  ArrowUpRight,
  ChevronRight,
  Menu,
  X,
  LogOut,
  Plus,
  Building2,
  ArrowRight,
} from "lucide-react";
import { Logo, Avatar, Modal, EmptyState, Button } from "./shared";
import { CommunicationHistory } from "./CommunicationHistory";
import { latestFailures, type Delivery } from "@/lib/notifications";
import { useSession } from "@/lib/session";
import { api } from "@/lib/api";
import type { Client, DocumentRequest } from "@/lib/types";
import { relativeDate, overdue } from "@/lib/format";

const links = [
  { path: "/app", name: "Início", icon: Home, end: true },
  { path: "/app/solicitacoes", name: "Solicitações", icon: Files },
  { path: "/app/clientes", name: "Clientes", icon: Users },
  { path: "/app/modelos", name: "Modelos", icon: Layers },
];
export function Shell() {
  const { session, onLogout } = useSession();
  const navigate = useNavigate();
  const location = useLocation();
  const [mobile, setMobile] = useState(false);
  const [search, setSearch] = useState(false);
  const [help, setHelp] = useState(false);
  const [bell, setBell] = useState(false);
  const [communications, setCommunications] = useState(false);
  const { data: deliveries = [] } = useQuery({
    queryKey: ["notifications"],
    queryFn: () => api<Delivery[]>("/notifications"),
  });
  const failureCount = latestFailures(deliveries).length;
  const [profile, setProfile] = useState(false);
  const [workspace, setWorkspace] = useState(false);
  const { data: requests = [] } = useQuery({
    queryKey: ["requests"],
    queryFn: () => api<DocumentRequest[]>("/requests"),
  });
  const reviewCount = requests.reduce(
    (a, r) => a + r.items.filter((i) => i.status === "uploaded").length,
    0,
  );
  const current =
    links.find((l) =>
      l.end
        ? location.pathname === l.path
        : location.pathname.startsWith(l.path),
    )?.name ||
    (location.pathname.includes("recorrencias")
      ? "Recorrências"
      : "Configurações");
  useEffect(() => {
    setMobile(false);
    setBell(false);
  }, [location.pathname]);
  useEffect(() => {
    const key = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        setBell(false);
        setProfile(false);
        setMobile(false);
      }
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setSearch((s) => !s);
      }
    };
    window.addEventListener("keydown", key);
    return () => window.removeEventListener("keydown", key);
  }, []);
  return (
    <div className="app-shell">
      <a className="skip-link" href="#main-content">
        Pular para o conteúdo
      </a>
      {mobile && (
        <button
          className="mobile-backdrop"
          aria-label="Fechar navegação"
          onClick={() => setMobile(false)}
        />
      )}
      <aside className={`sidebar ${mobile ? "open" : ""}`}>
        <NavLink to="/app" className="brand-link" aria-label="DocPing — início">
          <Logo />
        </NavLink>
        <button className="workspace-switch" onClick={() => setWorkspace(true)}>
          <span className="workspace-avatar">
            <Building2 size={17} />
          </span>
          <span>
            <strong>{session.organization.name}</strong>
            <small>Seu espaço de trabalho</small>
          </span>
          <ChevronsUpDown size={14} />
        </button>
        <div className="nav-section-label">PRINCIPAL</div>
        <nav aria-label="Menu principal" className="main-nav">
          {links.map((l) => (
            <NavLink
              key={l.path}
              to={l.path}
              end={l.end}
              className={({ isActive }) =>
                `nav-item ${isActive ? "active" : ""}`
              }
            >
              <l.icon size={18} strokeWidth={1.7} />
              <span>{l.name}</span>
              {l.name === "Solicitações" && reviewCount > 0 && (
                <span className="nav-count">{reviewCount}</span>
              )}
            </NavLink>
          ))}
        </nav>
        <div className="nav-section-label organization-label">ORGANIZAÇÃO</div>
        <nav aria-label="Organização" className="main-nav">
          <NavLink
            to="/app/recorrencias"
            className={({ isActive }) => `nav-item ${isActive ? "active" : ""}`}
          >
            <Repeat2 size={18} strokeWidth={1.7} />
            <span>Recorrências</span>
            <span className="new-dot" />
          </NavLink>
          <NavLink
            to="/app/configuracoes"
            className={({ isActive }) => `nav-item ${isActive ? "active" : ""}`}
          >
            <Settings2 size={18} strokeWidth={1.7} />
            <span>Configurações</span>
          </NavLink>
        </nav>
        <div className="sidebar-bottom">
          <div className="sidebar-tip">
            <span className="tip-topline">
              <span className="tiny-leaf">
                <Repeat2 size={16} />
              </span>
              Menos trabalho todo mês.
            </span>
            <p>Deixe suas solicitações prontas para o próximo fechamento.</p>
            <button onClick={() => navigate("/app/recorrencias")}>
              Conhecer recorrências
              <ArrowUpRight size={14} />
            </button>
          </div>
          <button className="sidebar-help" onClick={() => setHelp(true)}>
            <HelpCircle size={17} />
            Ajuda e primeiros passos
            <ArrowUpRight size={13} />
          </button>
          <div className="profile-container">
            <button
              className="profile-button"
              onClick={() => setProfile((p) => !p)}
              aria-expanded={profile}
            >
              <Avatar name={session.user.name} color="sand" />
              <span>
                <strong>{session.user.name}</strong>
                <small>Meu escritório</small>
              </span>
              <ChevronsUpDown size={14} />
            </button>
            {profile && (
              <div className="profile-menu">
                <span>{session.user.email}</span>
                <button
                  onClick={() => {
                    navigate("/app/configuracoes");
                    setProfile(false);
                  }}
                >
                  <Settings2 size={15} />
                  Meu escritório
                </button>
                <button onClick={() => void onLogout()}>
                  <LogOut size={15} />
                  Sair da conta
                </button>
              </div>
            )}
          </div>
        </div>
        <button
          className="mobile-nav-close icon-button"
          aria-label="Fechar menu"
          onClick={() => setMobile(false)}
        >
          <X size={20} />
        </button>
      </aside>
      <div className="main-shell">
        <header className="topbar">
          <div className="topbar-left">
            <button
              className="icon-button mobile-menu-toggle"
              aria-label="Abrir menu"
              onClick={() => setMobile(true)}
            >
              <Menu size={20} />
            </button>
            <span className="breadcrumb-home">Seu espaço</span>
            <ChevronRight size={13} />
            <span className="breadcrumb-current">{current}</span>
          </div>
          <div className="topbar-actions">
            {(session.organization.demo ||
              session.environment === "development") && (
              <span className="demo-label">
                <i />
                {session.organization.demo ? "Demonstração" : "Ambiente local"}
              </span>
            )}
            <button
              className="topbar-search"
              onClick={() => setSearch(true)}
              aria-label="Buscar no DocPing"
            >
              <Search size={17} />
              <span>Buscar no DocPing</span>
              <kbd>⌘ K</kbd>
            </button>
            <span className="topbar-divider" />
            <button
              className={`icon-button notification-trigger ${reviewCount || failureCount ? "has-notifications" : ""}`}
              aria-label={`Notificações: ${reviewCount} documentos para revisar${failureCount ? `, ${failureCount} falhas de e-mail` : ""}`}
              aria-expanded={bell}
              aria-controls="notifications-panel"
              onClick={() => setBell(!bell)}
            >
              <Bell size={19} />
            </button>
            <button
              className="icon-button topbar-help"
              aria-label="Abrir ajuda"
              onClick={() => setHelp(true)}
            >
              <HelpCircle size={19} />
            </button>
          </div>
          {bell && (
            <div className="notification-popover" id="notifications-panel">
              <div className="popover-title">
                <h3>Precisam da sua atenção</h3>
                <button
                  className="icon-button"
                  onClick={() => setBell(false)}
                  aria-label="Fechar notificações"
                >
                  <X size={16} />
                </button>
              </div>
              {requests
                .filter(
                  (r) =>
                    r.status === "under_review" ||
                    r.status === "correction_required" ||
                    overdue(r),
                )
                .slice(0, 4)
                .map((r) => (
                  <button
                    className="notification-row"
                    key={r.id}
                    onClick={() => navigate(`/app/solicitacoes/${r.id}`)}
                  >
                    <Avatar name={r.clientName} />
                    <span>
                      <strong>{r.clientName}</strong>
                      <small>
                        {r.items.some((i) => i.status === "uploaded")
                          ? "Novos documentos para revisar"
                          : overdue(r)
                            ? "Prazo de entrega vencido"
                            : "Aguardando uma correção"}{" "}
                        · {relativeDate(r.updatedAt)}
                      </small>
                    </span>
                    <ChevronRight size={15} />
                  </button>
                ))}
              {!requests.some(
                (r) =>
                  r.status === "under_review" ||
                  r.status === "correction_required" ||
                  overdue(r),
              ) && <p className="popover-empty">Tudo em dia por aqui.</p>}
              <button
                className="communication-history-link"
                onClick={() => {
                  setBell(false);
                  setCommunications(true);
                }}
              >
                {failureCount ? `${failureCount} falha(s) de e-mail · ` : ""}
                Histórico de comunicações
                <ArrowRight size={13} />
              </button>
            </div>
          )}
        </header>
        <main id="main-content" className="main-content">
          <Outlet />
        </main>
        <footer className="app-footer">
          <span>
            <i />
            Um documento de cada vez. Um fechamento mais tranquilo.
          </span>
          <span>Feito para a rotina do seu escritório.</span>
        </footer>
      </div>
      {communications && (
        <CommunicationHistory onClose={() => setCommunications(false)} />
      )}
      {search && (
        <SearchDialog requests={requests} onClose={() => setSearch(false)} />
      )}{" "}
      {workspace && (
        <Modal
          title="Seu espaço de trabalho"
          description="Todos os documentos e clientes deste escritório ficam neste espaço."
          onClose={() => setWorkspace(false)}
        >
          <div className="workspace-modal">
            <Building2 size={30} />
            <h3>{session.organization.name}</h3>
            <p className="muted">
              {session.organization.demo
                ? "Ambiente de demonstração com empresas e documentos fictícios."
                : "Plano " +
                  (session.organization.plan === "founder_pilot"
                    ? "Piloto fundador"
                    : session.organization.plan)}
            </p>
            <Button
              onClick={() => {
                setWorkspace(false);
                navigate("/app/configuracoes");
              }}
            >
              Configurar escritório
              <ArrowRight size={16} />
            </Button>
          </div>
        </Modal>
      )}
      {help && (
        <Modal
          title="Um fechamento mais leve começa aqui"
          description="Quatro passos. Todos os documentos no mesmo lugar."
          onClose={() => setHelp(false)}
        >
          <div className="help-steps">
            {[
              [
                "Solicite",
                "Escolha os clientes, um modelo de documentos e o prazo.",
              ],
              [
                "Compartilhe",
                "Copie o link ou abra o WhatsApp com uma mensagem pronta.",
              ],
              [
                "Confira",
                "O cliente envia pelo celular. Você aprova ou pede uma correção.",
              ],
              [
                "Conclua",
                "Com os itens obrigatórios aprovados, a solicitação é concluída.",
              ],
            ].map(([title, desc], i) => (
              <div key={title}>
                <span>{i + 1}</span>
                <section>
                  <h3>{title}</h3>
                  <p>{desc}</p>
                </section>
              </div>
            ))}
          </div>
          <Button
            onClick={() => {
              setHelp(false);
              navigate("/app/solicitacoes/nova");
            }}
          >
            <Plus size={16} />
            Criar uma solicitação
          </Button>
        </Modal>
      )}
    </div>
  );
}

function SearchDialog({
  requests,
  onClose,
}: {
  requests: DocumentRequest[];
  onClose: () => void;
}) {
  const [q, setQ] = useState("");
  const navigate = useNavigate();
  const { data: clients = [] } = useQuery({
    queryKey: ["clients"],
    queryFn: () => api<Client[]>("/clients"),
  });
  const query = q.toLowerCase();
  const matched = requests
    .filter((r) => `${r.clientName} ${r.title}`.toLowerCase().includes(query))
    .slice(0, 5);
  const matches = clients
    .filter((c) => `${c.name} ${c.cnpj}`.toLowerCase().includes(query))
    .slice(0, 3);
  const go = (url: string) => {
    onClose();
    navigate(url);
  };
  return (
    <Modal title="Buscar no DocPing" onClose={onClose}>
      <div className="search-input-wrap">
        <Search size={19} />
        <input
          className="input"
          aria-label="Buscar cliente, CNPJ ou solicitação"
          placeholder="Cliente, CNPJ ou solicitação…"
          value={q}
          onChange={(e) => setQ(e.target.value)}
          autoFocus
        />
      </div>
      <div className="command-results">
        <button
          className="command-new"
          onClick={() => go("/app/solicitacoes/nova")}
        >
          <Plus size={18} />
          Nova solicitação<kbd>↵</kbd>
        </button>
        <p className="eyebrow">SOLICITAÇÕES</p>
        {matched.map((r) => (
          <button key={r.id} onClick={() => go(`/app/solicitacoes/${r.id}`)}>
            <Files size={17} />
            <span>
              <strong>{r.clientName}</strong>
              <small>{r.title}</small>
            </span>
            <ChevronRight size={16} />
          </button>
        ))}
        <p className="eyebrow">CLIENTES</p>
        {matches.map((c) => (
          <button
            key={c.id}
            onClick={() =>
              go(`/app/clientes?busca=${encodeURIComponent(c.name)}`)
            }
          >
            <Avatar name={c.name} />
            <span>
              <strong>{c.name}</strong>
              <small>{c.cnpj}</small>
            </span>
            <ChevronRight size={16} />
          </button>
        ))}
        {!matched.length && !matches.length && (
          <EmptyState
            title="Nenhum resultado"
            description="Tente outro nome de cliente ou CNPJ."
          />
        )}
      </div>
    </Modal>
  );
}
