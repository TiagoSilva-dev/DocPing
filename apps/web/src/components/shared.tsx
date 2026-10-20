import { useEffect, useRef, type ReactNode } from "react";
import { X, Inbox, ArrowUpRight, Loader2, AlertCircle } from "lucide-react";
import {
  Button as EasyButton,
  type ButtonProps as EasyButtonProps,
} from "./ui/button";
import { initials, statusLabels } from "@/lib/format";

export function Button({
  variant = "primary",
  loading = false,
  className = "",
  ...props
}: Omit<EasyButtonProps, "variant" | "isLoading"> & {
  variant?: "primary" | "secondary" | "ghost" | "danger";
  loading?: boolean;
}) {
  return (
    <EasyButton
      variant={variant === "danger" ? "destructive" : variant}
      isLoading={loading}
      className={`dp-button ${variant} ${className}`}
      {...props}
    />
  );
}
export function Logo({
  light = false,
  small = false,
}: {
  light?: boolean;
  small?: boolean;
}) {
  return (
    <span
      className={`brand ${light ? "brand-light" : ""} ${small ? "brand-small" : ""}`}
    >
      <svg
        width="30"
        height="32"
        viewBox="0 0 30 32"
        fill="none"
        aria-hidden="true"
      >
        <path
          d="M5 3.5h11l8 8v15a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-21a2 2 0 0 1 2-2Z"
          stroke="currentColor"
          strokeWidth="2.1"
        />
        <path
          d="M16 3.5v7a1 1 0 0 0 1 1h7M8 16h10M8 21h6"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
        />
        <circle
          cx="24"
          cy="24"
          r="5.5"
          fill="#83B694"
          stroke="var(--bg,#F8F9F6)"
          strokeWidth="2.5"
        />
      </svg>
      <span>
        docping<span className="brand-period">.</span>
      </span>
    </span>
  );
}
export function Avatar({ name, color }: { name: string; color?: string }) {
  const colors = ["sage", "sand", "lilac", "blue", "rose"];
  const hash = Array.from(name).reduce((a, c) => a + c.charCodeAt(0), 0);
  return (
    <span
      className={`avatar ${color || colors[hash % colors.length]}`}
      aria-hidden="true"
    >
      {initials(name)}
    </span>
  );
}
export function StatusBadge({ status }: { status: string }) {
  return (
    <span className={`status-badge ${status}`}>
      <span />
      {statusLabels[status] || status}
    </span>
  );
}
export function PageHeading({
  eyebrow,
  title,
  description,
  action,
}: {
  eyebrow?: string;
  title: string;
  description?: string;
  action?: ReactNode;
}) {
  return (
    <div className="page-heading">
      <div>
        {eyebrow && <p className="eyebrow">{eyebrow}</p>}
        <h1>{title}</h1>
        {description && <p className="page-description">{description}</p>}
      </div>
      {action && <div className="page-heading-action">{action}</div>}
    </div>
  );
}
export function Field({
  label,
  children,
  hint,
}: {
  label: string;
  children: ReactNode;
  hint?: string;
}) {
  return (
    <label className="field">
      <span className="field-label">{label}</span>
      {children}
      {hint && <span className="field-hint">{hint}</span>}
    </label>
  );
}
export function Modal({
  title,
  description,
  onClose,
  children,
  wide = false,
}: {
  title: string;
  description?: string;
  onClose: () => void;
  children: ReactNode;
  wide?: boolean;
}) {
  const ref = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    const dialog = ref.current;
    dialog?.showModal();
    document.body.style.overflow = "hidden";
    return () => {
      dialog?.close();
      document.body.style.overflow = "";
    };
  }, []);
  return (
    <dialog
      ref={ref}
      className={`modal ${wide ? "modal-wide" : ""}`}
      onCancel={(e) => {
        e.preventDefault();
        onClose();
      }}
      onClick={(e) => {
        if (e.target === e.currentTarget) {
          const r = e.currentTarget.getBoundingClientRect();
          if (
            e.clientX < r.left ||
            e.clientX > r.right ||
            e.clientY < r.top ||
            e.clientY > r.bottom
          )
            onClose();
        }
      }}
      aria-labelledby="modal-title"
    >
      <div className="modal-heading">
        <div>
          <h2 id="modal-title">{title}</h2>
          {description && <p>{description}</p>}
        </div>
        <button
          className="icon-button"
          aria-label="Fechar"
          onClick={onClose}
          autoFocus
        >
          <X size={20} />
        </button>
      </div>
      <div className="modal-body">{children}</div>
    </dialog>
  );
}
export function EmptyState({
  title,
  description,
  action,
}: {
  title: string;
  description: string;
  action?: ReactNode;
}) {
  return (
    <div className="empty-state">
      <span className="empty-icon">
        <Inbox size={25} strokeWidth={1.5} />
      </span>
      <h3>{title}</h3>
      <p>{description}</p>
      {action}
    </div>
  );
}
export function Loading({
  label = "Carregando seu espaço…",
}: {
  label?: string;
}) {
  return (
    <div className="loading-state" role="status">
      <Loader2 size={21} className="spin" />
      <span>{label}</span>
    </div>
  );
}
export function ErrorState({
  message,
  onRetry,
}: {
  message: string;
  onRetry?: () => void;
}) {
  return (
    <div className="error-state" role="alert">
      <AlertCircle size={24} />
      <h3>Não foi possível carregar</h3>
      <p>{message}</p>
      {onRetry && (
        <Button variant="secondary" onClick={onRetry}>
          Tentar novamente
        </Button>
      )}
    </div>
  );
}
export function TextLink({
  children,
  onClick,
}: {
  children: ReactNode;
  onClick: () => void;
}) {
  return (
    <button className="text-link" onClick={onClick}>
      {children}
      <ArrowUpRight size={14} />
    </button>
  );
}
