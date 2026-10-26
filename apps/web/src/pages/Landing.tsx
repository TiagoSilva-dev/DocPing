import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import {
  ArrowDown,
  ArrowRight,
  BrainCircuit,
  Building2,
  Check,
  ChevronDown,
  CircleCheck,
  CircleX,
  Clock,
  Eye,
  FileCheck2,
  FileUp,
  KeyRound,
  Layers,
  Link2,
  Lock,
  Repeat2,
  ScanText,
  Scale,
  ShieldCheck,
  Server,
  TriangleAlert,
  X,
} from "lucide-react";
import { Button, Logo } from "@/components/shared";
import { post } from "@/lib/api";
import { PLANS as plans } from "@/lib/plans";
import {
  faqPageLd,
  softwareApplicationLd,
  usePageMeta,
  useJsonLd,
} from "@/lib/seo";
import type { Session } from "@/lib/types";
import "./landing.css";

// Everything claimed on this page must be true of the shipped product. The
// automated review is deliberately described as a recommendation the office
// may promote to a decision, never as a validated accuracy claim — see
// docs/06-validacao-da-entrega.md before strengthening any wording here.

const questions: [string, string][] = [
  [
    "Meus clientes precisam criar uma conta?",
    "Não. O cliente acessa o link da solicitação, envia os documentos pelo celular ou computador e acompanha o que ainda falta. Sem cadastro e sem instalar aplicativo.",
  ],
  [
    "A IA aprova documentos sozinha?",
    "Só se o seu escritório pedir. O padrão é o modo sugerir: a análise aparece como recomendação e a decisão continua sua. No modo automático, o DocPing aplica apenas vereditos em que todos os critérios concordam — qualquer dúvida vai para uma pessoa. Você pode rever qualquer decisão, e o histórico registra o que foi decidido pela análise e o que foi decidido por você.",
  ],
  [
    "Em que a análise se baseia?",
    "O texto do documento é extraído com OCR e comparado ao que foi pedido naquele item: se é o tipo certo, se o titular confere, se a competência bate, se está completo, se está legível e se há algum impedimento. Cada critério tem uma confiança própria, e todos ficam visíveis na tela de revisão.",
  ],
  [
    "E se a análise errar?",
    "Aprovar e reprovar continuam sendo seus, a qualquer momento. Quando um documento é devolvido, o cliente vê o motivo e reenvia apenas aquele item. Se preferir, desligue a análise em Configurações sem perder nada do restante.",
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
    "Onde os arquivos ficam guardados?",
    "Em armazenamento privado, nunca em link público. Só PDF, JPG, PNG e HEIC são aceitos, conferidos pelo conteúdo e não apenas pela extensão, e o arquivo só é servido a quem tem sessão no seu escritório, com os dados de cada escritório isolados dos demais.",
  ],
  [
    "Posso conhecer antes de cadastrar meus clientes?",
    "Sim. A demonstração abre um escritório de exemplo com clientes e documentos fictícios. Explore o fluxo e, quando fizer sentido, crie o espaço do seu escritório.",
  ],
];

const criteria = [
  ["Tipo do documento", "É mesmo o documento que foi pedido naquele item."],
  ["Titular", "O nome ou CNPJ no documento é o do cliente da solicitação."],
  ["Competência", "O período do documento é o da solicitação."],
  ["Integridade", "Está completo, sem páginas faltando ou conteúdo cortado."],
  ["Legibilidade", "Dá para ler — inclusive em foto tirada pelo celular."],
  ["Impedimentos", "Documento vencido, rasurado ou com indício de adulteração."],
];

const comparison = [
  ["Cobrar cliente por cliente, à mão", "Lembretes automáticos por e-mail, no ritmo que você definir"],
  ["Arquivo perdido no meio da conversa", "Cada arquivo preso ao item que foi pedido"],
  ["Conferir tudo na unha, documento por documento", "Análise automática com os seis critérios já verificados"],
  ["Descobrir o documento errado no fechamento", "Documento fora do padrão devolvido na hora do envio"],
  ["Montar pasta e renomear arquivo", "Exportação em ZIP, organizada, a um clique"],
  ["Não saber quem aprovou o quê", "Histórico com autor, motivo e data em cada decisão"],
];

export function Landing() {
  usePageMeta({
    title: "DocPing — Coleta de documentos do cliente com conferência por IA",
    description:
      "Solicite documentos por link, receba sem o cliente criar conta e deixe a primeira conferência com a análise automática. Feito para o fechamento contábil.",
    path: "/",
    index: true,
  });
  // Built from the arrays this page renders, so the markup search engines read
  // and the page a person reads can never disagree.
  useJsonLd("ld-aplicacao", softwareApplicationLd(plans));
  useJsonLd("ld-faq", faqPageLd(questions));
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
  const demoActions = (
    <>
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
      {error && (
        <p className="inline-error lp-demo-error" role="alert">
          {error}
        </p>
      )}
    </>
  );
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
          <a href="#analise-ia">Análise por IA</a>
          <a href="#precos">Preços</a>
          <a href="#seguranca">Segurança</a>
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
            <br />A IA confere.
            <br />
            <span>Você decide o que importa.</span>
          </h1>
          <p>
            Solicite documentos, receba por link e deixe a conferência
            <br className="lp-desktop-break" /> repetitiva com o DocPing. O seu
            olhar fica para o que pede julgamento.
          </p>
          {demoActions}
          <p className="lp-demo-disclosure">
            Demonstração com clientes e arquivos fictícios. Pode explorar à
            vontade.
          </p>
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
        <section className="lp-problem lp-container">
          <div className="lp-section-heading">
            <span className="lp-eyebrow">O FECHAMENTO DE TODO MÊS</span>
            <h2>
              O problema nunca foi o cliente.
              <br />
              <span>É a conferência que não escala.</span>
            </h2>
            <p>
              Cobrar é só a primeira parte. A segunda, mais cara, é abrir um por
              um para ver se veio o documento certo.
            </p>
          </div>
          <div className="lp-pain-grid">
            {[
              {
                title: "A cobrança manual.",
                text: "Mensagem atrás de mensagem, cliente por cliente, até o último dia do prazo.",
              },
              {
                title: "O arquivo solto.",
                text: "Documento no WhatsApp, no e-mail e no Drive. Na hora de fechar, falta justo um.",
              },
              {
                title: "A conferência no olho.",
                text: "Competência trocada, foto ilegível, documento de outra empresa. Tudo isso só aparece quando alguém abre o arquivo.",
              },
            ].map((pain) => (
              <article className="lp-pain" key={pain.title}>
                <span className="lp-pain-mark">
                  <X size={15} strokeWidth={2} />
                </span>
                <h3>{pain.title}</h3>
                <p>{pain.text}</p>
              </article>
            ))}
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
                text: "A análise chega com o documento. Você confirma, ajusta ou pede correção — e tudo fica junto da solicitação.",
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
        <section className="lp-ai" id="analise-ia">
          <div className="lp-container">
            <div className="lp-section-heading lp-ai-heading">
              <span className="lp-eyebrow">
                <Sparkle /> ANÁLISE AUTOMÁTICA DE DOCUMENTOS
              </span>
              <h2>
                Cada documento chega
                <br />
                <span>já conferido.</span>
              </h2>
              <p>
                Assim que o cliente envia, o DocPing lê o documento, compara com
                o que foi pedido e devolve um parecer com os critérios abertos.
                Você vê o porquê, não só o resultado.
              </p>
            </div>
            <div className="lp-pipeline" aria-label="Etapas da análise automática">
              <div className="lp-pipe-step">
                <FileUp size={21} strokeWidth={1.3} />
                <h3>Upload do documento</h3>
                <p>PDF, JPG, PNG ou HEIC — inclusive foto tirada na hora</p>
              </div>
              <span className="lp-pipe-arrow" aria-hidden="true">
                <ArrowDown size={15} />
              </span>
              <div className="lp-pipe-step">
                <ScanText size={21} strokeWidth={1.3} />
                <h3>Extração de informações</h3>
                <p>Leitura da estrutura do PDF e OCR para documento digitalizado</p>
              </div>
              <span className="lp-pipe-arrow" aria-hidden="true">
                <ArrowDown size={15} />
              </span>
              <div className="lp-pipe-step lp-pipe-highlight">
                <BrainCircuit size={21} strokeWidth={1.3} />
                <h3>Análise de conformidade</h3>
                <p>Seis critérios julgados em separado, cada um com sua confiança</p>
              </div>
              <span className="lp-pipe-arrow" aria-hidden="true">
                <ArrowDown size={15} />
              </span>
              <div className="lp-outcomes">
                <div className="lp-outcome lp-outcome-ok">
                  <CircleCheck size={19} strokeWidth={1.6} />
                  <strong>Aprovado</strong>
                  <span>Todos os critérios conferem</span>
                </div>
                <div className="lp-outcome lp-outcome-review">
                  <TriangleAlert size={19} strokeWidth={1.6} />
                  <strong>Revisão</strong>
                  <span>Na dúvida, vai para você</span>
                </div>
                <div className="lp-outcome lp-outcome-no">
                  <CircleX size={19} strokeWidth={1.6} />
                  <strong>Reprovado</strong>
                  <span>Correção pedida com o motivo</span>
                </div>
              </div>
            </div>
            <div className="lp-ai-detail">
              <div className="lp-criteria">
                <h3>O que é verificado</h3>
                <ul>
                  {criteria.map(([name, text]) => (
                    <li key={name}>
                      <Check size={14} strokeWidth={2} />
                      <div>
                        <strong>{name}</strong>
                        <span>{text}</span>
                      </div>
                    </li>
                  ))}
                </ul>
              </div>
              <div className="lp-modes">
                <h3>Quem decide é você</h3>
                <p className="lp-modes-intro">
                  A análise é uma recomendação até o seu escritório dizer o
                  contrário. São três modos, trocados a qualquer momento em
                  Configurações.
                </p>
                <div className="lp-mode">
                  <span className="mono">01</span>
                  <div>
                    <strong>Sugerir — o padrão</strong>
                    <span>
                      O parecer aparece junto do documento. Aprovar e reprovar
                      continuam com você.
                    </span>
                  </div>
                </div>
                <div className="lp-mode">
                  <span className="mono">02</span>
                  <div>
                    <strong>Automático</strong>
                    <span>
                      Vereditos em que todos os critérios concordam são
                      aplicados sozinhos. O resto espera por você.
                    </span>
                  </div>
                </div>
                <div className="lp-mode">
                  <span className="mono">03</span>
                  <div>
                    <strong>Desligado</strong>
                    <span>
                      Nenhum documento é analisado, e nada sai do seu
                      escritório.
                    </span>
                  </div>
                </div>
                <p className="lp-modes-note">
                  <ShieldCheck size={15} strokeWidth={1.6} />
                  Falha de serviço, resposta incompleta ou confiança baixa nunca
                  viram aprovação: o documento vai para revisão humana.
                </p>
              </div>
            </div>
          </div>
        </section>
        <section className="lp-compare lp-container">
          <div className="lp-section-heading">
            <span className="lp-eyebrow">A DIFERENÇA NA ROTINA</span>
            <h2>
              O mesmo fechamento,
              <br />
              <span>com bem menos atrito.</span>
            </h2>
          </div>
          <div className="lp-compare-table">
            <div className="lp-compare-head">
              <span>Hoje, sem o DocPing</span>
              <span>Com o DocPing</span>
            </div>
            {comparison.map(([before, after]) => (
              <div className="lp-compare-row" key={before}>
                <div className="lp-compare-before">
                  <X size={14} strokeWidth={2} />
                  <span>{before}</span>
                </div>
                <div className="lp-compare-after">
                  <Check size={14} strokeWidth={2} />
                  <span>{after}</span>
                </div>
              </div>
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
                <span>Conferir documento pode ser mais simples.</span>
              </h2>
              <p>
                O DocPing cuida da coleta e da primeira conferência, para você
                dedicar atenção ao que precisa do seu olhar.
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
                {
                  title: "Entregue tudo de uma vez.",
                  text: "Exportação em ZIP com os arquivos da solicitação, prontos para o seu sistema.",
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
        <section className="lp-segments lp-container">
          <div className="lp-section-heading">
            <span className="lp-eyebrow">FEITO PARA</span>
            <h2>Quem depende de documento de cliente.</h2>
            <p>
              O DocPing nasceu no fechamento contábil, e serve a qualquer rotina
              em que a mesma lista se repete todo mês.
            </p>
          </div>
          <div className="lp-segment-grid">
            {[
              {
                icon: Building2,
                title: "Contabilidade e BPO",
                text: "Fechamento mensal, notas, extratos e folha, cliente a cliente.",
              },
              {
                icon: Scale,
                title: "Advocacia e consultoria",
                text: "Documentação de processo e abertura de empresa, com prazo e histórico.",
              },
              {
                icon: KeyRound,
                title: "Imobiliárias e locação",
                text: "Documentos de locatário e fiador reunidos antes da assinatura.",
              },
            ].map((segment) => (
              <article className="lp-segment" key={segment.title}>
                <segment.icon size={22} strokeWidth={1.25} />
                <h3>{segment.title}</h3>
                <p>{segment.text}</p>
              </article>
            ))}
          </div>
        </section>
        <section className="lp-security" id="seguranca">
          <div className="lp-container lp-security-layout">
            <div className="lp-security-copy">
              <span className="lp-eyebrow">DOCUMENTO DE CLIENTE É SÉRIO</span>
              <h2>
                Guardado como
                <br />
                <span>você guardaria.</span>
              </h2>
              <p>
                Documento de cliente não é arquivo qualquer. O cuidado está no
                produto desde o primeiro dia, não como item de uma lista.
              </p>
            </div>
            <div className="lp-security-grid">
              {[
                {
                  icon: Lock,
                  title: "Armazenamento privado",
                  text: "Arquivos nunca ficam em link público. Só são servidos a quem tem sessão no seu escritório.",
                },
                {
                  icon: ShieldCheck,
                  title: "Tipo de arquivo conferido",
                  text: "Só PDF, JPG, PNG e HEIC entram, verificados pela assinatura do conteúdo e não apenas pela extensão.",
                },
                {
                  icon: Eye,
                  title: "Link próprio por solicitação",
                  text: "Cada solicitação tem seu link, revogável a qualquer momento, com código por e-mail quando você quiser.",
                },
                {
                  icon: Clock,
                  title: "Acesso protegido",
                  text: "Senha com Argon2id, limite de tentativas e verificação de origem a cada requisição.",
                },
                {
                  icon: Server,
                  title: "Dados isolados por escritório",
                  text: "Cada escritório enxerga apenas o que é seu, verificado a cada requisição.",
                },
                {
                  icon: FileCheck2,
                  title: "Histórico de decisões",
                  text: "Quem aprovou, quando e por quê — inclusive quando a decisão foi automática.",
                },
              ].map((item) => (
                <article className="lp-security-item" key={item.title}>
                  <item.icon size={17} strokeWidth={1.4} />
                  <div>
                    <h3>{item.title}</h3>
                    <p>{item.text}</p>
                  </div>
                </article>
              ))}
            </div>
          </div>
        </section>
        <section className="lp-pricing lp-container" id="precos">
          <div className="lp-section-heading">
            <span className="lp-eyebrow">PLANOS</span>
            <h2>
              A partir de R$ 97 por mês.
              <br />
              <span>Com a análise já inclusa.</span>
            </h2>
            <p>
              A conferência automática entra em todos os planos — ela é o
              produto, não um adicional. O que muda é o volume.
            </p>
          </div>
          <div className="lp-plan-grid">
            {plans.map((plan) => (
              <article
                className={`lp-plan${plan.featured ? " lp-plan-featured" : ""}`}
                key={plan.id}
              >
                {plan.featured && (
                  <span className="lp-plan-tag">Mais escolhido</span>
                )}
                <h3>{plan.name}</h3>
                <p className="lp-plan-pitch">{plan.pitch}</p>
                <div className="lp-plan-price">
                  <span className="lp-plan-currency">R$</span>
                  <strong>{plan.price}</strong>
                  <span className="lp-plan-period">/mês</span>
                </div>
                <ul>
                  {plan.items.map((item) => (
                    <li key={item}>
                      <Check size={14} strokeWidth={2} />
                      {item}
                    </li>
                  ))}
                </ul>
                <Link
                  className={`lp-plan-cta${plan.featured ? " lp-plan-cta-featured" : ""}`}
                  to="/cadastro"
                >
                  Criar meu escritório
                  <ArrowRight size={14} />
                </Link>
              </article>
            ))}
          </div>
          <p className="lp-plan-note">
            Quer conhecer antes de assinar? A demonstração abre na hora, com
            dados fictícios e sem cadastro. Todos os planos são mensais, sem
            fidelidade, e o cancelamento é feito por você a qualquer momento.
            Escritórios do piloto fundador mantêm o preço de entrada enquanto
            forem clientes.
          </p>
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
          {demoActions}
          <p className="lp-demo-disclosure">
            A demonstração usa somente dados fictícios.
          </p>
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

// Small inline mark used in the automated-review eyebrow, kept local so the
// section reads as one piece without pulling another icon into the bundle.
function Sparkle() {
  return (
    <svg width="11" height="11" viewBox="0 0 12 12" aria-hidden="true">
      <path
        d="M6 0.6 7.3 4.7 11.4 6 7.3 7.3 6 11.4 4.7 7.3 0.6 6 4.7 4.7Z"
        fill="currentColor"
      />
    </svg>
  );
}
