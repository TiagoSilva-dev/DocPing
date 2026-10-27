import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { Cookie } from "lucide-react";
import { deny, grant, restore, stored } from "@/lib/consent";
import { hasMeasurement } from "@/lib/site";
import "./cookie-notice.css";

// The consent notice. It appears only when there is something to consent to:
// a build without measurement ids has no third party to ask about, and a
// banner there would be theatre.
//
// Refusing is one click, in the same place and the same size as accepting.
// The site works identically either way, so there is nothing to win by
// hiding the no — and a notice that buries it is the kind the ANPD writes
// about.
export function CookieNotice() {
  const [answer, setAnswer] = useState(() => stored());
  useEffect(() => {
    restore();
  }, []);
  if (!hasMeasurement || answer) return null;
  return (
    <div className="ck" role="dialog" aria-label="Aviso de cookies">
      <div className="ck-body">
        <Cookie size={18} strokeWidth={1.5} aria-hidden="true" />
        <p>
          Usamos cookies de medição do Google para entender de onde vêm as
          visitas. Nenhuma delas é carregada antes de você aceitar, e recusar
          não muda nada no site. Detalhes na{" "}
          <Link to="/privacidade">Política de Privacidade</Link>.
        </p>
      </div>
      <div className="ck-actions">
        <button
          type="button"
          className="ck-deny"
          onClick={() => {
            deny();
            setAnswer("denied");
          }}
        >
          Recusar
        </button>
        <button
          type="button"
          className="ck-grant"
          onClick={() => {
            grant();
            setAnswer("granted");
          }}
        >
          Aceitar
        </button>
      </div>
    </div>
  );
}
