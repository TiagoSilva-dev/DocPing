import type { DocumentRequest } from "./types";
export const monthName = (value: string) =>
  new Intl.DateTimeFormat("pt-BR", {
    month: "long",
    year: "numeric",
    timeZone: "UTC",
  }).format(new Date(`${value.slice(0, 7)}-02T12:00:00Z`));
export const shortMonth = (value: string) =>
  new Intl.DateTimeFormat("pt-BR", {
    month: "short",
    year: "numeric",
    timeZone: "UTC",
  })
    .format(new Date(`${value.slice(0, 7)}-02T12:00:00Z`))
    .replace(" de ", " ")
    .replace(".", "");
export function dateLabel(value: string) {
  const timestamp = value.includes("T");
  const date = new Date(timestamp ? value : `${value}T12:00:00Z`);
  if (Number.isNaN(date.getTime())) return "—";
  return new Intl.DateTimeFormat("pt-BR", {
    day: "2-digit",
    month: "short",
    timeZone: timestamp ? "America/Sao_Paulo" : "UTC",
  })
    .format(date)
    .replace(" de ", " ")
    .replace(".", "");
}
export const today = () =>
  new Intl.DateTimeFormat("en-CA", {
    timeZone: "America/Sao_Paulo",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).format(new Date());
export function overdue(r: DocumentRequest) {
  return (
    r.status !== "completed" &&
    r.status !== "cancelled" &&
    r.dueDate.slice(0, 10) < today()
  );
}
export function daysLate(r: DocumentRequest) {
  return Math.max(
    1,
    Math.round(
      (new Date(`${today()}T12:00:00Z`).getTime() -
        new Date(`${r.dueDate.slice(0, 10)}T12:00:00Z`).getTime()) /
        86400000,
    ),
  );
}
export function initials(value: string) {
  return value
    .split(" ")
    .filter(Boolean)
    .slice(0, 2)
    .map((x) => x[0])
    .join("")
    .toUpperCase();
}
export function progress(r: DocumentRequest) {
  const required = r.items.filter((i) => i.required);
  const approved = required.filter((i) => i.status === "approved").length;
  return {
    approved,
    total: required.length,
    percent: required.length
      ? Math.round((approved / required.length) * 100)
      : 0,
  };
}
export function relativeDate(value: string) {
  const date = new Date(value.includes("T") ? value : `${value}T12:00:00Z`);
  if (Number.isNaN(date.getTime())) return "—";
  const minutes = (Date.now() - date.getTime()) / 60000;
  if (Math.abs(minutes) < 1) return "Agora";
  if (minutes < 0) return dateLabel(value);
  if (minutes < 60) return `Há ${Math.floor(minutes)} min`;
  const localDay = new Intl.DateTimeFormat("en-CA", {
    timeZone: "America/Sao_Paulo",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).format(date);
  const currentDay = today();
  if (localDay === currentDay) return `Há ${Math.floor(minutes / 60)} h`;
  const yesterday = new Date(`${currentDay}T12:00:00Z`);
  yesterday.setUTCDate(yesterday.getUTCDate() - 1);
  if (localDay === yesterday.toISOString().slice(0, 10)) return "Ontem";
  return dateLabel(value);
}
export const statusLabels: Record<string, string> = {
  pending: "Aguardando cliente",
  in_progress: "Em andamento",
  under_review: "Aguardando revisão",
  correction_required: "Precisa corrigir",
  completed: "Concluída",
  cancelled: "Cancelada",
  uploaded: "Em análise",
  approved: "Aprovado",
};
