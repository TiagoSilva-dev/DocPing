import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { MailWarning, ArrowRight } from "lucide-react";
import { api } from "@/lib/api";
import { latestFailures, type Delivery } from "@/lib/notifications";
import { CommunicationHistory } from "./CommunicationHistory";
export function CommunicationAlert() {
  const [open, setOpen] = useState(false);
  const { data = [] } = useQuery({
    queryKey: ["notifications"],
    queryFn: () => api<Delivery[]>("/notifications"),
  });
  const failures = latestFailures(data);
  if (!failures.length && !open) return null;
  return (
    <>
      <div className="communication-alert">
        <MailWarning size={17} />
        <div>
          <strong>
            {failures.length === 1
              ? "Um e-mail precisa da sua atenção."
              : `${failures.length} e-mails precisam da sua atenção.`}
          </strong>
          <span>O provedor não confirmou o envio. Confira as tentativas.</span>
        </div>
        <button onClick={() => setOpen(true)}>
          Ver detalhes
          <ArrowRight size={13} />
        </button>
      </div>
      {open && <CommunicationHistory onClose={() => setOpen(false)} />}
    </>
  );
}
