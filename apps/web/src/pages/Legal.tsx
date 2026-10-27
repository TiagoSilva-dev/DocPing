import { Link } from "react-router-dom";
import { ArrowLeft, ArrowRight } from "lucide-react";
import { Logo } from "@/components/shared";
import { usePageMeta } from "@/lib/seo";
import {
  COMPANY_ADDRESS,
  COMPANY_LEGAL_NAME,
  COMPANY_TAX_ID,
  CONTACT_EMAIL,
  POLICY_DATE,
  SUPPORT_EMAIL,
} from "@/lib/site";
import "./legal.css";

// The privacy policy and the terms of use. Google Ads refuses a site without
// both, and the LGPD requires the first one regardless of advertising.
//
// Everything here must describe the product that shipped. In particular: the
// absence of malware scanning is stated, the automated analysis is described
// as a recommendation and never as a professional opinion, and no
// certification or compliance seal is claimed. See
// docs/05-seguranca-e-piloto.md before softening any of it.

function LegalPage({
  title,
  lead,
  children,
}: {
  title: string;
  lead: string;
  children: React.ReactNode;
}) {
  return (
    <div className="lg-page">
      <header className="lg-top">
        <Link to="/" className="lg-back">
          <ArrowLeft size={14} />
          Voltar ao site
        </Link>
        <Link to="/" aria-label="DocPing — início">
          <Logo small />
        </Link>
      </header>
      <main className="lg-main">
        <p className="lg-eyebrow">DOCPING</p>
        <h1>{title}</h1>
        <p className="lg-lead">{lead}</p>
        <p className="lg-date">Em vigor desde {POLICY_DATE}.</p>
        {children}
        <Identification />
        <footer className="lg-footer">
          <Link to="/privacidade">Política de Privacidade</Link>
          <Link to="/termos">Termos de Uso</Link>
          <Link to="/">
            Início
            <ArrowRight size={12} />
          </Link>
        </footer>
      </main>
    </div>
  );
}

// The controller has to be identifiable for a data subject to exercise any
// right. The legal name, tax id and address come from the build environment:
// an empty one prints nothing rather than a placeholder that reads as a fact.
function Identification() {
  return (
    <section className="lg-id">
      <h2>Quem responde por este site</h2>
      <p>
        {COMPANY_LEGAL_NAME || "DocPing"}
        {COMPANY_TAX_ID && <> — CNPJ {COMPANY_TAX_ID}</>}
        {COMPANY_ADDRESS && <>, {COMPANY_ADDRESS}</>}.
      </p>
      <p>
        Contato geral: <a href={`mailto:${CONTACT_EMAIL}`}>{CONTACT_EMAIL}</a>.
        Encarregado de dados (DPO) e pedidos da LGPD:{" "}
        <a href={`mailto:${SUPPORT_EMAIL}`}>{SUPPORT_EMAIL}</a>.
      </p>
    </section>
  );
}

export function PrivacyPage() {
  usePageMeta({
    title: "Política de Privacidade — DocPing",
    description:
      "Quais dados o DocPing coleta, para que usa, com quem compartilha, por quanto tempo guarda e como você exerce os direitos da LGPD.",
  });
  return (
    <LegalPage
      title="Política de Privacidade"
      lead="O DocPing existe para receber documentos de terceiros. Por isso esta página é específica: ela diz o que fazemos com os dados do escritório, o que fazemos com os documentos dos clientes dele e o que não fazemos com nenhum dos dois."
    >
      <section>
        <h2>1. Dois papéis diferentes</h2>
        <p>
          Quando o assunto é a conta do escritório — nome, e-mail, senha,
          assinatura, registros de acesso — nós somos o{" "}
          <strong>controlador</strong> desses dados e respondemos por eles
          diretamente.
        </p>
        <p>
          Quando o assunto são os documentos e os dados dos clientes do
          escritório, nós somos <strong>operador</strong>: o escritório decide o
          que pedir, de quem pedir e por quanto tempo guardar, e nós tratamos
          esses dados apenas para executar o serviço que ele contratou. Se você
          é cliente de um escritório e quer saber por que um documento foi
          pedido, quem responde é o escritório.
        </p>
      </section>
      <section>
        <h2>2. O que coletamos</h2>
        <ul>
          <li>
            <strong>Do escritório:</strong> nome, e-mail, senha (guardada como
            hash Argon2id, nunca em texto), nome do escritório, preferências de
            notificação e, quando há assinatura, os dados de cobrança
            processados pela Stripe.
          </li>
          <li>
            <strong>Dos clientes cadastrados pelo escritório:</strong> nome ou
            razão social, CNPJ, nome do responsável, e-mail e telefone —
            fornecidos pelo escritório, não coletados por nós.
          </li>
          <li>
            <strong>Dos documentos enviados:</strong> o arquivo em si (PDF, JPG,
            PNG ou HEIC), seu nome, tamanho, tipo e um hash do conteúdo.
            Documentos contábeis e fiscais podem conter dados pessoais, e é o
            escritório que define quais são pedidos.
          </li>
          <li>
            <strong>Registros técnicos:</strong> data, hora, endereço IP e
            resultado das tentativas de acesso e dos envios, usados para
            segurança, limite de requisições e auditoria.
          </li>
          <li>
            <strong>Medição do site público:</strong> somente se você aceitar o
            aviso de cookies. Veja o item 5.
          </li>
        </ul>
        <p>
          Não pedimos nem usamos dados de crianças e adolescentes, não fazemos
          perfilamento publicitário com os documentos e não vendemos dado nenhum
          a ninguém, em nenhuma circunstância.
        </p>
      </section>
      <section>
        <h2>3. Para que usamos, e com que base legal</h2>
        <ul>
          <li>
            <strong>Executar o contrato</strong> (art. 7º, V da LGPD): criar e
            manter a conta, gerar solicitações, receber documentos, enviar
            avisos e lembretes, cobrar a assinatura.
          </li>
          <li>
            <strong>Cumprir obrigação legal</strong> (art. 7º, II): guardar
            registros de acesso conforme o Marco Civil da Internet e manter os
            documentos fiscais e contábeis exigidos de nós.
          </li>
          <li>
            <strong>Legítimo interesse</strong> (art. 7º, IX): segurança da
            plataforma, prevenção de abuso, limite de requisições e melhoria do
            serviço com dados agregados.
          </li>
          <li>
            <strong>Consentimento</strong> (art. 7º, I): apenas para os cookies
            de medição do site público, que você pode recusar sem perder nenhuma
            funcionalidade.
          </li>
        </ul>
      </section>
      <section>
        <h2>4. Análise automática de documentos</h2>
        <p>
          Quando o escritório ativa a conferência automática, o texto do
          documento é extraído e enviado ao serviço de inteligência artificial
          que usamos para produzir uma recomendação — se o tipo confere, se o
          titular confere, se a competência bate, se está completo, se está
          legível e se há impedimento.
        </p>
        <p>
          Essa recomendação{" "}
          <strong>não é parecer contábil, fiscal ou jurídico</strong> e não
          substitui a conferência de um profissional. O modo padrão apenas
          sugere, e a decisão é sempre de uma pessoa do escritório. No modo
          automático, escolhido pelo escritório, só são aplicados vereditos em
          que todos os critérios concordam; qualquer dúvida vai para revisão
          humana. Toda decisão fica registrada com sua origem, e o escritório
          pode revê-la. Os documentos não são usados para treinar modelos.
        </p>
        <p>
          Se a decisão automática afetar você, o art. 20 da LGPD lhe dá direito
          a revisão: peça ao escritório ou escreva para{" "}
          <a href={`mailto:${SUPPORT_EMAIL}`}>{SUPPORT_EMAIL}</a>.
        </p>
      </section>
      <section>
        <h2>5. Cookies e medição</h2>
        <p>
          O DocPing usa um cookie próprio de sessão (
          <code>docping_session</code>) e um token de proteção contra CSRF. São
          estritamente necessários: sem eles não há como manter você conectado,
          e por isso não dependem de consentimento.
        </p>
        <p>
          No site público usamos ferramentas de medição do Google (Google
          Analytics 4 e Google Ads) para saber de onde vêm as visitas e se um
          anúncio funcionou.{" "}
          <strong>
            Nenhuma dessas ferramentas é carregada antes de você aceitar
          </strong>{" "}
          — nem o script, nem um cookie, nem uma requisição. Se você recusar, ou
          simplesmente fechar sem responder, nenhum dado de medição sai daqui e
          o site funciona exatamente igual. Para mudar de ideia, limpe os dados
          deste site no seu navegador e o aviso aparecerá de novo.
        </p>
        <p>
          Uma ressalva honesta: as fontes tipográficas deste site vêm do Google
          Fonts, e o navegador as busca ao abrir a página, antes de qualquer
          resposta sua. Isso informa ao Google o seu endereço IP, não instala
          cookie nenhum e não serve para medir nem para anunciar — mas é uma
          requisição a um terceiro, e preferimos dizer a fingir que não existe.
        </p>
        <p>
          O portal onde o seu cliente envia documentos não tem nenhuma
          ferramenta de medição ou publicidade.
        </p>
      </section>
      <section>
        <h2>6. Com quem compartilhamos</h2>
        <p>
          Só com quem é necessário para o serviço funcionar, e sempre pelo
          mínimo necessário:
        </p>
        <ul>
          <li>
            <strong>Provedor de hospedagem e banco de dados</strong> —
            infraestrutura onde a aplicação roda.
          </li>
          <li>
            <strong>Provedor de armazenamento de objetos</strong> — onde os
            arquivos ficam, em bucket privado e criptografados em repouso.
          </li>
          <li>
            <strong>Provedor de e-mail transacional</strong> — para enviar
            convites, lembretes e confirmações.
          </li>
          <li>
            <strong>Stripe</strong> — cobrança da assinatura. Não recebemos nem
            guardamos o número do seu cartão.
          </li>
          <li>
            <strong>Serviço de análise por IA</strong> — recebe o texto extraído
            do documento quando a conferência automática está ativa.
          </li>
          <li>
            <strong>Google</strong> — medição do site público, somente após o
            seu aceite.
          </li>
        </ul>
        <p>
          Parte desses provedores opera fora do Brasil, o que caracteriza
          transferência internacional nos termos dos arts. 33 e seguintes da
          LGPD; exigimos deles cláusulas contratuais de proteção equivalentes.
          Além disso, podemos compartilhar dados por ordem judicial ou
          requisição de autoridade competente, e informaremos o titular sempre
          que a lei permitir.
        </p>
      </section>
      <section>
        <h2>7. Segurança — e o que ela não inclui</h2>
        <ul>
          <li>Tráfego sempre em HTTPS; senha com Argon2id.</li>
          <li>
            Arquivos em bucket privado, nunca em link público, servidos somente
            a quem tem sessão autorizada ou o link daquela solicitação.
          </li>
          <li>
            Só PDF, JPG, PNG e HEIC são aceitos, verificados pela assinatura do
            conteúdo e pela extensão, com limite de tamanho.
          </li>
          <li>
            Dados isolados por escritório, verificados em cada requisição;
            limite de tentativas de acesso e verificação de origem.
          </li>
          <li>
            Registro de quem decidiu o quê e quando, inclusive nas decisões
            automáticas.
          </li>
        </ul>
        <p>
          <strong>
            Não fazemos varredura antivírus nos arquivos enviados.
          </strong>{" "}
          Dizemos isso de forma clara porque o contrário seria uma promessa que
          não cumprimos: o que barra um arquivo aqui é o tipo, o tamanho, o
          bucket privado e a sessão. Não há, e não alegamos haver, certificação
          ou selo de conformidade de nenhuma entidade.
        </p>
      </section>
      <section>
        <h2>8. Por quanto tempo guardamos</h2>
        <ul>
          <li>
            <strong>Conta ativa:</strong> enquanto o escritório mantiver a
            conta. Os documentos ficam disponíveis para o escritório, que decide
            quando apagá-los.
          </li>
          <li>
            <strong>Demonstração:</strong> apagada automaticamente em até 24
            horas. Usa somente dados fictícios.
          </li>
          <li>
            <strong>Cadastro sem confirmação de e-mail:</strong> apagado
            automaticamente em até 7 dias.
          </li>
          <li>
            <strong>Conta encerrada:</strong> excluímos os dados em até 30 dias
            após o pedido, ressalvado o que a lei nos obriga a manter — como
            registros de acesso e documentos fiscais da própria assinatura.
          </li>
        </ul>
      </section>
      <section>
        <h2>9. Seus direitos</h2>
        <p>
          O art. 18 da LGPD lhe garante confirmação de tratamento, acesso,
          correção, anonimização, bloqueio ou eliminação, portabilidade,
          informação sobre compartilhamento, revisão de decisão automatizada e
          revogação do consentimento.
        </p>
        <p>
          Escreva para <a href={`mailto:${SUPPORT_EMAIL}`}>{SUPPORT_EMAIL}</a> e
          respondemos em até 15 dias. Se o pedido for sobre documentos enviados
          a um escritório, vamos encaminhá-lo a esse escritório, que é quem
          decide sobre eles. Você também pode reclamar à ANPD.
        </p>
      </section>
      <section>
        <h2>10. Mudanças nesta política</h2>
        <p>
          Se mudarmos algo relevante, avisamos por e-mail e atualizamos a data
          no alto desta página antes de a mudança valer. Versões anteriores
          ficam disponíveis a pedido.
        </p>
      </section>
    </LegalPage>
  );
}

export function TermsPage() {
  usePageMeta({
    title: "Termos de Uso — DocPing",
    description:
      "As regras do serviço DocPing: conta, planos e cancelamento, uso aceitável, responsabilidade pelos documentos e limites da conferência automática.",
  });
  return (
    <LegalPage
      title="Termos de Uso"
      lead="As regras de uso do DocPing, escritas para serem lidas uma vez e entendidas na primeira leitura. Ao criar uma conta você concorda com elas."
    >
      <section>
        <h2>1. O que é o serviço</h2>
        <p>
          O DocPing é um software que ajuda escritórios de contabilidade a
          pedir, receber e conferir documentos de seus clientes por um link, sem
          que o cliente precise criar conta. É uma ferramenta: não prestamos
          serviço contábil, fiscal, jurídico ou de auditoria, e não respondemos
          pelo conteúdo nem pela validade dos documentos que trafegam por ela.
        </p>
      </section>
      <section>
        <h2>2. Conta e responsabilidade</h2>
        <ul>
          <li>
            A conta é de pessoa jurídica ou profissional que atue no ramo, com
            no mínimo 18 anos, e exige confirmação do e-mail informado.
          </li>
          <li>
            Você é responsável por manter a senha em segredo e pelo que for
            feito com a sua conta. Suspeita de acesso indevido: escreva para{" "}
            <a href={`mailto:${SUPPORT_EMAIL}`}>{SUPPORT_EMAIL}</a>.
          </li>
          <li>
            Você declara ter base legal para pedir a terceiros os documentos que
            pedir, e para cadastrar os dados de contato de seus clientes. Como
            operador, seguimos as suas instruções; a licitude de pedir é sua.
          </li>
        </ul>
      </section>
      <section>
        <h2>3. Planos, pagamento e cancelamento</h2>
        <ul>
          <li>
            Os planos são mensais, sem fidelidade, cobrados pela Stripe, e os
            valores vigentes são os publicados na página de planos.
          </li>
          <li>
            O que cada plano define é o número de clientes atendidos. Ao atingir
            esse número, a plataforma deixa de aceitar clientes novos e o que já
            existe continua funcionando.
          </li>
          <li>
            <strong>
              O cancelamento é feito por você, a qualquer momento, pela própria
              plataforma
            </strong>
            , e vale até o fim do período já pago — sem multa e sem ligação.
          </li>
          <li>
            Nos 7 dias seguintes à primeira contratação você pode desistir e
            receber o valor de volta, como manda o art. 49 do Código de Defesa
            do Consumidor. Depois disso não há reembolso proporcional de mês em
            curso.
          </li>
          <li>
            Mudanças de preço são avisadas com no mínimo 30 dias de antecedência
            e nunca valem para um período já pago. Escritórios do piloto
            fundador mantêm o preço de entrada enquanto permanecerem clientes.
          </li>
          <li>
            Pagamento recusado: mantemos o acesso enquanto a cobrança está sendo
            retentada. Assinatura cancelada ou não paga suspende o acesso do
            escritório — mas nunca o portal onde o cliente envia documentos.
          </li>
        </ul>
      </section>
      <section>
        <h2>4. Uso aceitável</h2>
        <p>Usando o DocPing, você se compromete a não:</p>
        <ul>
          <li>
            enviar conteúdo ilícito, ou pedir documentos sem base legal para
            pedi-los;
          </li>
          <li>
            usar a plataforma para enviar mensagem não solicitada em massa a
            quem não é seu cliente;
          </li>
          <li>
            tentar burlar limites, sondar, sobrecarregar ou acessar dados de
            outro escritório;
          </li>
          <li>
            revender, sublicenciar ou expor o serviço como se fosse seu, salvo a
            personalização de marca que o próprio plano oferece;
          </li>
          <li>
            usar a demonstração com dados reais — ela é apagada em até 24 horas
            e existe para dados fictícios.
          </li>
        </ul>
        <p>
          Descumprimento grave ou reiterado permite suspensão da conta, com
          aviso sempre que for possível avisar.
        </p>
      </section>
      <section>
        <h2>5. Conferência automática</h2>
        <p>
          A análise automática produz uma <strong>recomendação</strong>, com uma
          confiança própria para cada critério, visível na tela de revisão. Ela
          não é parecer profissional, não tem exatidão garantida e seus limites
          não foram calibrados para o seu conjunto de documentos. O modo padrão
          apenas sugere. Se o escritório escolher o modo automático, a decisão
          continua sendo de responsabilidade do escritório, que pode revê-la a
          qualquer momento.
        </p>
      </section>
      <section>
        <h2>6. Seus dados continuam seus</h2>
        <p>
          Os documentos e os dados que você insere são seus e dos seus clientes.
          Não os usamos para treinar modelos, não os vendemos e não os acessamos
          além do necessário para operar e dar suporte — e, nesse caso, com
          registro. O software, a marca e o código do DocPing continuam nossos;
          a assinatura dá direito de uso, não de propriedade.
        </p>
        <p>
          Ao encerrar a conta, você pode exportar seus dados antes da exclusão.
          Veja prazos na <Link to="/privacidade">Política de Privacidade</Link>.
        </p>
      </section>
      <section>
        <h2>7. Disponibilidade e limites de responsabilidade</h2>
        <p>
          Trabalhamos para manter o serviço disponível, mas{" "}
          <strong>não oferecemos SLA contratual de disponibilidade</strong> e
          pode haver manutenção, falha de provedor ou indisponibilidade
          temporária. Avisamos o que for programado.
        </p>
        <p>
          O serviço é fornecido no estado em que se encontra. Na medida máxima
          permitida pela lei brasileira, nossa responsabilidade total por
          qualquer reclamação fica limitada ao valor que você pagou nos 12 meses
          anteriores ao fato, e não respondemos por lucros cessantes, perda de
          prazo fiscal, multa aplicada por terceiro ou dano indireto. Nada aqui
          afasta direitos que o Código de Defesa do Consumidor torne
          irrenunciáveis.
        </p>
        <p>
          <strong>Guarde seus próprios backups do que é essencial.</strong> A
          recuperação de backup da plataforma ainda não foi homologada, e
          dizemos isso em vez de deixar você descobrir.
        </p>
      </section>
      <section>
        <h2>8. Mudanças, lei aplicável e foro</h2>
        <p>
          Podemos alterar estes termos avisando por e-mail com no mínimo 30 dias
          de antecedência; continuar usando depois disso significa aceitar a
          nova versão, e discordar significa poder cancelar sem custo. Aplica-se
          a lei brasileira, e fica eleito o foro do domicílio do consumidor ou
          do contratante para qualquer discussão.
        </p>
        <p>
          Dúvida sobre estes termos:{" "}
          <a href={`mailto:${CONTACT_EMAIL}`}>{CONTACT_EMAIL}</a>.
        </p>
      </section>
    </LegalPage>
  );
}
