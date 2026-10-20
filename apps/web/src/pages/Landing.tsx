import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import {
  ArrowDown,
  ArrowRight,
  Check,
  ChevronDown,
  FileCheck2,
  Layers,
  Link2,
  Repeat2,
} from "lucide-react";
import { Button, Logo } from "@/components/shared";
import { post } from "@/lib/api";
import type { Session } from "@/lib/types";
import "./landing.css";

const questions = [
  [
    "Meus clientes precisam criar uma conta?",
    "Não. O cliente acessa o link da solicitação, envia os documentos pelo celular ou computador e acompanha o que ainda falta. Sem cadastro e sem instalar aplicativo.",
  ],
  [
    "Como funciona o envio por WhatsApp?",
    "O DocPing abre o WhatsApp com uma mensagem pronta e o link da solicitação. Você confere e confirma o envio. Os lembretes automáticos são enviados por e-mail; o WhatsApp é compartilhado manualmente.",
  ],
  [
    "Posso usar a mesma lista todos os meses?",
    "Sim. Salve uma lista como modelo ou configure uma recorrência mensal. O DocPing cria as próximas solicitações para os clientes escolhidos, na data que você definir.",
  ],
  [
    "E se o cliente enviar um documento errado?",
    "Você solicita uma correção no próprio documento e informa o motivo. O cliente vê o que precisa ajustar e reenvia somente aquele item. O restante do trabalho fica preservado.",
  ],
  [
    "Posso conhecer antes de cadastrar meus clientes?",
    "Sim. A demonstração abre um escritório de exemplo com clientes e documentos fictícios. Explore o fluxo e, quando fizer sentido, crie o espaço do seu escritório.",
  ],
];

export function Landing() {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [previewAvailable, setPreviewAvailable] = useState(true);
  async function openDemo() {
    setLoading(true);
    setError("");
    try {
      const session = await post<Session>("/auth/demo");
      queryClient.clear();
      queryClient.setQueryData(["session"], session);
      navigate("/app");
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Não foi possível abrir a demonstração. Tente novamente.",
      );
    } finally {
      setLoading(false);
    }
  }
  return (
    <div className="lp-page">
      <a className="skip-link" href="#landing-content">
        Pular para o conteúdo
      </a>
      <header className="lp-header">
        <Link to="/" className="lp-logo" aria-label="DocPing — início">
          <Logo />
        </Link>
        <nav aria-label="Navegação principal">
          <a href="#como-funciona">Como funciona</a>
          <a href="#para-seu-escritorio">Para seu escritório</a>
        </nav>
        <div className="lp-header-actions">
          <Link to="/entrar">Entrar</Link>
          <Link className="lp-small-cta" to="/cadastro">
            Criar meu escritório
            <ArrowRight size={13} />
          </Link>
        </div>
      </header>
      <main id="landing-content">
        <section className="lp-hero">
          <div className="lp-hero-eyebrow">
            <span />
            UM POUCO MAIS DE CALMA NO SEU FECHAMENTO
          </div>
          <h1>
            Seus clientes enviam.
            <br />
            Você acompanha.
            <br />
            <span>O DocPing organiza.</span>
          </h1>
          <p>
            Solicite documentos, acompanhe pendências e deixe
            <br className="lp-desktop-break" /> a rotina do seu escritório
            contábil mais leve.
          </p>
          <div className="lp-hero-actions">
            <Button loading={loading} onClick={() => void openDemo()}>
              Explorar demonstração
              <ArrowRight size={16} />
            </Button>
            <Link className="lp-secondary-link" to="/cadastro">
              Criar meu escritório
              <ArrowRight size={15} />
            </Link>
          </div>
          <p className="lp-demo-disclosure">
            Demonstração com clientes e arquivos fictícios. Pode explorar à
            vontade.
          </p>
          {error && (
            <p className="inline-error lp-demo-error" role="alert">
              {error}
            </p>
          )}
          <a className="lp-see-product" href="#produto">
            Conheça seu próximo espaço de trabalho
            <ArrowDown size={13} />
          </a>
        </section>
        <section
          className="lp-product-section"
          id="produto"
          aria-label="Prévia real do DocPing"
        >
          <div className="lp-browser-frame">
            <div className="lp-browser-chrome">
              <span className="lp-window-dots">
                <i />
                <i />
                <i />
              </span>
              <span className="lp-browser-title">
                docping · seu espaço de trabalho
              </span>
              <span className="lp-browser-demo">DEMONSTRAÇÃO</span>
            </div>
            {previewAvailable ? (
              <img
                className="lp-product-image"
                src="/product-preview.png"
                alt="Tela real do DocPing com o fechamento mensal, clientes com pendências e documentos aguardando revisão, em um escritório de demonstração."
                width="1440"
                height="1000"
                fetchPriority="high"
                onError={() => setPreviewAvailable(false)}
              />
            ) : (
              <div className="lp-preview-fallback">
                <FileCheck2 size={35} strokeWidth={1.2} />
                <h2>Seu fechamento em um só lugar.</h2>
                <p>
                  Entre na demonstração para conhecer a experiência completa.
                </p>
                <Button
                  variant="secondary"
                  loading={loading}
                  onClick={() => void openDemo()}
                >
                  Abrir demonstração
                  <ArrowRight size={15} />
                </Button>
              </div>
            )}
          </div>
          <div className="lp-product-caption">
            <span>
              {previewAvailable
                ? "Tela real do produto. Dados fictícios para demonstração."
                : "Explore a demonstração com clientes e documentos fictícios."}
            </span>
            <span>Feito para o trabalho de todos os meses.</span>
          </div>
        </section>
        <section className="lp-workflow lp-container" id="como-funciona">
          <div className="lp-section-heading">
            <span className="lp-eyebrow">PEDIR. RECEBER. CONFERIR.</span>
            <h2>
              Um caminho curto.
              <br />
              <span>Da solicitação ao documento certo.</span>
            </h2>
            <p>
              Sem procurar arquivos em várias conversas.
              <br />
              Cada cliente sabe o que enviar. Você sabe o que falta.
            </p>
          </div>
          <div className="lp-step-grid">
            {[
              {
                number: "01",
                icon: Layers,
                title: "Peça o que precisa.",
                text: "Escolha os clientes, use um modelo de documentos e defina o prazo. Sua lista está pronta para compartilhar.",
              },
              {
                number: "02",
                icon: Link2,
                title: "Compartilhe um link.",
                text: "Seu cliente abre pelo celular e envia os arquivos, um por vez. Pode sair e continuar depois, sem perder o que já enviou.",
              },
              {
                number: "03",
                icon: FileCheck2,
                title: "Confira e siga em frente.",
                text: "Veja o que chegou, aprove os documentos ou explique uma correção. Tudo fica junto da solicitação.",
              },
            ].map((step) => (
              <article className="lp-step" key={step.number}>
                <div className="lp-step-top">
                  <span className="mono">{step.number}</span>
                  <step.icon size={24} strokeWidth={1.25} />
                </div>
                <h3>{step.title}</h3>
                <p>{step.text}</p>
              </article>
            ))}
          </div>
        </section>
        <section className="lp-for-office" id="para-seu-escritorio">
          <div className="lp-container lp-office-layout">
            <div className="lp-office-copy">
              <span className="lp-eyebrow">
                PARA QUEM CUIDA DE TANTAS EMPRESAS
              </span>
              <h2>
                O fechamento já tem
                <br />
                trabalho suficiente.
                <br />
                <span>Cobrar documento pode ser mais simples.</span>
              </h2>
              <p>
                O DocPing cuida da organização da coleta, para você dedicar
                atenção ao que precisa do seu olhar.
              </p>
              <Link to="/cadastro">
                Conheça o piloto fundador
                <ArrowRight size={16} />
              </Link>
            </div>
            <div className="lp-benefit-list">
              {[
                {
                  title: "Enxergue as pendências.",
                  text: "Clientes atrasados e documentos recebidos aparecem na mesma central de trabalho.",
                },
                {
                  title: "Dê contexto à correção.",
                  text: "Um motivo claro em cada documento. O cliente ajusta o item certo e continua de onde parou.",
                },
                {
                  title: "Deixe o próximo mês encaminhado.",
                  text: "Modelos reutilizáveis e recorrências para repetir o que funciona sem recadastrar a lista.",
                },
              ].map((benefit) => (
                <div className="lp-benefit" key={benefit.title}>
                  <span>
                    <Check size={16} strokeWidth={1.8} />
                  </span>
                  <section>
                    <h3>{benefit.title}</h3>
                    <p>{benefit.text}</p>
                  </section>
                </div>
              ))}
              <div className="lp-routine-note">
                <Repeat2 size={16} />
                <span>Uma rotina consistente, mês após mês.</span>
              </div>
            </div>
          </div>
        </section>
        <section className="lp-faq lp-container">
          <div className="lp-section-heading">
            <span className="lp-eyebrow">ANTES DE COMEÇAR</span>
            <h2>Ficou alguma dúvida?</h2>
            <p>Respostas diretas, como a rotina pede.</p>
          </div>
          <div className="lp-faq-list">
            {questions.map(([question, answer]) => (
              <details key={question}>
                <summary>
                  {question}
                  <ChevronDown size={17} />
                </summary>
                <p>{answer}</p>
              </details>
            ))}
          </div>
        </section>
        <section className="lp-final-cta lp-container">
          <span className="lp-eyebrow">
            SEU PRÓXIMO FECHAMENTO COMEÇA AGORA
          </span>
          <h2>
            Menos “você já enviou?”.
            <br />
            <span>Mais “está tudo aqui”.</span>
          </h2>
          <p>
            Conheça o DocPing com um escritório de exemplo.
            <br />
            Depois, faça dele o seu espaço.
          </p>
          <div className="lp-hero-actions">
            <Button loading={loading} onClick={() => void openDemo()}>
              Explorar demonstração
              <ArrowRight size={16} />
            </Button>
            <Link className="lp-secondary-link" to="/cadastro">
              Criar meu escritório
              <ArrowRight size={15} />
            </Link>
          </div>
          <p className="lp-demo-disclosure">
            A demonstração usa somente dados fictícios.
          </p>
          {error && (
            <p className="inline-error lp-demo-error" role="alert">
              {error}
            </p>
          )}
        </section>
      </main>
      <footer className="lp-footer lp-container">
        <Link to="/" className="lp-logo" aria-label="DocPing — início">
          <Logo small />
        </Link>
        <p>Simples na utilização. Cuidadoso em cada detalhe.</p>
        <Link to="/entrar">
          Acessar meu escritório
          <ArrowRight size={13} />
        </Link>
      </footer>
    </div>
  );
}
