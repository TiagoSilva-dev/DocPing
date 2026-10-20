# 07 — Deploy em produção

Topologia escolhida: **tudo na Railway**, com os arquivos no **Cloudflare R2**.
O navegador fala com um único domínio, o que mantém o cookie de sessão
`SameSite=Lax` funcionando e deixa o upload de 20 MB passar sem intermediário.

```
navegador
   │
   └── https://docping.com.br ──→ Railway / serviço "web" (nginx)
                                     ├── /        SPA estática
                                     └── /api/*   proxy → serviço "api" (Go)
                                                        ├── MySQL
                                                        ├── ClamAV
                                                        ├── Docling
                                                        └── Cloudflare R2 (externo)
```

Não há serviço de front separado: `apps/web/Dockerfile` compila a SPA e a serve
pelo nginx, que também faz o proxy de `/api/`. Por isso `credentials:
"same-origin"` em `lib/api.ts` continua válido e nada no código muda.

## Serviços na Railway

| Serviço | Origem | Observações |
|---|---|---|
| `web` | `apps/web/Dockerfile` | Público. Recebe o domínio. Precisa de `API_UPSTREAM` |
| `api` | `apps/api/Dockerfile` | **Sem domínio público** — só a rede interna |
| `mysql` | plugin MySQL | Ver a ressalva de TLS abaixo |
| `clamav` | `clamav/clamav:1.4` | ~2 GB de RAM; obrigatório em produção |
| `docling` | `quay.io/docling-project/docling-serve-cpu:v1.37.0` | Opcional; só com IA ligada |

O `api` **não** deve receber domínio público. Todo o tráfego entra pelo `web`,
que é quem faz a checagem de origem valer alguma coisa.

### Imagem do Docling

Use a variante **cpu**. A imagem padrão (`docling-serve`) carrega a pilha CUDA
do PyTorch e pesa **15,4 GB**, o que nenhum host gerenciado puxa com conforto.
A `docling-serve-cpu` fica na casa de 2 a 3 GB. O `docker-compose.yml` já aponta
para ela.

O Docling baixa os modelos no primeiro uso. Monte um volume em
`/opt/app-root/src/.cache`, senão cada reinício refaz o download.

## Variáveis de ambiente

### Serviço `web`

```sh
PORT=                 # a Railway injeta
API_UPSTREAM=http://api.railway.internal:8080
```

O nginx resolve o upstream **a cada requisição**, não no boot. Isso é
deliberado: resolvendo no boot, o `web` entraria em crash-loop sempre que
subisse antes do `api`. Ver o comentário em `infra/web-nginx.conf.template`.

### Serviço `api`

```sh
APP_ENV=production
PORT=                 # a Railway injeta
WEB_ORIGIN=https://docping.com.br          # precisa ser HTTPS
TRUSTED_PROXY_CIDRS=fd00::/8               # rede interna da Railway

STORE_DRIVER=mysql
MYSQL_DSN=usuario:senha@tcp(host:porta)/railway?parseTime=true&tls=true

S3_ENDPOINT=https://<conta>.r2.cloudflarestorage.com
S3_BUCKET=docping
S3_ACCESS_KEY=...
S3_SECRET_KEY=...

CLAMAV_ADDRESS=clamav.railway.internal:3310

SMTP_HOST=...
SMTP_PORT=587
SMTP_FROM=nao-responda@docping.com.br
SMTP_USERNAME=...
SMTP_PASSWORD=...
SMTP_REQUIRE_TLS=true

# Opcional — a API recusa subir se ligado e incompleto
AI_REVIEW_ENABLED=false
DOCLING_URL=http://docling.railway.internal:5001
TYPESAFE_API_KEY=...
TYPESAFE_API_URL=https://api.typesafe.ai
```

`TRUSTED_PROXY_CIDRS` não é detalhe: o limite de requisições é por endereço, e
sem ele o `web` inteiro conta como um único cliente — um visitante tranca o
resto. A rede interna da Railway é IPv6 privada (`fd00::/8`).

`ALLOW_UNSCANNED_UPLOADS` não pode existir em produção. `LoadConfig` recusa
subir se encontrá-la.

## A ressalva do MySQL — verifique isto primeiro

`config.go` exige `tls=true` no DSN, e no driver Go isso significa **verificação
completa**: cadeia do certificado e hostname, contra as raízes do sistema. Não
é `skip-verify`.

**Não confirmei que o MySQL da Railway apresenta um certificado que passe nessa
verificação.** A documentação deles não trata de TLS, e a rede interna pode nem
usar TLS. Se não passar, a API simplesmente não sobe, com
`production MySQL DSN requires verified TLS (tls=true)`.

Teste antes de montar o resto:

```sh
mysql -h <host> -P <porta> -u <user> -p --ssl-mode=VERIFY_IDENTITY
```

Se falhar, troque o banco por um gerenciado com certificado de CA pública —
PlanetScale ou Aiven resolvem — e aponte o `MYSQL_DSN` para lá. **Não** relaxe
o gate em `config.go`: ele existe para impedir que documento de cliente
trafegue em claro.

## Ordem de criação

1. Bucket no R2, privado, com chave de acesso.
2. Provedor de SMTP com TLS e remetente verificado.
3. Projeto na Railway, a partir do repositório do GitHub.
4. Plugin MySQL — **teste o TLS antes de seguir**.
5. Serviço `clamav`, com memória suficiente para a base de vírus.
6. Serviço `api`, com as variáveis acima. Sem domínio público.
7. Serviço `web`, com `API_UPSTREAM` apontando para o `api`.
8. Domínio no `web`, e só então `WEB_ORIGIN` com esse domínio.
9. Migrações: são embutidas e aplicadas sozinhas no start da API.

Com IA, acrescente o `docling` e só então ligue `AI_REVIEW_ENABLED=true`.
Deixe os escritórios em `suggest` — os limiares seguem sem calibração, como
registra `docs/06-validacao-da-entrega.md`.

## O que ainda não foi validado

Nada deste documento foi executado contra a Railway. A topologia, as imagens e
o comportamento do nginx foram verificados localmente com Docker; a criação dos
serviços, o TLS do MySQL gerenciado e o consumo de memória do ClamAV em
produção continuam sem teste.
