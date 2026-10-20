import { useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  ArrowLeft,
  ArrowRight,
  ArrowUpRight,
  Check,
  CheckCheck,
  Clock3,
  FileText,
  Image,
  Download,
  Link2,
  Copy,
  Mail,
  MessageCircle,
  ShieldCheck,
  MoreHorizontal,
  CalendarDays,
  ExternalLink,
  ChevronRight,
  RotateCcw,
  AlertCircle,
  X,
} from "lucide-react";
import { api, post } from "@/lib/api";
import type {
  Client,
  DocumentRequest,
  RequestItem,
  DocumentFile,
} from "@/lib/types";
import { useToast } from "@/lib/toast";
import { useSession } from "@/lib/session";
import { AnalysisPanel } from "@/components/AnalysisPanel";
import {
  Avatar,
  Button,
  Modal,
  Loading,
  ErrorState,
  StatusBadge,
} from "@/components/shared";
import {
  dateLabel,
  monthName,
  overdue,
  progress,
  relativeDate,
} from "@/lib/format";
import "./request-flow.css";

export function RequestDetail() {
  const { id = "" } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const toast = useToast();
  const { session } = useSession();
  const requestQuery = useQuery({
    queryKey: ["request", id],
    queryFn: () => api<DocumentRequest>(`/requests/${id}`),
  });
  const [selectedId, setSelectedId] = useState("");
  const [fileId, setFileId] = useState("");
  const [correcting, setCorrecting] = useState(false);
  const [reason, setReason] = useState("");
  const [sharing, setSharing] = useState(false);
  const [more, setMore] = useState(false);
  const [cancel, setCancel] = useState(false);
  const [exporting, setExporting] = useState(false);
  const [history, setHistory] = useState(false);
  const r = requestQuery.data;
  const selected =
    r?.items.find((i) => i.id === selectedId) ||
    r?.items.find((i) => i.status === "uploaded") ||
    r?.items[0];
  const activeFiles = selected?.files.filter((f) => !f.superseded) || [];
  const file =
    (history ? selected?.files : activeFiles)?.find((f) => f.id === fileId) ||
    activeFiles[activeFiles.length - 1];
  const analysis = file?.analysis;
  const analyze = useMutation({
    mutationFn: () =>
      post<DocumentRequest>(`/requests/${id}/items/${selected!.id}/analyze`),
    onSuccess: (updated) => {
      queryClient.setQueryData(["request", id], updated);
      toast("Análise solicitada. O resultado aparece aqui em instantes.");
    },
    onError: (e) => toast(e.message, "error"),
  });
  const review = useMutation({
    mutationFn: (action: "approve" | "correct") =>
      post<DocumentRequest>(`/requests/${id}/items/${selected!.id}/review`, {
        action,
        reason: action === "correct" ? reason : undefined,
        fileId:
          action === "correct"
            ? file && !file.superseded
              ? file.id
              : activeFiles[activeFiles.length - 1]?.id
            : undefined,
      }),
    onSuccess: async (updated, action) => {
      queryClient.setQueryData(["request", id], updated);
      await queryClient.invalidateQueries({ queryKey: ["requests"] });
      await queryClient.invalidateQueries({ queryKey: ["notifications"] });
      setCorrecting(false);
      setReason("");
      if (action === "approve") {
        const next = updated.items.find(
          (i) => i.status === "uploaded" && i.id !== selected?.id,
        );
        if (next) {
          setSelectedId(next.id);
          setFileId("");
        }
        toast(
          updated.status === "completed"
            ? "Tudo conferido. Solicitação concluída!"
            : "Documento aprovado.",
        );
      } else
        toast("Correção registrada. O motivo já aparece no portal do cliente.");
    },
    onError: (e) => toast(e.message, "error"),
  });
  const cancelMutation = useMutation({
    mutationFn: () => post<DocumentRequest>(`/requests/${id}/cancel`),
    onSuccess: async (updated) => {
      queryClient.setQueryData(["request", id], updated);
      await queryClient.invalidateQueries({ queryKey: ["requests"] });
      await queryClient.invalidateQueries({ queryKey: ["notifications"] });
      setCancel(false);
      toast("Solicitação cancelada e links revogados.");
    },
    onError: (e) => toast(e.message, "error"),
  });
  async function exportFiles() {
    setExporting(true);
    try {
      const response = await fetch(`/api/requests/${id}/export`);
      if (!response.ok) {
        const data = await response.json();
        throw new Error(data.error?.message || "Não foi possível exportar.");
      }
      const url = URL.createObjectURL(await response.blob());
      const a = document.createElement("a");
      a.href = url;
      a.download = `${r?.clientName}-${r?.competence}.zip`;
      a.click();
      setTimeout(() => URL.revokeObjectURL(url), 1000);
      toast("Arquivos exportados.");
    } catch (e) {
      toast(e instanceof Error ? e.message : "Falha ao exportar.", "error");
    } finally {
      setExporting(false);
    }
  }
  if (requestQuery.isLoading) return <Loading label="Abrindo os documentos…" />;
  if (requestQuery.error || !r)
    return (
      <ErrorState
        message={requestQuery.error?.message || "Solicitação não encontrada."}
        onRetry={() => void requestQuery.refetch()}
      />
    );
  const p = progress(r);
  const uploaded = r.items.filter((i) => i.status === "uploaded").length;
  return (
    <div className="request-detail">
      <Link to="/app/solicitacoes" className="back-link">
        <ArrowLeft size={14} />
        Voltar para solicitações
      </Link>
      <div className="detail-heading">
        <div className="detail-client">
          <Avatar name={r.clientName} />
          <div>
            <p className="eyebrow">{r.title}</p>
            <h1>{r.clientName}</h1>
          </div>
        </div>
        <div className="detail-heading-actions">
          <Button
            variant="secondary"
            loading={exporting}
            disabled={!r.items.some((i) => i.files.some((f) => !f.superseded))}
            onClick={() => void exportFiles()}
          >
            <Download size={14} />
            Exportar
          </Button>
          <Button
            onClick={() => setSharing(true)}
            disabled={r.status === "cancelled"}
          >
            <Link2 size={14} />
            Compartilhar
          </Button>
          <div className="detail-menu-anchor">
            <button
              className="icon-button"
              onClick={() => setMore(!more)}
              aria-label="Mais opções"
              aria-expanded={more}
            >
              <MoreHorizontal size={20} />
            </button>
            {more && (
              <div className="detail-menu">
                <button
                  onClick={() => {
                    setMore(false);
                    setHistory(!history);
                  }}
                >
                  <RotateCcw size={14} />
                  {history ? "Ocultar histórico" : "Ver versões anteriores"}
                </button>
                <button
                  className="danger-text"
                  disabled={
                    r.status === "completed" || r.status === "cancelled"
                  }
                  onClick={() => {
                    setCancel(true);
                    setMore(false);
                  }}
                >
                  <X size={14} />
                  Cancelar solicitação
                </button>
              </div>
            )}
          </div>
        </div>
      </div>
      <div className="detail-meta">
        <span>
          <CalendarDays size={14} />
          {monthName(r.competence)}
        </span>
        <span className={overdue(r) ? "late-text" : ""}>
          <Clock3 size={14} />
          Entrega até {dateLabel(r.dueDate)}
        </span>
        <StatusBadge status={r.status} />
        <div className="detail-progress">
          <span>
            {p.approved} de {p.total} aprovados
          </span>
          <div className="mini-progress">
            <span style={{ width: `${p.percent}%` }} />
          </div>
        </div>
      </div>
      {r.status === "completed" && (
        <div className="completion-banner">
          <CheckCheck size={20} />
          <div>
            <strong>Fechamento com os documentos em dia.</strong>
            <p>
              Todos os documentos obrigatórios foram aprovados. Esta solicitação
              está concluída.
            </p>
          </div>
        </div>
      )}
      {r.status === "cancelled" && (
        <div className="cancelled-banner">
          <AlertCircle size={19} />
          <span>
            Solicitação cancelada. Os links de acesso foram revogados.
          </span>
        </div>
      )}
      <div className="review-layout">
        <aside className="panel review-documents">
          <div className="review-documents-heading">
            <h2>
              Documentos<span>{r.items.length}</span>
            </h2>
            <p>
              {uploaded
                ? `${uploaded} ${uploaded === 1 ? "documento pronto" : "documentos prontos"} para conferir`
                : "Acompanhe os envios por aqui"}
            </p>
          </div>
          <div className="review-documents-list">
            {r.items.map((item, i) => (
              <button
                key={item.id}
                className={`review-document-row ${selected?.id === item.id ? "active" : ""}`}
                onClick={() => {
                  setSelectedId(item.id);
                  setFileId("");
                }}
              >
                <span className={`document-state-icon ${item.status}`}>
                  {item.status === "approved" ? (
                    <Check size={16} />
                  ) : item.status === "correction_required" ? (
                    <RotateCcw size={15} />
                  ) : (
                    <FileText size={17} strokeWidth={1.4} />
                  )}
                </span>
                <span>
                  <strong>{item.name}</strong>
                  <small>
                    {item.status === "pending"
                      ? "Falta enviar"
                      : item.status === "uploaded"
                        ? `${item.files.filter((f) => !f.superseded).length} arquivo(s) · Em análise`
                        : item.status === "approved"
                          ? "Aprovado"
                          : "Correção solicitada"}
                    {!item.required && " · Opcional"}
                  </small>
                </span>
                {selected?.id === item.id && <ChevronRight size={14} />}
              </button>
            ))}
          </div>
          <div className="review-list-footer">
            <ShieldCheck size={13} />
            <p>
              Os arquivos ficam restritos
              <br />
              ao seu escritório e a este cliente.
            </p>
          </div>
        </aside>
        <section className="panel review-viewer">
          {selected && (
            <>
              <div className="viewer-heading">
                <div>
                  <h2>{selected.name}</h2>
                  <p>
                    {selected.instruction ||
                      "Confira se o documento está completo e legível."}
                  </p>
                </div>
                <StatusBadge
                  status={
                    selected.status === "pending" ? "pending" : selected.status
                  }
                />
              </div>
              {(activeFiles.length > 1 ||
                (history && selected.files.some((f) => f.superseded))) && (
                <div className="file-tabs">
                  {(history ? selected.files : activeFiles).map((f) => (
                    <button
                      key={f.id}
                      className={file?.id === f.id ? "active" : ""}
                      onClick={() => setFileId(f.id)}
                    >
                      <FileText size={12} />
                      {f.name}
                      {f.superseded && <small>Anterior</small>}
                    </button>
                  ))}
                </div>
              )}
              {selected.status === "correction_required" && (
                <div className="correction-notice">
                  <RotateCcw size={16} />
                  <div>
                    <strong>Correção solicitada</strong>
                    <p>{selected.correctionReason}</p>
                  </div>
                </div>
              )}
              {file ? (
                <>
                  <div className="file-preview">
                    <FilePreview file={file} />
                  </div>
                  <div className="file-meta-bar">
                    <span>
                      <FileText size={13} />
                      <strong>{file.name}</strong>
                      <small>
                        {formatBytes(file.size)} ·{" "}
                        {relativeDate(file.createdAt)}
                      </small>
                    </span>
                    <a
                      href={`/api/files/${file.id}`}
                      target="_blank"
                      rel="noreferrer"
                      aria-label={`Abrir ${file.name} em nova aba`}
                    >
                      <ExternalLink size={14} />
                    </a>
                  </div>
                  {analysis && (
                    <AnalysisPanel
                      analysis={analysis}
                      retrying={analyze.isPending}
                      onRetry={
                        selected.status === "uploaded"
                          ? () => analyze.mutate()
                          : undefined
                      }
                    />
                  )}
                  {(file.sample ||
                    file.scanStatus === "unscanned_development") && (
                    <div className="sample-file-note">
                      {file.sample
                        ? "Documento fictício para explorar a demonstração."
                        : "Ambiente local: este arquivo ainda não passou por verificação antivírus."}
                    </div>
                  )}
                </>
              ) : (
                <div className="viewer-empty">
                  <div>
                    <FileText size={36} strokeWidth={1} />
                  </div>
                  <h3>Aguardando o documento chegar.</h3>
                  <p>
                    Assim que seu cliente enviar, o arquivo aparece aqui para
                    você conferir.
                  </p>
                  <Button variant="secondary" onClick={() => setSharing(true)}>
                    <Link2 size={14} />
                    Compartilhar link com o cliente
                  </Button>
                </div>
              )}
              {selected.status === "uploaded" && (
                <div className="review-actionbar">
                  <span>
                    <span className="review-ready-dot" />
                    {analysis?.verdict === "approved"
                      ? "A análise sugere aprovar"
                      : analysis?.verdict === "rejected"
                        ? "A análise sugere solicitar correção"
                        : "Pronto para sua revisão"}
                  </span>
                  <div>
                    <Button
                      variant="secondary"
                      disabled={review.isPending}
                      onClick={() => {
                        setReason(
                          analysis?.verdict === "rejected"
                            ? analysis.reason || ""
                            : "",
                        );
                        setCorrecting(true);
                      }}
                    >
                      <RotateCcw size={14} />
                      Solicitar correção
                    </Button>
                    <Button
                      loading={review.isPending}
                      onClick={() => review.mutate("approve")}
                    >
                      <Check size={16} />
                      Aprovar documento
                    </Button>
                  </div>
                </div>
              )}
              {selected.status === "approved" && (
                <div className="review-approved-bar">
                  <CheckCheck size={17} />
                  <span>Documento aprovado. Tudo certo por aqui.</span>
                </div>
              )}
            </>
          )}
        </section>
      </div>
      {selected?.reviews && selected.reviews.length > 0 && (
        <div className="review-history">
          <h3>Histórico deste documento</h3>
          {selected.reviews
            .slice()
            .reverse()
            .map((entry, i) => (
              <div key={`${entry.createdAt}-${i}`}>
                <span className={`history-icon ${entry.action}`}>
                  {entry.action === "approve" ? (
                    <Check size={12} />
                  ) : (
                    <RotateCcw size={12} />
                  )}
                </span>
                <p>
                  <strong>
                    {entry.action === "approve"
                      ? "Documento aprovado"
                      : "Correção solicitada"}
                  </strong>
                  {entry.reason && <span>{entry.reason}</span>}
                  <small>
                    {dateLabel(entry.createdAt)} ·{" "}
                    {entry.source === "jev"
                      ? "Análise automática (Jev)"
                      : entry.userId === session.user.id
                        ? session.user.name
                        : "Equipe do escritório"}
                  </small>
                </p>
              </div>
            ))}
        </div>
      )}
      {correcting && (
        <Modal
          title="O que precisa ser corrigido?"
          description="Seu cliente verá este motivo no portal e poderá reenviar só este documento."
          onClose={() => setCorrecting(false)}
        >
          <form
            onSubmit={(e) => {
              e.preventDefault();
              if (reason.trim()) review.mutate("correct");
            }}
          >
            <div className="correction-examples">
              <span>Sugestões para facilitar</span>
              {[
                ...(analysis?.verdict === "rejected" && analysis.reason
                  ? [analysis.reason]
                  : []),
                "A imagem está cortada. Envie uma nova foto mostrando o documento inteiro.",
                "O arquivo está ilegível. Tente fotografar em um ambiente mais iluminado.",
                "O documento não corresponde à competência solicitada.",
              ].map((s) => (
                <button type="button" key={s} onClick={() => setReason(s)}>
                  {s}
                </button>
              ))}
            </div>
            <label className="field">
              <span className="field-label">Motivo da correção</span>
              <textarea
                className="textarea"
                aria-label="Motivo da correção"
                placeholder="Explique de forma simples o que precisa mudar…"
                maxLength={1000}
                required
                value={reason}
                onChange={(e) => setReason(e.target.value)}
              />
            </label>
            <div className="form-actions">
              <Button variant="ghost" onClick={() => setCorrecting(false)}>
                Voltar
              </Button>
              <Button
                type="submit"
                loading={review.isPending}
                disabled={!reason.trim()}
              >
                Solicitar correção
              </Button>
            </div>
          </form>
        </Modal>
      )}
      {sharing && <ShareModal request={r} onClose={() => setSharing(false)} />}{" "}
      {cancel && (
        <Modal
          title="Cancelar esta solicitação?"
          description="Os documentos e o histórico serão preservados. Os links do cliente deixarão de funcionar."
          onClose={() => setCancel(false)}
        >
          <div className="form-actions">
            <Button variant="secondary" onClick={() => setCancel(false)}>
              Manter solicitação
            </Button>
            <Button
              variant="danger"
              loading={cancelMutation.isPending}
              onClick={() => cancelMutation.mutate()}
            >
              Cancelar solicitação
            </Button>
          </div>
        </Modal>
      )}
    </div>
  );
}

export function FilePreview({
  file,
  token,
}: {
  file: DocumentFile;
  token?: string;
}) {
  const url = `/api/files/${file.id}${token ? `?token=${encodeURIComponent(token)}` : ""}`;
  if (file.mimeType === "application/pdf")
    return <iframe src={url} title={`Pré-visualização de ${file.name}`} />;
  if (["image/jpeg", "image/png"].includes(file.mimeType))
    return <img src={url} alt={`Documento enviado: ${file.name}`} />;
  return (
    <div className="unsupported-preview">
      <Image size={30} strokeWidth={1.3} />
      <p>Este arquivo está salvo.</p>
      <span>O formato HEIC pode ser aberto no seu dispositivo.</span>
      <a className="text-link" href={url} target="_blank" rel="noreferrer">
        Baixar arquivo
        <Download size={14} />
      </a>
    </div>
  );
}
export function formatBytes(size: number) {
  return size < 1024 * 1024
    ? `${Math.max(1, Math.round(size / 1024))} KB`
    : `${(size / 1024 / 1024).toFixed(1)} MB`;
}

export function ShareModal({
  request,
  onClose,
}: {
  request: DocumentRequest;
  onClose: () => void;
}) {
  const [link, setLink] = useState("");
  const [loading, setLoading] = useState(false);
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");
  const toast = useToast();
  const queryClient = useQueryClient();
  const { data: clients = [] } = useQuery({
    queryKey: ["clients"],
    queryFn: () => api<Client[]>("/clients"),
  });
  const client = clients.find((c) => c.id === request.clientId);
  async function ensureLink() {
    if (link) return link;
    setLoading(true);
    try {
      const result = await post<{ url: string }>(
        `/requests/${request.id}/share`,
      );
      setLink(result.url);
      return result.url;
    } finally {
      setLoading(false);
    }
  }
  async function action(type: string) {
    setError("");
    setBusy(type);
    try {
      if (type === "email") {
        const result = await post<{ status: string; error?: string }>(
          `/requests/${request.id}/notify`,
          { channel: "email" },
        );
        if (result.status !== "accepted")
          throw new Error(
            result.error || "O e-mail ainda não foi aceito pelo provedor.",
          );
        toast("E-mail aceito pelo provedor.");
      } else if (type === "revoke") {
        await post(`/requests/${request.id}/revoke`);
        setLink("");
        toast("Links anteriores revogados. Gere um novo quando precisar.");
      } else {
        const url = await ensureLink();
        if (type === "copy") {
          await navigator.clipboard.writeText(url);
          toast("Link copiado para compartilhar.");
        } else if (type === "portal") {
          window.open(url, "_blank", "noopener,noreferrer");
        } else {
          const phone = client?.phone.replace(/\D/g, "") || "";
          window.open(
            `https://wa.me/${phone.length === 10 || phone.length === 11 ? "55" : ""}${phone}?text=${encodeURIComponent(`Olá${client?.contactName ? ", " + client.contactName : ""}! Precisamos dos documentos de ${request.title}. Você pode enviar até ${request.dueDate.split("-").reverse().join("/")} pelo link: ${url}`)}`,
            "_blank",
            "noopener,noreferrer",
          );
          await post(`/requests/${request.id}/notify`, {
            channel: "whatsapp_opened",
          });
          toast("WhatsApp aberto. Confirme o envio por lá.");
        }
      }
    } catch (e) {
      setError(
        e instanceof Error ? e.message : "Não foi possível compartilhar.",
      );
    } finally {
      await queryClient.invalidateQueries({ queryKey: ["notifications"] });
      setBusy("");
    }
  }
  return (
    <Modal
      title="Um link. Todos os documentos."
      description={`Compartilhe a solicitação de ${request.clientName}. O cliente não precisa criar uma conta.`}
      onClose={onClose}
    >
      <div className="share-options">
        <button onClick={() => void action("copy")} disabled={!!busy}>
          <span>
            <Copy size={18} />
          </span>
          <div>
            <strong>Copiar link</strong>
            <p>Compartilhe pelo canal que preferir.</p>
          </div>
          {busy === "copy" ? <Clock3 size={15} /> : <ArrowRight size={15} />}
        </button>
        <button onClick={() => void action("whatsapp")} disabled={!!busy}>
          <span>
            <MessageCircle size={18} />
          </span>
          <div>
            <strong>Abrir no WhatsApp</strong>
            <p>A mensagem fica pronta. Você confirma o envio.</p>
          </div>
          <ArrowUpRight size={15} />
        </button>
        <button onClick={() => void action("email")} disabled={!!busy}>
          <span>
            <Mail size={18} />
          </span>
          <div>
            <strong>Enviar por e-mail</strong>
            <p>{client?.email || "Usa o e-mail do responsável cadastrado."}</p>
          </div>
          {busy === "email" ? <Clock3 size={15} /> : <ArrowRight size={15} />}
        </button>
      </div>
      {link && (
        <div className="share-link">
          <input
            className="input"
            readOnly
            value={link}
            aria-label="Link público da solicitação"
          />
          <button className="text-link" onClick={() => void action("portal")}>
            Abrir portal
            <ExternalLink size={13} />
          </button>
          <small>Este link expira em 30 dias.</small>
        </div>
      )}
      {error && (
        <div className="inline-error" role="alert">
          {error}
        </div>
      )}
      <div className="share-footer">
        <ShieldCheck size={14} />
        <span>Acesso individual. Somente esta solicitação.</span>
        <button
          onClick={() => void action("revoke")}
          disabled={!!busy || loading}
        >
          Revogar links
        </button>
      </div>
    </Modal>
  );
}
