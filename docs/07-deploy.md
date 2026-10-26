# 07 — Deploy em produção

Topologia: **tudo na Railway**, arquivos incluídos. O navegador fala com um
único domínio, o que mantém o cookie de sessão `SameSite=Lax` funcionando e
deixa o upload de 20 MB passar sem intermediário.

```
navegador
   │
   └── https://<dominio> ──→ Railway / serviço "web" (nginx)
                                ├── /        SPA estática
                                └── /api/*   proxy → serviço "api" (Go)
                                                   ├── MySQL      (Railway)
                                                   ├── Docling    (Railway, opcional)
                                                   └── Bucket S3  (Railway — ver "Os arquivos")
```

Não há serviço de front separado: `apps/web/Dockerfile` compila a SPA e a serve
pelo nginx, que também faz o proxy de `/api/`. Por isso `credentials:
"same-origin"` em `lib/api.ts` continua válido e nada no código muda.

O ambiente que existe hoje é o projeto `docping`
(`f45084e3-77db-4974-9a56-e5e4ec761b6f`), ambiente `production`, servido em
`web-production-18459.up.railway.app`.

## Serviços na Railway

| Serviço | Origem | Observações |
|---|---|---|
| `web` | repo, `apps/web/Dockerfile` | Público. Diretório raiz `/`. Precisa de `API_UPSTREAM` |
| `api` | repo, `apps/api/Dockerfile` | **Sem domínio público**. Diretório raiz `/apps/api` |
| `MySQL` | repo, `infra/mysql.Dockerfile` | Imagem própria. Ver "O TLS do MySQL" |
| `docling` | `quay.io/docling-project/docling-serve-cpu:v1.37.0` | Opcional; só com IA ligada |

O `api` **não** deve receber domínio público. Todo o tráfego entra pelo `web`,
que é quem faz a checagem de origem valer alguma coisa.

Os dois Dockerfiles do app esperam contextos de build diferentes — o da API faz
`COPY go.mod go.sum ./` e precisa de `apps/api`, o da web precisa da raiz. Isso
se resolve com o diretório raiz de cada serviço, não com um arquivo único.

### Imagem do Docling

Use a variante **cpu**. A imagem padrão (`docling-serve`) carrega a pilha CUDA
do PyTorch e pesa **15,4 GB**, o que nenhum host gerenciado puxa com conforto.
A `docling-serve-cpu` fica na casa de 2 a 3 GB. O `docker-compose.yml` já aponta
para ela.

**Não monte volume no cache de modelos.** Os pesos vêm embutidos na imagem —
o serviço registra no start `artifacts_path is set to a valid directory. No
model weights will be downloaded at runtime`. Um volume vazio montado em
`/opt/app-root/src/.cache` esconderia esses pesos. Sem volume o serviço sobe em
cerca de 10 segundos.

Medido em produção: 2,18 GB comprimidos em `linux/amd64` (14 camadas) e
**1,27 GB de RAM ocioso**, com CPU perto de zero fora de uma conversão.

## O que a Railway não faz como se espera

Três comportamentos custaram tempo e vale registrar:

- **Config as code está depreciada.** `railway.json` e `railway.toml` não podem
  mais ser habilitados em serviço novo, então um arquivo desses no repositório
  nunca é lido. Foi por isso que ele saiu daqui. A configuração de build de cada
  serviço (diretório raiz, Dockerfile, healthcheck, política de restart) vive na
  própria Railway, via API ou painel. A sucessora é a IaC em
  `.railway/railway.ts`, que só a CLI avalia — ela não é lida no deploy.
- **O start command roda como argv, sem shell.** O runtime V2 não interpreta
  `&&`, `;` nem redirecionamento, e um `sh -c '...'` ali é desembrulhado: o
  primeiro token acaba tratado como executável. Qualquer bootstrap de várias
  etapas precisa virar entrypoint numa imagem, como em `infra/mysql.Dockerfile`.
- **O uvicorn só escuta IPv4 por padrão.** A rede interna da Railway é IPv6, e
  `UVICORN_HOST` vem como `0.0.0.0`, então `docling.railway.internal:5001`
  ficaria inalcançável sem erro visível na API — só análises falhando. O serviço
  `docling` leva `UVICORN_HOST=::`, e o log confirma com
  `Uvicorn running on http://[::]:5001`.
- **A branch não ficou gravada no serviço.** Apontar o serviço para uma branch
  que não é a padrão do repositório não persistiu, e os deploys seguintes saíram
  de `main`. Hoje `api` e `web` estão fixados por commit, o que é determinístico
  mas **não** redeploya a cada push: ao mudar de commit, refixe a origem.

## O domínio próprio

`docping.com.br` serve o `web` desde 22/10/2026, com certificado Let's Encrypt
emitido pela Railway (`CN=docping.com.br`, válido até 20/01/2027). O DNS
continua na Hostinger; não foi preciso mover nameservers.

A Railway não publica IP fixo, então um domínio só chega nela por `CNAME`. Na
raiz o padrão de DNS proíbe `CNAME` (RFC 1912), e só funciona em provedor que
implemente *flattening*/ALIAS — a documentação da Railway lista a Hostinger
entre os que **não** fazem isso. Na prática ela faz: o editor de zona aceita
`CNAME` em `@` e grava um ALIAS. O que ele recusa é a convivência, com a
mensagem `RRset docping.com.br IN ALIAS must not be used with AAAA on the same
name` — o apex trazia o `A` e o `AAAA` da página estacionada da Hostinger, e o
ALIAS não divide o nome com eles.

A zona final, no apex:

| Tipo | Nome | Valor |
|---|---|---|
| CNAME (gravado como ALIAS) | `@` | `6bl1afkk.up.railway.app` |
| TXT | `_railway-verify` | `railway-verify=<token do painel>` |

Os dois são obrigatórios: sem o TXT a Railway não confirma a posse e responde
404 mesmo com o roteamento correto. Apagar o `A` e o `AAAA` do apex antes de
criar o ALIAS; **não** tocar em `MX`, no TXT do SPF, nos três `CNAME
hostingermail-*._domainkey`, no `_dmarc` nem no `autodiscover`, que são o
e-mail do domínio.

Ordem importa mais do que parece. Cadastrar o domínio na Railway antes de o DNS
existir faz com que ela peça o certificado enquanto o apex ainda responde outro
servidor; cada pedido vira uma *failed authorization* no Let's Encrypt, que
limita a cinco por hostname por hora, e a emissão passa a falhar por limite e
não por configuração. A espera resolve — nenhum certificado chega a ser
emitido, então o limite semanal de duplicatas não entra na conta. Crie os
registros primeiro, ou aceite esperar a janela de uma hora.

`WEB_ORIGIN` passou a ser `https://docping.com.br`, e `WEB_ORIGINS` está vazio
de propósito: o `web-production-*.up.railway.app` foi removido do serviço, e um
endereço que não controlamos mais não pode continuar numa lista de origens
permitidas. Gerar um domínio da Railway de novo exige recolocá-lo ali.

## Variáveis de ambiente

### Serviço `web`

```sh
PORT=8080
API_UPSTREAM=http://api.railway.internal:8080
```

Fixe o `PORT` em vez de confiar no que a Railway injeta: o `API_UPSTREAM` aponta
para uma porta fixa, e um `PORT` diferente do esperado quebra o proxy em
silêncio.

O nginx resolve o upstream **a cada requisição**, não no boot. Isso é
deliberado: resolvendo no boot, o `web` entraria em crash-loop sempre que
subisse antes do `api`. Ver o comentário em `infra/web-nginx.conf.template`.

### Serviço `api`

```sh
APP_ENV=production
PORT=8080
DATA_DIR=/data
WEB_ORIGIN=https://<dominio>                 # precisa ser HTTPS
TRUSTED_PROXY_CIDRS=fd00::/8                 # rede interna da Railway

STORE_DRIVER=mysql
MYSQL_DSN=${{MySQL.MYSQLUSER}}:${{MySQL.MYSQL_ROOT_PASSWORD}}@tcp(${{MySQL.RAILWAY_PRIVATE_DOMAIN}}:3306)/${{MySQL.MYSQL_DATABASE}}?parseTime=true&tls=docping-ca
MYSQL_CA_CERT=-----BEGIN CERTIFICATE-----...  # ver "O TLS do MySQL"

S3_ENDPOINT=${{docping-files.ENDPOINT}}
S3_BUCKET=${{docping-files.BUCKET}}
S3_ACCESS_KEY=${{docping-files.ACCESS_KEY_ID}}
S3_SECRET_KEY=${{docping-files.SECRET_ACCESS_KEY}}
S3_ACCEPT_UNCONFIRMED_ENCRYPTION=true         # ver "Os arquivos"

# E-mail. Na Railway abaixo do plano Pro o SMTP de saída é bloqueado e a
# conexão expira sem erro do provedor -- veja "O e-mail não sai por SMTP".
RESEND_API_KEY=re_...
MAIL_FROM=nao-responda@<dominio>            # domínio verificado no provedor

# Alternativa, onde a porta de SMTP não for bloqueada:
# SMTP_HOST=...
# SMTP_PORT=587
# SMTP_FROM=nao-responda@<dominio>
# SMTP_USERNAME=...
# SMTP_PASSWORD=...
# SMTP_REQUIRE_TLS=true

# Assinaturas -- a API recusa subir com a chave e sem o resto
STRIPE_SECRET_KEY=sk_live_...                 # produção recusa sk_test/rk_test
STRIPE_WEBHOOK_SECRET=whsec_...               # do endpoint, não da conta
STRIPE_PRICE_ESSENCIAL=price_...
STRIPE_PRICE_ESCRITORIO=price_...
STRIPE_PRICE_REDE=price_...

# Análise automática — a API recusa subir se ligado e incompleto
AI_REVIEW_ENABLED=true
DOCLING_URL=http://docling.railway.internal:5001
DOCLING_API_KEY=...            # mesmo valor de DOCLING_SERVE_API_KEY no docling
DOCLING_OCR_LANG=pt
TYPESAFE_API_KEY=...
TYPESAFE_API_URL=https://api.typesafe.ai
```

Prefira referência (`${{Servico.VAR}}`) a copiar segredo: ela sobrevive a
rotação de credencial, e as credenciais de bucket da Railway vêm redigidas para
aplicações OAuth de todo modo.

`TRUSTED_PROXY_CIDRS` não é detalhe: o limite de requisições é por endereço, e
sem ele o `web` inteiro conta como um único cliente — um visitante tranca o
resto. A rede interna da Railway é IPv6 privada (`fd00::/8`).

## O TLS do MySQL

A ressalva que este documento levantava se confirmou, e de forma pior do que o
previsto: o MySQL gerenciado da Railway apresenta o certificado autogerado da
imagem oficial, que **não carrega nome nenhum** — sem SAN, e o Go ignora CN
desde a 1.15. O erro é exato:

```
tls: failed to verify certificate: x509: certificate is not valid for any
names, but wanted to match mysql.railway.internal
```

Não existe host que case com esse certificado. Não é ajuste de DSN, não se
resolve confiando na CA, e `tls=skip-verify` entregaria o tráfego do banco sem
autenticação de servidor numa rede que a Railway não criptografa.

A solução mantém a verificação e troca só a âncora de confiança:

1. `infra/mysql.Dockerfile` emite, no primeiro boot, uma CA e um certificado de
   servidor com `subjectAltName=DNS:mysql.railway.internal`, guardados no
   volume para sobreviverem a restart. As chaves privadas são criadas no
   container e nunca saem dele; só o certificado da CA, que é público, é
   impresso no log.
2. Os certificados ficam **ao lado** do data directory, não dentro: o mysqld
   recusa inicializar um data directory que contenha qualquer outra coisa. Por
   isso o datadir desce para `/var/lib/mysql/data` e o volume raiz abriga os
   dois.
3. `MYSQL_CA_CERT` na API recebe esse certificado, e o DSN usa
   `tls=docping-ca`. O `config.go` registra a âncora antes de qualquer conexão e
   deixa o driver preencher o `ServerName` com o host do DSN, de modo que a
   conferência de hostname continua sendo contra o endereço realmente discado.

O gate segue recusando `tls=skip-verify`, DSN sem TLS, e o modo novo sem CA ou
com CA ilegível — ele não tem como decair para conexão não verificada. Há teste
para cada um desses casos em `config_test.go`.

Verificado localmente com volume limpo: o mysqld carrega esses certificados, o
`openssl s_client` valida cadeia e hostname com retorno 0, e um hostname
diferente falha com `hostname mismatch`. Em produção a API sobe e serve.

Note que a Railway resolve `mysql:8` para **8.4**, enquanto o
`docker-compose.yml` local usa 8.0.

## O ClamAV foi removido — 26/10/2026

Havia um serviço `clamav` varrendo todo upload. Ele saiu do produto e da
infraestrutura **por custo**: o clamd carrega a base de assinaturas inteira em
memória — 976 MiB medidos localmente, 1,33 GB na Railway, e com o padrão de 1 GB
o processo morria antes de abrir o socket, sem log próprio, só o laço `Socket
for clamd not found yet, retrying`. Rodava em 4 GiB / 2 vCPU, mais memória que
o resto da pilha somada, para um produto que ainda não tem escritório pagante.

Consequências, em ordem de importância:

- **Nenhum arquivo é varrido.** Todo upload é gravado como `unscanned`. O que
  protege o upload agora é a verificação de tipo por conteúdo *e* extensão (só
  PDF/JPG/PNG/HEIC), o limite de tamanho, o nome saneado, o bucket privado e a
  autorização em cada download. Nada disso detecta malware.
- `CLAMAV_ADDRESS` e `ALLOW_UNSCANNED_UPLOADS` **não existem mais**: deixe as
  duas fora do ambiente. Os códigos `503 scan_failed` e `422 malware_rejected`
  saíram do contrato.
- O risco está registrado como aceito em `docs/05-seguranca-e-piloto.md`, no
  gate "Upload malicioso". Antes de um piloto com documentos reais, ou alguém
  aceita o risco por escrito, ou o scanner volta.

Para reintroduzi-lo: `git revert` do commit que o removeu, serviço com mais de
1 GiB de RAM e **sem** volume em `/var/lib/clamav` (volume novo monta vazio e
esconde a base que a imagem já traz; o freshclam reconstrói a cada início, ~110
MB de download).

## Os arquivos

Ficam no **bucket da própria Railway**, com a confirmação de criptografia
dispensada por decisão explícita de quem opera. Isso precisa de contexto.

`storage.go` exige, em produção, que o provedor confirme `SSE-S3` com
`x-amz-server-side-encryption: AES256` na resposta de cada objeto; se não vier,
o objeto é apagado e o upload falha. A verificação existe para que metadado de
arquivo só fique visível depois que o provedor confirmou a criptografia.

Nenhum dos dois candidatos disponíveis confirma:

- **Bucket da Railway** — aceita o upload e não devolve o header. Testado
  contra o deploy real.
- **Cloudflare R2**, o plano original deste documento — testado com credencial
  real, pelo mesmo cliente minio-go e o mesmo `encrypt.NewSSE()` que a API usa:

  | Passo | Resultado |
  |---|---|
  | `BucketExists` | ok |
  | `PutObject` com SSE-S3 | aceito, sem erro |
  | `StatObject` **com** `versionId` | `501 Not Implemented` |
  | `StatObject` **sem** `versionId` | ok |
  | Header SSE na resposta | ausente — metadata traz só `Content-Type` |

  Confirma a tabela de compatibilidade S3 da Cloudflare, que marca
  `x-amz-server-side-encryption` como não implementado no `PutObject`.

Compatíveis, por documentação do provedor, caso se queira voltar a ter a
confirmação: **AWS S3** (SSE-S3 por padrão desde 2023) e **Backblaze B2**
(aceita `AES256` no `PutObject` e devolve o header no `HeadObject`).

### A dispensa, e o que ela custa

`S3_ACCEPT_UNCONFIRMED_ENCRYPTION=true` mantém o arquivo mesmo sem a
confirmação. O que se perde é preciso: a criptografia em repouso deixa de ser
**fato verificado** e passa a ser **confiança no provedor**. A Railway e a
Cloudflare declaram criptografar tudo em repouso com chaves próprias; o app
deixa de ter como checar isso.

Ela nunca é o padrão — sem a variável, o comportamento antigo vale inteiro,
com teste cobrindo a recusa e a remoção do objeto — e a API registra um aviso
em todo start enquanto estiver ligada.

### Uma fragilidade registrada, não corrigida

O R2 **devolve** um `VersionID` no `PutObject` mas não implementa HEAD
versionado. O `storage.go` guarda esse id e o usa na verificação, levando um
`501` por motivo que nada tem a ver com criptografia. Falha fechado, então não
é brecha; mas qualquer provedor com esse comportamento quebra o upload sem que
a mensagem explique o porquê.

### O domínio público

O bucket do R2 tinha um domínio `pub-*.r2.dev` ativo. Objeto em bucket deste
app nunca deve ser público: o download passa por `GET /api/files/{id}`
justamente para ser autorizado, e as chaves (`orgID/requestID/fileID`) são
aleatórias mas não são controle de acesso.

## O e-mail não sai por SMTP

A Railway bloqueia SMTP de saída em todo plano abaixo de Pro. O sintoma não é
um erro do provedor: a conexão simplesmente expira.

```
smtp implicit TLS dial failed (smtp.hostinger.com:465, mode implicit):
dial tcp 172.65.255.143:465: i/o timeout
```

Verificado em 22/10/2026: as portas 465 e 587 da Hostinger respondem em TLS 1.3
com certificado válido a partir de uma máquina comum, e expiram a partir do
container. A configuração estava correta; ela só não alcançava a rede. A
documentação da Railway diz o mesmo em uma linha: *"SMTP is only available on
the Pro plan and above."*

Por isso o envio tem dois transportes. Com `RESEND_API_KEY` a mensagem sai por
HTTPS, pelo mesmo caminho de qualquer outra chamada de API do binário, e o
bloqueio deixa de existir. O SMTP continua para o Compose local, onde o Mailpit
é justamente o ponto. `MAIL_FROM` é o remetente dos dois; sem ele vale
`SMTP_FROM`, para um deploy anterior continuar igual.

Antes da primeira mensagem, o domínio do remetente precisa estar verificado no
provedor: são registros de SPF e DKIM no DNS de `docping.com.br`, que fica na
Hostinger. Sem isso o provedor recusa com `validation_error`, e o log registra
a frase dele.

**Isto bloqueia o produto inteiro quando falha**, não só o cadastro:
confirmação de e-mail, link do portal para o cliente e lembrete de prazo saem
todos pelo mesmo caminho.

## A Stripe

Três coisas precisam existir na conta antes de a cobrança funcionar, e duas
delas não têm como ser inferidas pelo código:

1. **Um produto por plano**, cada um com seu preço mensal em BRL. Preços de
   planos diferentes no mesmo produto fazem toda linha de fatura e de Checkout
   sair com o mesmo nome, e o cliente não distingue o que comprou.
2. **O endpoint de webhook**, apontando para `https://<domínio>/api/billing/webhook`,
   com os dez eventos que o `applyEvent` trata. O segredo de assinatura
   (`whsec_...`) aparece uma única vez, na criação, e é ele que vai em
   `STRIPE_WEBHOOK_SECRET` — é específico do endpoint, não da conta.
3. **Uma configuração do portal do cliente**, senão `POST /v1/billing_portal/sessions`
   recusa e o botão "área de cobrança" não abre. É onde o assinante troca o
   cartão, muda de plano e cancela; nada disso é reconstruído aqui.

Sem assinatura de webhook verificada, qualquer um que alcance a rota assina de
graça. É por isso que `LoadConfig` recusa subir com chave e sem segredo, em vez
de aceitar e registrar um aviso.

**Imposto.** `automatic_tax` **não** está ligado, e ligar não basta: a Stripe
não calcula nem recolhe nada enquanto não houver registro fiscal ativo na
conta, e não devolve erro dizendo isso. Enquanto não houver essa decisão, o
preço cobrado é o preço anunciado, sem imposto somado no Checkout. A coleta de
CNPJ está ligada, o que cobre a nota do cliente PJ, não a apuração.

## Ordem de criação

1. Bucket privado. Confirmando SSE-S3 (AWS S3, Backblaze B2), nada mais é
   preciso; não confirmando (bucket da Railway, R2), é necessário ligar
   `S3_ACCEPT_UNCONFIRMED_ENCRYPTION` — leia antes o que isso custa.
2. Provedor de SMTP com TLS e remetente verificado.
3. Projeto na Railway.
4. Serviço `MySQL` a partir de `infra/mysql.Dockerfile`, com volume em
   `/var/lib/mysql`. Leia o certificado da CA no log do primeiro boot.
5. Serviço `api`, diretório raiz `/apps/api`, healthcheck `/api/health`, com as
   variáveis acima e o `MYSQL_CA_CERT` do passo 4. Sem domínio público.
6. Serviço `web`, diretório raiz `/`, Dockerfile `apps/web/Dockerfile`, com
   `API_UPSTREAM` apontando para o `api`.
7. Domínio no `web`, e só então `WEB_ORIGIN` com esse domínio. O webhook da
   Stripe aponta para esse mesmo domínio, então ele precisa existir antes.
8. Migrações: são embutidas e aplicadas sozinhas no start da API.

Com IA, acrescente o `docling` e só então ligue `AI_REVIEW_ENABLED=true`.
Deixe os escritórios em `suggest` — os limiares seguem sem calibração, como
registra `docs/06-validacao-da-entrega.md`.

### Um motor basta, mas não qualquer um

O `config.go` exige `TYPESAFE_API_KEY` e **`DOCLING_URL` ou `PADDLEOCR_URL`**,
e o `Chain` aceita primário ou fallback nulo. Só que os dois não são
equivalentes, e **PaddleOCR sozinho é perigoso**: com `Primary` nulo, uma falha
do Paddle faz `Chain.Extract` devolver texto vazio e erro nil, e o
`analyzer.go` lê texto vazio como constatação — veredito `rejected` com
confiança 1. Em modo `auto`, isso rejeitaria sozinho o documento bom de um
cliente porque o OCR estava fora. Com Docling sozinho não acontece: `Fallback`
nulo propaga o erro, o status vira `failed` e o item vai para revisão humana.

O Docling também é o motor que preserva tabela, e faz OCR por conta própria
(`do_ocr: true`), cobrindo PDF escaneado e foto. Este deploy usa **só Docling**.

## O que foi validado em produção, e o que não

Executado contra a Railway e observado funcionando:

- `web` servindo a SPA em HTTPS e fazendo proxy de `/api/`.
- `api` subindo com `APP_ENV=production` — o que significa que todo o
  `LoadConfig` passou — e respondendo `/api/health`.
- MySQL com **verificação completa de TLS**, cadeia e hostname.
- Fluxo de escritório: registro, criação de cliente, modelos semeados,
  criação de solicitação e geração de link do portal, todos `200`.
- Upload completo: objeto gravado e baixado de volta com os bytes idênticos ao
  enviado. *(A varredura `clean` observada nessa rodada era do ClamAV, removido
  em 26/10/2026; hoje o mesmo upload grava `unscanned`.)*
- **Análise automática de ponta a ponta**: um DARF de teste com 862 caracteres
  de texto foi extraído pelo Docling (`extractor: docling`, 882 caracteres),
  julgado pelo Jev (`model: jev-1.13.0`) e composto pelo `Decide` em seis
  sinais tipados, concluindo em menos de 6 segundos. O veredito foi `rejected`
  com confiança 0,98, e estava **correto**: o arquivo não correspondia ao
  documento pedido no item. O item permaneceu `uploaded`, porque o escritório
  está em `suggest` e só `auto` aplica veredito sozinho.

Sem validação:

- **E-mail não sai** — `SMTP_HOST` é placeholder, por decisão de quem operou o
  deploy. As entregas ficam registradas como falha.
- Playwright e validação visual seguem pendentes, como registra
  `docs/06-validacao-da-entrega.md`.

O domínio saiu de pendente: `https://docping.com.br` responde `200` com
certificado válido pela cadeia e pelo hostname, `/api/health` responde `{"status":"ok"}`
pelo apex, `http://` redireciona com `301`, a origem nova passa na checagem
(`401 unauthorized` em `/api/session`, que é a resposta de quem não tem sessão)
e uma origem estranha recebe `403 origin_rejected`.

Com a confirmação de e-mail, `SMTP_HOST` placeholder deixou de ser só uma
entrega que falha: um cadastro novo responde `verification_sent` e nunca
recebe o link, então **não consegue entrar**. Contas já confirmadas seguem
entrando — `User.EmailPending` vazio significa "não pendente". Antes de
qualquer piloto, configure SMTP real.

O banco de produção contém organizações de teste criadas pelo smoke manual,
com e-mails `smoke-*@docping-smoke.invalid`.
