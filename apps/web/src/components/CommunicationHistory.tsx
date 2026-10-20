import { useState } from "react";
import { Link } from "react-router-dom";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  ArrowUpRight,
  Check,
  Clock3,
  Mail,
  MessageCircle,
  RotateCcw,
  TriangleAlert,
} from "lucide-react";
import { api, post } from "@/lib/api";
import type { DocumentRequest } from "@/lib/types";
import { useToast } from "@/lib/toast";
import { Button, EmptyState, ErrorState, Loading, Modal } from "./shared";
import "./communication-history.css";

type Delivery = {
  id: string;
  requestId: string;
  channel: string;
  kind: string;
  status: string;
  error?: string;
  createdAt: string;
};
const statusLabels: Record<string, string> = {
  accepted: "Aceito pelo provedor",
  failed: "Falha no envio",
  opened: "WhatsApp aberto",
  pending: "Aguardando processamento",
};
function timestamp(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "Horário não disponível";
  return new Intl.DateTimeFormat("pt-BR", {
    day: "2-digit",
    month: "short",
    hour: "2-digit",
    minute: "2-digit",
    timeZone: "America/Sao_Paulo",
  }).format(date);
}
function kindLabel(kind: string) {
  const reminder = /^reminder_(\d+)$/.exec(kind);
  if (reminder) return `Lembrete após ${reminder[1]} dias`;
  return (
    (
      {
        initial: "Solicitação inicial",
        manual: "Ação manual",
        correction: "Correção necessária",
      } as Record<string, string>
    )[kind] || "Comunicação da solicitação"
  );
}

export function CommunicationHistory({ onClose }: { onClose: () => void }) {
  const notifications = useQuery({
    queryKey: ["notifications"],
    queryFn: () => api<Delivery[]>("/notifications"),
  });
  const requests = useQuery({
    queryKey: ["requests"],
    queryFn: () => api<DocumentRequest[]>("/requests"),
  });
  const [retrying, setRetrying] = useState("");
  const [onlyFailed, setOnlyFailed] = useState(false);
  const queryClient = useQueryClient();
  const toast = useToast();
  const deliveries = [...(notifications.data ?? [])].sort(
    (a, b) =>
      b.createdAt.localeCompare(a.createdAt) || b.id.localeCompare(a.id),
  );
  const latestEmail = new Map<string, string>();
  for (const delivery of deliveries)
    if (delivery.channel === "email" && !latestEmail.has(delivery.requestId))
      latestEmail.set(delivery.requestId, delivery.id);
  const visible = onlyFailed
    ? deliveries.filter((delivery) => delivery.status === "failed")
    : deliveries;
  const failures = deliveries.filter(
    (delivery) => delivery.status === "failed",
  ).length;
  async function retry(delivery: Delivery) {
    setRetrying(delivery.id);
    try {
      const result = await post<Delivery>(
        `/requests/${delivery.requestId}/notify`,
        { channel: "email" },
      );
      if (result.status === "accepted")
        toast(
          "E-mail aceito pelo provedor. A entrega ao destinatário ainda não está confirmada.",
        );
      else
        toast(
          result.error ||
            "O provedor ainda não aceitou o e-mail. Confira o registro da tentativa.",
          "error",
        );
    } catch (error) {
      toast(
        error instanceof Error
          ? error.message
          : "Não foi possível tentar novamente.",
        "error",
      );
    } finally {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["notifications"] }),
        queryClient.invalidateQueries({ queryKey: ["requests"] }),
      ]);
      setRetrying("");
    }
  }
  return (
    <Modal
      title="Histórico de comunicações"
      description="Aceito pelo provedor confirma o recebimento do e-mail para envio, sem confirmar entrega. WhatsApp aberto registra a abertura da mensagem, sem confirmar envio."
      onClose={onClose}
      wide
    >
      {notifications.isPending || requests.isPending ? (
        <Loading label="Consultando as tentativas de comunicação…" />
      ) : notifications.isError || requests.isError ? (
        <ErrorState
          message={
            (notifications.error || requests.error)?.message ||
            "Não foi possível carregar as comunicações."
          }
          onRetry={() => {
            void notifications.refetch();
            void requests.refetch();
          }}
        />
      ) : !deliveries.length ? (
        <EmptyState
          title="As primeiras comunicações aparecem aqui"
          description="Cada tentativa por e-mail e cada abertura do WhatsApp será registrada com seu resultado real."
        />
      ) : (
        <div className="ch-history">
          <div className="ch-toolbar">
            <span>
              {deliveries.length}{" "}
              {deliveries.length === 1 ? "registro" : "registros"}
              <i />
              Horários de Brasília
            </span>
            <button
              className={
                onlyFailed ? "ch-filter ch-filter-active" : "ch-filter"
              }
              aria-pressed={onlyFailed}
              onClick={() => setOnlyFailed((value) => !value)}
            >
              Com falha<span>{failures}</span>
            </button>
          </div>
          {!visible.length ? (
            <EmptyState
              title="Nenhuma falha registrada"
              description="As tentativas que não forem aceitas aparecerão neste filtro."
            />
          ) : (
            <div className="ch-list">
              {visible.map((delivery) => {
                const request = requests.data?.find(
                  (value) => value.id === delivery.requestId,
                );
                const active =
                  request &&
                  request.status !== "completed" &&
                  request.status !== "cancelled";
                const canRetry =
                  active &&
                  delivery.status === "failed" &&
                  delivery.channel === "email" &&
                  latestEmail.get(delivery.requestId) === delivery.id;
                return (
                  <article
                    className={`ch-row ch-row-${delivery.status}`}
                    key={delivery.id}
                  >
                    <span className="ch-channel-icon">
                      {delivery.channel === "whatsapp_opened" ? (
                        <MessageCircle size={18} strokeWidth={1.5} />
                      ) : (
                        <Mail size={18} strokeWidth={1.5} />
                      )}
                    </span>
                    <div className="ch-row-main">
                      <div className="ch-row-heading">
                        <strong>
                          {request?.clientName || "Solicitação não disponível"}
                        </strong>
                        <span
                          className={`ch-status ch-status-${delivery.status}`}
                        >
                          {delivery.status === "accepted" ? (
                            <Check size={12} />
                          ) : delivery.status === "failed" ? (
                            <TriangleAlert size={12} />
                          ) : delivery.status === "pending" ? (
                            <Clock3 size={12} />
                          ) : null}
                          {statusLabels[delivery.status] || delivery.status}
                        </span>
                      </div>
                      <p>
                        {kindLabel(delivery.kind)}
                        <span>·</span>
                        {delivery.channel === "email"
                          ? "E-mail"
                          : delivery.channel === "whatsapp_opened"
                            ? "WhatsApp"
                            : delivery.channel}
                      </p>
                      {delivery.error && (
                        <p className="ch-delivery-error">{delivery.error}</p>
                      )}
                      <div className="ch-row-bottom">
                        <time dateTime={delivery.createdAt}>
                          {timestamp(delivery.createdAt)}
                        </time>
                        {request && (
                          <Link
                            to={`/app/solicitacoes/${request.id}`}
                            onClick={onClose}
                          >
                            Ver solicitação
                            <ArrowUpRight size={12} />
                          </Link>
                        )}
                      </div>
                    </div>
                    {canRetry && (
                      <Button
                        className="ch-retry"
                        variant="secondary"
                        loading={retrying === delivery.id}
                        disabled={!!retrying}
                        onClick={() => void retry(delivery)}
                      >
                        <RotateCcw size={13} />
                        Tentar e-mail novamente
                      </Button>
                    )}
                  </article>
                );
              })}
            </div>
          )}
          <p className="ch-footer-note">
            Solicitações concluídas ou canceladas não recebem novas tentativas.
            O reenvio fica disponível na tentativa de e-mail mais recente que
            falhou.
          </p>
        </div>
      )}
    </Modal>
  );
}
