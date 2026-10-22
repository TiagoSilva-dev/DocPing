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

**Enquanto o DNS de `docping.com.br` não apontar para o serviço `web`**, o
canonical aponta para um endereço que não responde e o host
`web-production-18459.up.railway.app` não será indexado. Isso é o efeito
desejado — um host `*.up.railway.app` indexado viraria conteúdo duplicado no dia
da troca —, mas é uma pendência, não um estado final. Ao apontar o domínio,
atualize também `WEB_ORIGIN` (ver [07 — Deploy](07-deploy.md)).

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
