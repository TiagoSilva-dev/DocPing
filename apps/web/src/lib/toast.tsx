import {
  createContext,
  useCallback,
  useContext,
  useState,
  type ReactNode,
} from "react";
import { CheckCircle2, AlertCircle, X } from "lucide-react";
type Toast = (message: string, tone?: "success" | "error") => void;
const ToastContext = createContext<Toast>(() => {});
export function useToast() {
  return useContext(ToastContext);
}
export function ToastProvider({ children }: { children: ReactNode }) {
  const [messages, setMessages] = useState<
    { id: number; text: string; tone: string }[]
  >([]);
  const toast = useCallback<Toast>((text, tone = "success") => {
    const id = Date.now() + Math.random();
    setMessages((p) => [...p.slice(-2), { id, text, tone }]);
    window.setTimeout(
      () => setMessages((p) => p.filter((m) => m.id !== id)),
      6500,
    );
  }, []);
  return (
    <ToastContext.Provider value={toast}>
      {children}
      <div className="toast-region" aria-live="polite">
        {messages.map((m) => (
          <div
            key={m.id}
            className={`toast ${m.tone}`}
            role={m.tone === "error" ? "alert" : "status"}
          >
            {m.tone === "error" ? (
              <AlertCircle size={18} />
            ) : (
              <CheckCircle2 size={18} />
            )}
            <span>{m.text}</span>
            <button
              aria-label="Dispensar mensagem"
              onClick={() => setMessages((p) => p.filter((t) => t.id !== m.id))}
            >
              <X size={15} />
            </button>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}
