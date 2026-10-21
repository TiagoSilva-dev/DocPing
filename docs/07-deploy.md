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
                                                   ├── ClamAV     (Railway)
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
| `clamav` | `clamav/clamav:1.4.6-debian` | **Mais de 1 GiB de RAM.** Ver "O ClamAV" |
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

O Docling baixa os modelos no primeiro uso. Monte um volume em
`/opt/app-root/src/.cache`, senão cada reinício refaz o download.

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
- **A branch não ficou gravada no serviço.** Apontar o serviço para uma branch
  que não é a padrão do repositório não persistiu, e os deploys seguintes saíram
  de `main`. Hoje `api` e `web` estão fixados por commit, o que é determinístico
  mas **não** redeploya a cada push: ao mudar de commit, refixe a origem.

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

CLAMAV_ADDRESS=clamav.railway.internal:3310

SMTP_HOST=...
SMTP_PORT=587
SMTP_FROM=nao-responda@<dominio>
SMTP_USERNAME=...
SMTP_PASSWORD=...
SMTP_REQUIRE_TLS=true

# Opcional — a API recusa subir se ligado e incompleto
AI_REVIEW_ENABLED=false
DOCLING_URL=http://docling.railway.internal:5001
TYPESAFE_API_KEY=...
TYPESAFE_API_URL=https://api.typesafe.ai
```

Prefira referência (`${{Servico.VAR}}`) a copiar segredo: ela sobrevive a
rotação de credencial, e as credenciais de bucket da Railway vêm redigidas para
aplicações OAuth de todo modo.

`TRUSTED_PROXY_CIDRS` não é detalhe: o limite de requisições é por endereço, e
sem ele o `web` inteiro conta como um único cliente — um visitante tranca o
resto. A rede interna da Railway é IPv6 privada (`fd00::/8`).

`ALLOW_UNSCANNED_UPLOADS` não pode existir em produção. `LoadConfig` recusa
subir se encontrá-la.

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

## O ClamAV

Duas coisas, ambas descobertas na prática:

- **Memória.** O clamd carrega a base de assinaturas inteira: medi 976 MiB
  localmente e 1,33 GB na Railway. A Railway aplicou 1 GB por padrão ao serviço
  e o processo morria antes de abrir o socket, sem deixar log próprio — só o
  laço `Socket for clamd not found yet, retrying`. Está em 4 GiB / 2 vCPU.
- **Não monte volume em `/var/lib/clamav`.** Volume novo monta vazio e esconde a
  base que a imagem já traz; o clamd não sobe. A base é reconstruída pelo
  freshclam a cada início, o que custa um download de ~110 MB e é o preço aceito
  aqui. Se quiser cache persistente, monte em outro caminho e aponte o
  `DatabaseDirectory`.

Quando o scanner não responde, o upload falha **fechado**, com
`503 scan_failed` — é o comportamento desejado, não um defeito.

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

## Ordem de criação

1. Bucket privado. Confirmando SSE-S3 (AWS S3, Backblaze B2), nada mais é
   preciso; não confirmando (bucket da Railway, R2), é necessário ligar
   `S3_ACCEPT_UNCONFIRMED_ENCRYPTION` — leia antes o que isso custa.
2. Provedor de SMTP com TLS e remetente verificado.
3. Projeto na Railway.
4. Serviço `MySQL` a partir de `infra/mysql.Dockerfile`, com volume em
   `/var/lib/mysql`. Leia o certificado da CA no log do primeiro boot.
5. Serviço `clamav`, com mais de 1 GiB de RAM e **sem** volume em
   `/var/lib/clamav`.
6. Serviço `api`, diretório raiz `/apps/api`, healthcheck `/api/health`, com as
   variáveis acima e o `MYSQL_CA_CERT` do passo 4. Sem domínio público.
7. Serviço `web`, diretório raiz `/`, Dockerfile `apps/web/Dockerfile`, com
   `API_UPSTREAM` apontando para o `api`.
8. Domínio no `web`, e só então `WEB_ORIGIN` com esse domínio.
9. Migrações: são embutidas e aplicadas sozinhas no start da API.

Com IA, acrescente o `docling` e só então ligue `AI_REVIEW_ENABLED=true`.
Deixe os escritórios em `suggest` — os limiares seguem sem calibração, como
registra `docs/06-validacao-da-entrega.md`.

## O que foi validado em produção, e o que não

Executado contra a Railway e observado funcionando:

- `web` servindo a SPA em HTTPS e fazendo proxy de `/api/`.
- `api` subindo com `APP_ENV=production` — o que significa que todo o
  `LoadConfig` passou — e respondendo `/api/health`.
- MySQL com **verificação completa de TLS**, cadeia e hostname.
- ClamAV varrendo: o upload de teste passou pela varredura.
- Fluxo de escritório: registro, criação de cliente, modelos semeados,
  criação de solicitação e geração de link do portal, todos `200`.

Sem validação:

- **E-mail não sai** — `SMTP_HOST` é placeholder, por decisão de quem operou o
  deploy. As entregas ficam registradas como falha.
- Análise automática desligada; `docling` não foi criado.
- Domínio próprio, Playwright e validação visual seguem pendentes, como
  registra `docs/06-validacao-da-entrega.md`.

O banco de produção contém organizações de teste criadas pelo smoke manual,
com e-mails `smoke-*@docping-smoke.invalid`.
