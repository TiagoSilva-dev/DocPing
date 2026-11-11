import { useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowRight, Check, LogOut } from "lucide-react";
import { Button, Logo } from "@/components/shared";
import { ApiError, getSession, post, startCheckout } from "@/lib/api";
import { checkoutStarted } from "@/lib/consent";
import { usePageMeta } from "@/lib/seo";
import { PLANS, planName, trialDaysLeft } from "@/lib/plans";
import "./subscribe.css";

// The screen a workspace without a subscription lands on. It is the only one
// such a workspace can reach, so it also has to carry the way out: the name
// of who is signed in, and a way to sign out.
export default function Subscribe() {
  usePageMeta({ title: "Assinar — DocPing" });
  const [params] = useSearchParams();
  const queryClient = useQueryClient();
  const [pending, setPending] = useState("");
  const [error, setError] = useState("");
  const { data: session } = useQuery({
    queryKey: ["session"],
    queryFn: getSession,
    retry: false,
  });
  const billing = session?.billing;
  const lapsed = Boolean(billing?.status) && !billing?.entitled;
  const trialDays = trialDaysLeft(billing?.trialEndsAt);
  // The trial ran out and nothing was bought: the most common way to arrive
  // here, and the one that most needs to hear that nothing was lost.
  const trialOver =
    Boolean(billing?.trialEndsAt) && trialDays === 0 && !billing?.entitled;

  async function choose(plan: string) {
    setError("");
    setPending(plan);
    try {
      // The server answers with Stripe's URL and the browser leaves. No card
      // detail is ever typed into this application.
      const { url } = await startCheckout(plan);
      checkoutStarted(
        plan,
        Number(PLANS.find((p) => p.id === plan)?.price ?? 0),
      );
      window.location.assign(url);
    } catch (e) {
      setPending("");
      setError(
        e instanceof ApiError
          ? e.message
          : "Não foi possível abrir o pagamento. Tente novamente.",
      );
    }
  }

  return (
    <div className="subscribe-page">
      <header className="subscribe-header">
        <Link to="/" aria-label="DocPing">
          <Logo />
        </Link>
        {session && (
          <div className="subscribe-identity">
            <span>{session.user.email}</span>
            <button
              type="button"
              className="subscribe-signout"
              onClick={async () => {
                await post("/auth/logout");
                queryClient.clear();
                window.location.assign("/entrar");
              }}
            >
              <LogOut size={14} aria-hidden />
              Sair
            </button>
          </div>
        )}
      </header>
      <main className="subscribe-main">
        <h1>
          {lapsed
            ? "Sua assinatura não está ativa."
            : trialOver
              ? "Seu teste grátis terminou."
              : "Escolha o plano do seu escritório."}
        </h1>
        <p className="subscribe-lead">
          {lapsed
            ? "Os documentos que você já recebeu continuam guardados. Para voltar a criar solicitações, reative a assinatura."
            : trialOver
              ? "Os clientes, as solicitações e os documentos que você recebeu continuam guardados. Escolha um plano para continuar de onde parou."
              : "A conferência automática entra em todos os planos. O que muda é quantos clientes o escritório atende."}
        </p>
        {trialDays > 0 && (
          <p className="subscribe-note" role="status">
            Seu teste grátis vai até o fim{" "}
            {trialDays === 1 ? "de hoje" : `dos próximos ${trialDays} dias`}.
            Assinar agora não interrompe nada.{" "}
            <Link to="/app">Voltar ao escritório</Link>.
          </p>
        )}
        {params.get("assinatura") === "cancelada" && (
          <p className="subscribe-note" role="status">
            O pagamento foi interrompido e nada foi cobrado. Você pode escolher
            um plano quando quiser.
          </p>
        )}
        {error && (
          <p className="subscribe-error" role="alert">
            {error}
          </p>
        )}
        <div className="subscribe-grid">
          {PLANS.map((plan) => (
            <article
              className={`subscribe-plan${plan.featured ? " subscribe-plan-featured" : ""}`}
              key={plan.id}
            >
              {plan.featured && (
                <span className="subscribe-tag">Mais escolhido</span>
              )}
              <h2>{plan.name}</h2>
              <p className="subscribe-pitch">{plan.pitch}</p>
              <div className="subscribe-price">
                <span>R$</span>
                <strong>{plan.price}</strong>
                <span>/mês</span>
              </div>
              <ul>
                {plan.items.map((item) => (
                  <li key={item}>
                    <Check size={14} strokeWidth={2} aria-hidden />
                    {item}
                  </li>
                ))}
              </ul>
              <Button
                onClick={() => void choose(plan.id)}
                disabled={Boolean(pending)}
                className={plan.featured ? "subscribe-cta-featured" : undefined}
              >
                {pending === plan.id ? "Abrindo pagamento…" : "Assinar"}
                <ArrowRight size={14} aria-hidden />
              </Button>
            </article>
          ))}
        </div>
        <p className="subscribe-foot">
          O pagamento é processado pela Stripe. Cancelamento a qualquer momento,
          pela própria área de cobrança.
          {billing?.manageable && (
            <>
              {" "}
              Já assinou antes?{" "}
              <Link to="/app/configuracoes">Ver a área de cobrança</Link>.
            </>
          )}
        </p>
        {billing?.plan && (
          <p className="subscribe-foot subscribe-current">
            Plano registrado hoje: {planName(billing.plan)}
            {billing.status ? ` (${billing.status})` : ""}.
          </p>
        )}
      </main>
    </div>
  );
}
