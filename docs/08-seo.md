# 08 — SEO e presença pública

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
| `apps/web/src/lib/seo.ts` | `SITE_URL`, base do canonical por rota e do JSON-LD da landing |
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

Indexável: `/`, e só.

Fora do índice, por três caminhos independentes, porque um só não basta quando
o que está em jogo é um link de portal:

1. `<meta name="robots" content="noindex, nofollow">` aplicado por rota em
   `src/lib/seo.ts`. Toda página chama `usePageMeta`, e `index` é opt-in: uma
   página nova nasce privada.
2. `robots.txt`, que desautoriza `/app`, `/p/`, `/portal/`, `/entrar`,
   `/cadastro`, `/esqueci-senha`, `/redefinir-senha` e `/api/`.
3. `X-Robots-Tag: noindex, nofollow` no nginx
   (`infra/web-nginx.conf.template`) para esses mesmos prefixos. É o único que
   vale para um crawler que ignora o `robots.txt` e não executa o JavaScript da
   página — e todas as rotas são servidas pelo mesmo `index.html`, então sem
   esse cabeçalho o HTML de um link de portal é idêntico ao da landing.

Os formulários de acesso ficam fora do índice por serem conteúdo raso, não por
sigilo. Se um dia `/cadastro` virar página de campanha, ela precisa de conteúdo
próprio antes de ser liberada.

## Dados estruturados

`Organization` e `WebSite` são estáticos no `index.html`. `SoftwareApplication`
(com os preços) e `FAQPage` são emitidos pela própria landing, em
`useJsonLd`, a partir dos arrays `plans` e `questions` que ela renderiza — a
marcação e a página não têm como divergir. `apps/web/src/lib/seo.test.ts`
cobre os dois construtores.

Não há `aggregateRating` nem `review`: o produto não tem avaliações, e marcação
inventada é penalizada além de ser mentira. Os preços marcados são exatamente os
que a seção de planos imprime — e continuam sendo uma proposta comercial, não um
limite aplicado pela API (ver o comentário em `Landing.tsx`).

## Limitação conhecida

A landing é renderizada no cliente. O Googlebot executa JavaScript e enxerga a
página, mas com atraso de indexação, e qualquer crawler que não execute só
recebe o `<head>`. Hoje isso é aceitável porque a página é uma só e as tags
sociais são estáticas. Se a busca orgânica virar canal de aquisição, o próximo
passo é pré-renderizar `/` na build.

A imagem `product-preview.png` (362 KB) é o maior ativo da landing e o provável
LCP. Converter para WebP com `<picture>` é a otimização mais barata disponível
se o Core Web Vitals pesar.

## Pendências

Nada aqui foi verificado em produção. Em ordem:

1. Apontar `docping.com.br` para o serviço `web` e ajustar `WEB_ORIGIN`.
2. Registrar a propriedade no Google Search Console e enviar o `sitemap.xml`.
3. Conferir `/robots.txt`, `/sitemap.xml` e o `X-Robots-Tag` de `/p/<token>`
   no domínio real.
4. Validar os dados estruturados no Rich Results Test.
5. Conferir a prévia social (OG/Twitter) de `https://docping.com.br/`.

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
