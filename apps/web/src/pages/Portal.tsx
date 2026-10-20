import { useRef, useState } from "react";
import { useParams } from "react-router-dom";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Upload,
  Camera,
  FileText,
  Check,
  CheckCheck,
  ChevronDown,
  ShieldCheck,
  Clock3,
  ArrowRight,
  RotateCcw,
  AlertCircle,
  ExternalLink,
  LockKeyhole,
  ArrowUpRight,
  Loader2,
  Building2,
} from "lucide-react";
import { api, ApiError, post, uploadFile } from "@/lib/api";
import type {
  Portal as PortalData,
  RequestItem,
  DocumentRequest,
} from "@/lib/types";
import { Button, Logo, Loading, Modal } from "@/components/shared";
import { dateLabel, initials, monthName } from "@/lib/format";
import { formatBytes, FilePreview } from "./RequestDetail";
import { useToast } from "@/lib/toast";
import "./portal.css";

export function PortalPage() {
  const { token = "" } = useParams();
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey: ["portal", token],
    queryFn: () => api<PortalData>(`/portal/${token}`),
    retry: false,
    refetchInterval: 20000,
  });
  const toast = useToast();
  const [uploads, setUploads] = useState<
    Record<
      string,
      { file: File; progress: number; error?: string; busy: boolean }
    >
  >({});
  const [preview, setPreview] = useState<{
    file: RequestItem["files"][number];
    token: string;
  } | null>(null);
  const data = query.data;
  const required = data?.request.items.filter((i) => i.required) || [];
  const sent = required.filter((i) =>
    ["uploaded", "approved"].includes(i.status),
  ).length;
  const complete = data?.request.status === "completed";
  const allSent = required.length > 0 && sent === required.length;
  const maxBytes = data?.maxUploadBytes || 20 * 1024 * 1024;
  async function send(item: RequestItem, file: File) {
    if (uploads[item.id]?.busy) return;
    const allowed = ["pdf", "jpg", "jpeg", "png", "heic"];
    if (!allowed.includes(file.name.split(".").pop()?.toLowerCase() || "")) {
      toast("Escolha um arquivo PDF, JPG, PNG ou HEIC.", "error");
      return;
    }
    if (file.size > maxBytes) {
      toast(
        `O arquivo precisa ter até ${Math.round(maxBytes / 1024 / 1024)} MB.`,
        "error",
      );
      return;
    }
    if (file.size === 0) {
      toast("Este arquivo está vazio. Escolha outro arquivo.", "error");
      return;
    }
    setUploads((p) => ({ ...p, [item.id]: { file, progress: 0, busy: true } }));
    try {
      const updated = (await uploadFile(token, item.id, file, (percent) =>
        setUploads((p) => ({
          ...p,
          [item.id]: { ...p[item.id], progress: percent },
        })),
      )) as DocumentRequest;
      queryClient.setQueryData<PortalData>(["portal", token], (old) =>
        old ? { ...old, request: updated } : old,
      );
      setUploads((p) => ({
        ...p,
        [item.id]: { ...p[item.id], progress: 100, busy: false },
      }));
      toast("Documento enviado. Seu progresso está salvo.");
    } catch (e) {
      setUploads((p) => ({
        ...p,
        [item.id]: {
          ...p[item.id],
          busy: false,
          error:
            e instanceof Error ? e.message : "O envio falhou. Tente novamente.",
        },
      }));
    }
  }
  if (query.isLoading)
    return (
      <div className="portal-loading">
        <Logo />
        <Loading label="Abrindo sua solicitação…" />
      </div>
    );
  if (query.error) {
    if (
      query.error instanceof ApiError &&
      query.error.code === "identity_required"
    )
      return (
        <IdentityGate token={token} onVerified={() => void query.refetch()} />
      );
    return (
      <div className="portal-error-page">
        <Logo />
        <div className="portal-error-icon">
          <LockKeyhole size={31} strokeWidth={1.3} />
        </div>
        <span className="eyebrow">ACESSO AO DOCUMENTO</span>
        <h1>
          {query.error instanceof ApiError && query.error.status === 410
            ? "Este link não está mais disponível."
            : "Não conseguimos abrir sua solicitação."}
        </h1>
        <p>{query.error.message}</p>
        <p className="portal-error-help">
          Fale com o escritório que solicitou os documentos para receber um novo
          acesso.
        </p>
        <Button variant="secondary" onClick={() => void query.refetch()}>
          <RotateCcw size={15} />
          Tentar novamente
        </Button>
        <div className="portal-error-footer">
          Seus documentos continuam privados e protegidos.
        </div>
      </div>
    );
  }
  if (!data) return null;
  return (
    <div className="portal-page">
      <header className="portal-topbar">
        <span>
          <ShieldCheck size={14} />
          Portal de documentos
        </span>
        <Logo small />
      </header>
      <main className="portal-main">
        <div className="portal-office">
          <span className="portal-office-mark">
            {initials(data.organization.name)}
          </span>
          <div>
            <strong>{data.organization.name}</strong>
            <span>Solicitação de documentos</span>
          </div>
          {data.organization.demo && (
            <span className="portal-demo-badge">Demonstração</span>
          )}
        </div>
        <div className="portal-intro">
          <span className="portal-client-name">
            OLÁ, {data.client.name.toUpperCase()}
          </span>
          <h1>
            {complete
              ? "Tudo certo com os seus documentos."
              : allSent
                ? "Seus documentos chegaram."
                : `Vamos organizar seus documentos?`}
          </h1>
          <p>
            {complete
              ? "O escritório conferiu e aprovou os documentos desta solicitação. Obrigado por enviar!"
              : allSent
                ? "O escritório vai conferir os arquivos. Se algo precisar de ajuste, você encontra a orientação por aqui."
                : `Envie os arquivos abaixo para ${data.request.title.toLowerCase()}. Pode ser pelo celular, no seu tempo.`}
          </p>
          <span className="portal-due">
            <Clock3 size={14} />
            {monthName(data.request.competence)}
            <span>·</span>Prazo: {dateLabel(data.request.dueDate)}
          </span>
        </div>
        <div className={`portal-progress-card ${allSent ? "all-sent" : ""}`}>
          <div>
            <span>
              {complete ? (
                <>
                  <CheckCheck size={16} />
                  Tudo aprovado
                </>
              ) : allSent ? (
                <>
                  <Check size={16} />
                  Tudo enviado para análise
                </>
              ) : (
                <>Cada envio é um passo a menos.</>
              )}
            </span>
            <strong>
              {sent}
              <small>/{required.length}</small>
            </strong>
          </div>
          <div className="portal-progress-track">
            <span
              style={{
                width: `${required.length ? (sent / required.length) * 100 : 0}%`,
              }}
            />
          </div>
          <p>
            {complete
              ? "Esta solicitação está concluída."
              : `${sent} de ${required.length} documentos obrigatórios enviados`}
          </p>
        </div>
        {(data.organization.demo || data.environment === "development") && (
          <div className="portal-demo-notice">
            <AlertCircle size={14} />
            <span>
              {data.organization.demo
                ? "Este é um ambiente de demonstração. Use apenas arquivos fictícios."
                : "Ambiente local de teste. Use apenas arquivos fictícios."}
            </span>
          </div>
        )}
        <section
          className="portal-documents"
          aria-label="Documentos solicitados"
        >
          {data.request.items.map((item, i) => (
            <PortalItem
              key={item.id}
              item={item}
              index={i}
              token={token}
              maxBytes={maxBytes}
              upload={uploads[item.id]}
              closed={complete}
              onSend={(file) => void send(item, file)}
              onRetry={() => {
                const file = uploads[item.id]?.file;
                if (file) void send(item, file);
              }}
              onPreview={(file) => setPreview({ file, token })}
            />
          ))}
        </section>
        <div className="portal-autosave">
          <span>
            <ShieldCheck size={18} />
          </span>
          <div>
            <strong>Seu progresso está salvo.</strong>
            <p>
              Você pode fechar esta página e continuar depois pelo mesmo link.
              Não precisa enviar tudo de uma vez.
            </p>
          </div>
        </div>
        <div className="portal-help-card">
          <Building2 size={18} />
          <p>
            Ficou com alguma dúvida sobre um documento?
            <br />
            <strong>Converse com {data.organization.name}.</strong>
          </p>
        </div>
      </main>
      <footer className="portal-footer">
        <span>
          Organizado com <Logo small />
        </span>
        <span>
          <LockKeyhole size={11} />
          Acesso individual e temporário.
        </span>
      </footer>
      {preview && (
        <Modal title={preview.file.name} onClose={() => setPreview(null)} wide>
          <div className="portal-file-preview">
            <FilePreview file={preview.file} token={preview.token} />
          </div>
          <div className="portal-preview-footer">
            <a
              className="text-link"
              href={`/api/files/${preview.file.id}?token=${token}`}
              target="_blank"
              rel="noreferrer"
            >
              Abrir arquivo
              <ArrowUpRight size={13} />
            </a>
          </div>
        </Modal>
      )}
    </div>
  );
}

function PortalItem({
  item,
  index,
  token,
  maxBytes,
  upload,
  closed,
  onSend,
  onRetry,
  onPreview,
}: {
  item: RequestItem;
  index: number;
  token: string;
  maxBytes: number;
  upload?: { file: File; progress: number; busy: boolean; error?: string };
  closed: boolean;
  onSend: (file: File) => void;
  onRetry: () => void;
  onPreview: (file: RequestItem["files"][number]) => void;
}) {
  const [expanded, setExpanded] = useState(
    item.status === "pending" || item.status === "correction_required",
  );
  const [drag, setDrag] = useState(false);
  const input = useRef<HTMLInputElement>(null);
  const camera = useRef<HTMLInputElement>(null);
  const files = item.files.filter((f) => !f.superseded);
  const canUpload =
    !closed &&
    item.status !== "approved" &&
    (files.length < item.maxFiles || item.status === "correction_required");
  const label = {
    pending: "Falta enviar",
    uploaded: "Em análise",
    approved: "Aprovado",
    correction_required: "Precisa corrigir",
  }[item.status];
  return (
    <article
      className={`portal-document ${item.status} ${expanded ? "expanded" : ""}`}
    >
      <button
        className="portal-document-heading"
        aria-expanded={expanded}
        aria-controls={`portal-document-${item.id}`}
        onClick={() => setExpanded((e) => !e)}
      >
        <span className={`portal-document-icon ${item.status}`}>
          {item.status === "approved" ? (
            <Check size={18} />
          ) : item.status === "uploaded" ? (
            <Clock3 size={17} />
          ) : item.status === "correction_required" ? (
            <RotateCcw size={17} />
          ) : (
            <FileText size={19} strokeWidth={1.4} />
          )}
        </span>
        <span>
          <strong>{item.name}</strong>
          <small>
            <i />
            {label}
            {!item.required && <em>Opcional</em>}
          </small>
        </span>
        <ChevronDown size={17} className="portal-accordion-arrow" />
      </button>
      {expanded && (
        <div className="portal-document-body" id={`portal-document-${item.id}`}>
          <p className="portal-item-instruction">
            {item.instruction || "Envie o documento completo e legível."}
          </p>
          {item.status === "correction_required" && (
            <div className="portal-correction">
              <AlertCircle size={16} />
              <div>
                <strong>Uma pequena correção</strong>
                <p>{item.correctionReason}</p>
                <span>
                  Envie uma nova versão abaixo. Os outros documentos continuam
                  salvos.
                </span>
              </div>
            </div>
          )}
          {files.length > 0 && (
            <div className="portal-sent-files">
              {files.map((f) => (
                <button onClick={() => onPreview(f)} key={f.id}>
                  <span>
                    <FileText size={17} />
                  </span>
                  <div>
                    <strong>
                      {f.name}
                      {item.correctionFileId === f.id
                        ? " · Corrigir este arquivo"
                        : ""}
                    </strong>
                    <small>
                      {formatBytes(f.size)} · Enviado em{" "}
                      {dateLabel(f.createdAt)}
                    </small>
                  </div>
                  <ExternalLink size={14} />
                </button>
              ))}
            </div>
          )}
          {upload?.busy ? (
            <div className="portal-upload-progress" role="status">
              <div>
                <Loader2 size={18} className="spin" />
                <span>
                  {upload.progress >= 99
                    ? "Salvando e verificando o arquivo…"
                    : "Enviando " + upload.file.name}
                </span>
                <strong>{upload.progress}%</strong>
              </div>
              <div className="portal-upload-track">
                <span style={{ width: `${upload.progress}%` }} />
              </div>
              <p>Mantenha esta página aberta até o envio terminar.</p>
            </div>
          ) : upload?.error ? (
            <div className="portal-upload-error" role="alert">
              <AlertCircle size={17} />
              <div>
                <strong>O envio não terminou.</strong>
                <p>{upload.error}</p>
                <Button variant="secondary" onClick={onRetry}>
                  <RotateCcw size={14} />
                  Tentar novamente
                </Button>
              </div>
            </div>
          ) : null}
          {canUpload && !upload?.busy && (
            <div
              className={`portal-upload-area ${drag ? "dragging" : ""}`}
              onDragOver={(e) => {
                e.preventDefault();
                setDrag(true);
              }}
              onDragLeave={() => setDrag(false)}
              onDrop={(e) => {
                e.preventDefault();
                setDrag(false);
                const file = e.dataTransfer.files[0];
                if (file) onSend(file);
              }}
            >
              <Upload size={24} strokeWidth={1.2} />
              <p>
                {item.status === "correction_required"
                  ? "Envie uma nova versão"
                  : files.length
                    ? "Precisa adicionar outro arquivo?"
                    : "Escolha um arquivo ou arraste até aqui."}
              </p>
              <div className="portal-upload-buttons">
                <input
                  ref={input}
                  className="sr-only"
                  type="file"
                  id={`upload-${item.id}`}
                  aria-label={`Enviar ${item.name}`}
                  accept=".pdf,.jpg,.jpeg,.png,.heic,application/pdf,image/jpeg,image/png,image/heic"
                  onChange={(e) => {
                    const file = e.target.files?.[0];
                    if (file) onSend(file);
                    e.target.value = "";
                  }}
                />
                <Button
                  variant="secondary"
                  onClick={() => input.current?.click()}
                >
                  <Upload size={14} />
                  Escolher arquivo
                </Button>
                <input
                  ref={camera}
                  className="sr-only"
                  type="file"
                  capture="environment"
                  accept="image/*"
                  aria-label={`Fotografar ${item.name}`}
                  onChange={(e) => {
                    const file = e.target.files?.[0];
                    if (file) onSend(file);
                    e.target.value = "";
                  }}
                />
                <button
                  className="portal-camera"
                  onClick={() => camera.current?.click()}
                >
                  <Camera size={16} />
                  Tirar foto
                </button>
              </div>
              <small>
                PDF, JPG, PNG ou HEIC · Até {Math.round(maxBytes / 1024 / 1024)}{" "}
                MB
              </small>
            </div>
          )}
          {item.status === "approved" && (
            <div className="portal-approved">
              <CheckCheck size={15} />
              Documento conferido e aprovado pelo escritório.
            </div>
          )}
          {item.status === "uploaded" && !canUpload && (
            <div className="portal-review-note">
              <Clock3 size={14} />
              Recebido! Agora é com o escritório.
            </div>
          )}
        </div>
      )}
    </article>
  );
}

function IdentityGate({
  token,
  onVerified,
}: {
  token: string;
  onVerified: () => void;
}) {
  const [sent, setSent] = useState(false);
  const [code, setCode] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  async function send() {
    setBusy(true);
    setError("");
    try {
      await post(`/portal/${token}/code`);
      setSent(true);
    } catch (e) {
      setError(
        e instanceof Error ? e.message : "Não foi possível enviar o código.",
      );
    } finally {
      setBusy(false);
    }
  }
  async function verify() {
    setBusy(true);
    setError("");
    try {
      const result = await post<{ verified: boolean; message?: string }>(
        `/portal/${token}/verify`,
        { code },
      );
      if (result.verified) onVerified();
      else setError(result.message || "Código inválido ou expirado.");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Não foi possível confirmar.");
    } finally {
      setBusy(false);
    }
  }
  return (
    <div className="portal-error-page">
      <Logo />
      <div className="portal-error-icon">
        <ShieldCheck size={31} />
      </div>
      <p className="eyebrow">UMA CAMADA A MAIS DE CUIDADO</p>
      <h1>Confirme que é você.</h1>
      <p>
        Esta solicitação contém documentos sensíveis. Enviaremos um código para
        o e-mail cadastrado no escritório.
      </p>
      {sent ? (
        <form
          className="identity-form"
          onSubmit={(e) => {
            e.preventDefault();
            void verify();
          }}
        >
          <label className="field">
            <span className="field-label">Código de 6 dígitos</span>
            <input
              className="input mono"
              inputMode="numeric"
              pattern="[0-9]{6}"
              maxLength={6}
              autoComplete="one-time-code"
              value={code}
              onChange={(e) => setCode(e.target.value.replace(/\D/g, ""))}
              required
            />
          </label>
          <Button type="submit" loading={busy}>
            Confirmar código
            <ArrowRight size={14} />
          </Button>
          <button
            type="button"
            className="text-link"
            onClick={() => void send()}
            disabled={busy}
          >
            Reenviar código
          </button>
        </form>
      ) : (
        <Button loading={busy} onClick={() => void send()}>
          Receber código por e-mail
          <ArrowRight size={14} />
        </Button>
      )}
      {error && (
        <div className="inline-error" role="alert">
          {error}
        </div>
      )}
    </div>
  );
}
