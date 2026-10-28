# 08 — SEO, presença pública e motores de resposta

A superfície pública do DocPing é **uma única página**: a landing em `/`. Todo o
resto — o espaço de trabalho, o portal do cliente e os formulários de acesso —
é privado ou raso demais para um resultado de busca, e os links do portal são
credenciais de acesso que nunca podem chegar a um índice.

## Domínio canônico

O endereço canônico é `https://docping.com.br`. Ele aparece, escrito por
extenso, em quatro lugares, e os quatro mudam juntos:

| Arquivo | O que carrega |
|---|---|
| `apps/web/index.html` | `<link rel="canonical">`, Open Graph, Twitter Card e o JSON-LD de `Organization`/`WebSite` |
| `apps/web/src/lib/answers.ts` | `SITE_URL`, base do canonical por rota, do JSON-LD e do `/llms.txt` (`src/lib/seo.ts` reexporta) |
| `apps/web/public/robots.txt` | linha `Sitemap:` |
| `apps/web/public/sitemap.xml` | a única `<loc>` |

O valor é estático de propósito. Crawlers de redes sociais e do WhatsApp não
executam JavaScript: as tags que eles leem precisam estar no HTML servido, e o
HTML é compilado na imagem (`apps/web/Dockerfile`), antes de qualquer variável
de ambiente de execução existir.

Desde 22/10/2026 o DNS aponta para o serviço `web` e o canonical responde:
`https://docping.com.br` serve a landing em HTTPS com certificado válido, e
`WEB_ORIGIN` já é esse endereço. O host `web-production-*.up.railway.app` foi
removido do serviço, então não há um segundo endereço servindo a mesma página —
que era o risco de conteúdo duplicado. Como chegar lá está em
[07 — Deploy](07-deploy.md).

Indexação é outra coisa, e segue pendente: nada foi submetido ao Search Console
e nenhuma busca real foi conferida.

## O que é indexável

Indexável: `/`, `/privacidade` e `/termos` — as três que estão no
`sitemap.xml`. Mais `/robots.txt`, `/sitemap.xml` e `/llms.txt`, que são
arquivos para máquina e não páginas.

Fora do índice, por três caminhos independentes, porque um só não basta quando
o que está em jogo é um link de portal:

1. `<meta name="robots" content="noindex, nofollow">` aplicado por rota em
   `src/lib/seo.ts`. Toda página chama `usePageMeta`, e `index` é opt-in: uma
   página nova nasce privada.
2. `robots.txt`, que desautoriza `/app`, `/p/`, `/portal/`, `/entrar`,
   `/cadastro`, `/esqueci-senha`, `/redefinir-senha`, `/confirmar-email`,
   `/assinar` e `/api/` — no grupo `*` e, repetida, no grupo dos crawlers de
   IA.
3. `X-Robots-Tag: noindex, nofollow` no nginx
   (`infra/web-nginx.conf.template`) para esses mesmos prefixos. É o único que
   vale para um crawler que ignora o `robots.txt` e não executa o JavaScript da
   página — e todas as rotas são servidas pelo mesmo `index.html`, então sem
   esse cabeçalho o HTML de um link de portal é idêntico ao da landing.

Os formulários de acesso ficam fora do índice por serem conteúdo raso, não por
sigilo. Se um dia `/cadastro` virar página de campanha, ela precisa de conteúdo
próprio antes de ser liberada.

## Dados estruturados

`Organization` e `WebSite` são estáticos no `index.html`: descrevem o site, não
uma página, e precisam existir até numa rota que não renderiza nada.
`SoftwareApplication` (com os preços), `FAQPage` e `HowTo` (os três passos) são
escritos **na build**, dentro do `<head>`, pelo plugin `docping:answer-engines`
em `apps/web/vite.config.ts`, a partir dos mesmos arrays que a landing
renderiza (`src/lib/answers.ts`). Antes eram injetados pelo React; não são mais,
porque o crawler que mais precisa deles é justamente o que não executa o
bundle — e emitir nos dois lugares daria dois `FAQPage` para o único que
executa. `apps/web/src/lib/answers.test.ts` cobre os três construtores e
`seo.test.ts` segue cobrindo os preços marcados.

Não há `aggregateRating` nem `review`: o produto não tem avaliações, e marcação
inventada é penalizada além de ser mentira. Os preços marcados são exatamente os
que a seção de planos imprime — e continuam sendo uma proposta comercial, não um
limite aplicado pela API (ver o comentário em `Landing.tsx`).

## Motores de resposta (GEO/AEO)

Buscador executa JavaScript; motor de resposta, em geral, não. O GPTBot, o
OAI-SearchBot, o PerplexityBot, o ClaudeBot e os buscadores que alimentam as
respostas de IA leem o HTML que chega pela rede — e o HTML de uma SPA é um
`<div id="root">` vazio. Sem o que está descrito abaixo, a resposta de um
desses motores sobre o DocPing seria inventada a partir de terceiros.

### Uma fonte só: `src/lib/answers.ts`

O módulo não depende de nada além de `./plans` de propósito: o
`vite.config.ts` o importa **em Node**, onde a substituição de
`import.meta.env` do Vite não roda e o React não tem o que fazer. Dele saem
três artefatos, e a landing renderiza os mesmos arrays — então a página, o
`/llms.txt` e a marcação não têm como discordar de uma frase:

| Artefato | Onde aparece | Quem escreve |
|---|---|---|
| Fonte de respostas em HTML | dentro do `<div id="root">` do `index.html` servido | `answerSourceHtml()`, na build |
| JSON-LD do produto, da FAQ e dos passos | `<head>` do `index.html` servido | `landingLd()`, na build |
| `/llms.txt` | raiz do site | `llmsTxt()`, na build (e um middleware no `vite dev`) |

As perguntas da FAQ, os seis critérios, os três passos, os segmentos e a lista
de segurança saíram do `Landing.tsx` e moraram ali. Na página ficaram só os
ícones, na ordem dos arrays que ilustram.

### A fonte de respostas pré-renderizada

`answerSourceHtml()` escreve, em HTML simples, o que a landing diz: o que é o
produto, para quem é, como funciona, o que a análise verifica, quem decide, os
preços, a segurança, **o que o DocPing não faz** e a FAQ inteira com cada
pergunta num `<h3>`. O React limpa o container ao montar, então uma pessoa só
vê esse bloco enquanto o bundle carrega — e quem está com o JavaScript
desligado lê a página em vez de uma tela branca. Não é escondido e não é
servido só para crawler: é a página, dita curta. O único estilo que ele usa
está inline no `index.html`, porque a folha de estilo real vem no bundle que
ainda não carregou.

O bloco é injetado **só na build**. O `vite dev` serve o HTML que sempre
serviu, para que um dia de trabalho e a suíte do Playwright vejam a mesma
página de antes. Conferir, depois de `npm run build`:

```sh
grep -c 'id="answer-source"' apps/web/dist/index.html   # 1
head -30 apps/web/dist/llms.txt
```

### `/llms.txt`

Markdown, um `#`, um resumo em citação e seções —
[llmstxt.org](https://llmstxt.org). Serve para o agente ou o motor que quer
saber o que é o site sem interpretar a página. Inclui a tabela de preços, os
critérios, a FAQ, as páginas públicas e duas notas: que os links de portal são
credenciais e não devem ser seguidos nem citados, e que aquele texto é a
descrição oficial do produto. O nginx o serve com `charset utf-8` junto do
`robots.txt` e do `sitemap.xml` (`infra/web-nginx.conf.template`).

`ANSWERS_UPDATED` é a data impressa no arquivo, e é uma constante, não a data
da build: o mesmo commit precisa gerar o mesmo arquivo.

### O que o produto **não** faz

A seção mais útil para um motor de resposta é a que ele não encontraria em
outro lugar. `BOUNDARIES`, em `answers.ts`, responde quatro perguntas antes que
sejam inventadas: a análise não substitui conferência humana e os limiares não
foram calibrados; não há verificação de malware; o DocPing não assina nem
certifica documento; lembrete automático é por e-mail, o WhatsApp é manual.
São afirmações verdadeiras sobre o que está em produção — ver
[06 — Validação](06-validacao-da-entrega.md) antes de suavizar qualquer uma.

### Crawlers de IA no `robots.txt`

O grupo `User-agent: *` continua desautorizando o espaço de trabalho, o portal
e os formulários. Um segundo grupo nomeia os crawlers dos motores de resposta e
**repete** a mesma lista de `Disallow`, porque um grupo nomeado não lê o grupo
anterior — esse é o erro clássico de quem adiciona `GPTBot` ao arquivo e
descobre que acabou de liberar o portal do cliente para ele.

Estão nomeados: `GPTBot`, `OAI-SearchBot`, `ChatGPT-User`, `ClaudeBot`,
`Claude-User`, `Claude-SearchBot`, `anthropic-ai`, `PerplexityBot`,
`Perplexity-User`, `DuckAssistBot`, `Amazonbot`, `MistralAI-User`, `cohere-ai`,
`Meta-ExternalAgent`, `Meta-ExternalFetcher`, `YouBot`, além de
`Google-Extended` e `Applebot-Extended` — esses dois não controlam rastreio
nenhum, controlam se o conteúdo pode ser usado nas respostas de IA e nos
modelos daquelas empresas. O padrão dos dois já é "liberado"; estão escritos
para que seja escolha e não esquecimento.

Nenhum crawler é bloqueado por completo. Ser citado numa resposta é como um
escritório que nunca digitou "DocPing" descobre que ele existe. Se um dia o
tráfego de um deles custar banda sem devolver visita — `Bytespider` é o caso
mais citado —, o bloqueio é um grupo novo com `Disallow: /`, e não a remoção
de um nome da lista acima.

### Trecho sem limite

O `max-snippet:-1` no `robots` da landing e das rotas indexáveis levanta o teto
de quanto da página um motor pode citar na resposta. Não há nada numa página
pública nossa que seja melhor parafraseado do que citado.

## Limitação conhecida

A landing continua sendo renderizada no cliente. O que mudou é que o HTML
servido já carrega os fatos: a fonte de respostas pré-renderizada, o JSON-LD e
o `/llms.txt` não dependem do bundle. O que ainda depende é a página completa —
layout, imagem do produto, demonstração. Para o Googlebot isso é indiferente
(ele executa, só indexa com atraso); para um motor de resposta, o que importa
já está no HTML.

Pré-renderizar a landing inteira (SSR na build) segue sendo o passo seguinte se
a busca orgânica virar canal de aquisição de verdade. Hoje ele custaria tornar
o `App.tsx` seguro para renderizar em Node, e o ganho sobre o que já existe
seria de layout, não de conteúdo.

A imagem `product-preview.png` (362 KB) é o maior ativo da landing e o provável
LCP. Converter para WebP com `<picture>` é a otimização mais barata disponível
se o Core Web Vitals pesar.

## Pendências

Nada aqui foi verificado em produção. Em ordem:

1. Apontar `docping.com.br` para o serviço `web` e ajustar `WEB_ORIGIN`.
2. Registrar a propriedade no Google Search Console e enviar o `sitemap.xml`.
3. Conferir `/robots.txt`, `/sitemap.xml` e o `X-Robots-Tag` de `/p/<token>`
   no domínio real.
4. Validar os dados estruturados no Rich Results Test (`SoftwareApplication`,
   `FAQPage`, `HowTo`, `Organization`, `WebSite`).
5. Conferir a prévia social (OG/Twitter) de `https://docping.com.br/`.
6. Conferir `https://docping.com.br/llms.txt` no domínio real: `Content-Type`
   `text/plain; charset=utf-8` e os acentos íntegros.
7. Conferir que o HTML servido traz a fonte de respostas, buscando pelo que um
   motor buscaria: `curl -s https://docping.com.br/ | grep answer-source`.
8. Perguntar a um motor de resposta — ChatGPT, Perplexity, Gemini — "o que é o
   DocPing?" e comparar com o `/llms.txt`. Nada disso foi feito; a citação em
   motor de resposta não tem prazo nem garantia, e nenhum deles confirma
   indexação como o Search Console confirma.

## Páginas legais, medição e Google Ads

O Google Ads recusa anunciante sem política de privacidade, sem termos e sem
contato acessível na própria landing. As três coisas passam a existir:

- `/privacidade` e `/termos` (`apps/web/src/pages/Legal.tsx`), indexáveis, no
  `sitemap.xml`, linkadas no rodapé da landing e uma na outra.
- Rodapé com e-mail de contato, e-mail de suporte/LGPD e, quando
  `VITE_MEETING_URL` está definida, link de agenda. Sem a variável o rodapé
  oferece apenas e-mail — nunca um botão morto.
- Identificação do controlador (razão social, CNPJ, endereço) vinda de
  `VITE_COMPANY_*`. Enquanto estiverem vazias o bloco é omitido em vez de
  imprimir texto genérico. **Preencher antes de subir campanha:** sem CNPJ o
  anunciante não é identificável, e a verificação do Google pede isso.

### Medição

`apps/web/src/lib/consent.ts` é a única porta. Nada do Google é carregado antes
do aceite: nem o `gtag.js`, nem cookie, nem requisição. Isso é mais rígido que
o Consent Mode sozinho, onde a tag carrega negada e ainda assim fala com o
Google, e é a única versão que a política de privacidade consegue descrever
honestamente. Recusar é um clique do mesmo tamanho que aceitar, e o site
funciona igual nos dois casos.

Sem `VITE_GA4_ID` e sem `VITE_GOOGLE_ADS_ID` não há tag e o aviso de cookies
não aparece — não existe com o que consentir. As variáveis são `ARG` no
`apps/web/Dockerfile` porque o Vite as grava no bundle: trocar uma exige
rebuild da imagem.

A conversão de cadastro dispara em `AuthPages` quando o registro é aceito
(inclusive no caminho `verification_sent`, que é o normal em produção): evento
`sign_up` no GA4 e, se `VITE_ADS_SIGNUP_LABEL` existir, `conversion` no Ads.
Navegação de SPA reporta `page_view` por rota — sem isso só a landing seria
contada.

O portal do cliente não recebe nenhuma tag de medição ou publicidade.

### Preço na vitrine

O título dos planos diz o preço do plano mais barato realmente à venda
(R$ 67). Anúncio e landing precisam dizer a mesma coisa, e o Google compara.
O selo do plano em destaque diz "Nossa recomendação": "Mais escolhido" é uma
afirmação sobre a escolha de outras pessoas, e não temos esse dado.

### Pendências de Ads

1. Criar a propriedade GA4 e a conta do Google Ads; preencher as `VITE_*`.
2. Definir a ação de conversão no Ads e colar o rótulo em
   `VITE_ADS_SIGNUP_LABEL`.
3. Preencher `VITE_COMPANY_LEGAL_NAME`, `VITE_COMPANY_TAX_ID` e
   `VITE_COMPANY_ADDRESS`.
4. Criar as caixas `contato@` e `suporte@` — as duas estão publicadas nas
   páginas legais e precisam responder.
5. Revisão jurídica das duas páginas antes da campanha. O texto foi escrito
   contra o produto que existe, não por advogado.
