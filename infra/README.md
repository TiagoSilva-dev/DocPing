# Infraestrutura local

O Compose executa Go, MySQL 8, storage S3 local, caixa SMTP de teste e scanner. O frontend roda com Vite no host. Este ambiente é de desenvolvimento; todos os serviços publicados estão ligados a `127.0.0.1`.

## Subir com Docker

1. Se ainda não existe `.env`, copie `.env.example` para ele na raiz (`cp -n .env.example .env`) e troque os valores `change-me`. Preserve o `.env` já configurado desta sessão; mudar o arquivo não altera senhas de um volume MySQL existente. Os valores do exemplo não são credenciais de produção.
2. Valide a configuração: `docker compose --env-file .env config --quiet`.
3. Execute `docker compose --env-file .env up --build -d`.
4. Inicie o frontend seguindo o README da raiz.
5. Abra o produto em `http://localhost:5173`; API em `http://localhost:8080` por padrão; Mailpit em `http://localhost:8025`; console do storage em `http://localhost:9001`.

Se a porta 8080 já estiver ocupada, configure `API_HOST_PORT=18080` no `.env`. Isso altera somente a publicação no host; Nginx e outros containers continuam usando `api:8080`. A integração desta entrega usou `http://localhost:18080` para evitar conflito com outro processo.

Para executar também o frontend estático em container, use `docker compose --env-file .env --profile web up --build -d` e abra `http://localhost:5173`. Esse perfil compila o React e usa Nginx para o fallback de rotas e proxy `/api`; não inicie o Vite ao mesmo tempo na porta 5173. O proxy usa HTTP/1.1, transmite upload/download sem buffering de disco, desabilita cache da API e preserva cookies de sessão/portal. A landing e rotas do cliente têm `no-store` e política de referência que não expõe tokens. Não inclui TLS de produção. Comportamento de streaming configurado conforme a [documentação oficial do proxy Nginx](https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_request_buffering).

API usa nomes internos `mysql`, `minio`, `mailpit`, `clamav`. `WEB_ORIGIN` deve coincidir com a origem em que o app é aberto. Em desenvolvimento os apelidos de loopback equivalentes (`localhost`, `127.0.0.1`, `[::1]`) e a porta da própria API são aceitos automaticamente, de modo que abrir `http://127.0.0.1:5173` não devolve mais `origin_rejected`. Para qualquer outro endereço — o IP da máquina na rede local, usado para testar no celular, ou a porta publicada da API quando `API_HOST_PORT` difere de 8080 — liste a origem em `WEB_ORIGINS` (separadas por vírgula). Em produção a lista é exata e exige HTTPS em todas as entradas.

`TRUSTED_PROXY_CIDRS` define quais faixas de proxy podem informar o endereço do cliente por `X-Forwarded-For`. O perfil `web` já define as faixas privadas do Compose, porque o Nginx encaminha todo o tráfego e, sem isso, todos os visitantes dividiriam um único limite de requisições por IP. Credenciais do MinIO ficam apenas no servidor. O backend aplica as migrações e inicializa o bucket apenas em desenvolvimento. Em produção, o bucket privado deve existir e estar configurado antes de iniciar a API.

Primeiro build do MinIO compila a fonte fixada e pode levar minutos. ClamAV pode atualizar assinaturas na primeira execução e ainda não aceitar varreduras; a API inicia após o healthcheck `clamdcheck.sh` confirmar o scanner, com seis minutos de tolerância inicial. Uma varredura posterior sem resultado válido continua falhando fechada. A documentação oficial recomenda 3–4 GiB de RAM para o scanner. [ClamAV Docker](https://docs.clamav.net/manual/Installing/Docker.html).

Mailpit captura mensagens localmente: aceitação nessa caixa não prova entrega para um e-mail real. Não configurar encaminhamento externo para testes. A porta do scanner não é publicada, para evitar um daemon de leitura/varredura exposto no host.

Volumes nomeados preservam banco, objetos, e-mails e assinaturas entre reinícios. `docker compose down` conserva volumes; não usar `down -v` sem aceitar a perda dos dados locais. Arquivos sintéticos devem ficar separados dos ambientes comerciais.

Limites locais configuráveis: `CLIENT_LIMIT=30`, `MAX_UPLOAD_BYTES=20971520` (20 MiB), `MAX_ORG_STORAGE_BYTES=1073741824` (1 GiB) e `MAX_REQUEST_STORAGE_BYTES=524288000` (500 MiB). A quota inclui versões antigas/substituídas, pois elas continuam ocupando armazenamento. São limites técnicos iniciais; a UI deve obter permissões reais do servidor e não fixar o plano nos componentes.

## Perfil `ai`: extração de texto

A análise automática usa dois containers, ambos no perfil `ai` porque baixam modelos de centenas de MiB e não devem subir no dia a dia:

```sh
docker compose --env-file .env --profile ai up --build -d
```

- **docling** usa a imagem oficial `quay.io/docling-project/docling-serve`, fixada em `v1.37.0` (índice multi-arquitetura, roda nativo em Apple Silicon), que expõe a mesma pipeline da biblioteca por HTTP em `:5001`. A API chama `POST /v1/convert/file` pedindo Markdown com OCR ligado. Conversão de PDF longo leva dezenas de segundos na primeira execução, enquanto os modelos são baixados para o volume `docling_models`. O Docling já traz OCR próprio, então ele sozinho atende o fluxo. [Docling](https://docling.ai/).
- **paddleocr** é construído a partir de `infra/paddleocr.Dockerfile`: o projeto não publica imagem de serviço pronta, e o `paddlex --serve --pipeline OCR` é a forma documentada de expor a pipeline em HTTP (`POST /ocr`, porta 8080). Serve de fallback quando o Docling recupera menos de 240 caracteres, que é o caso da foto de documento sem camada de texto. [PaddleOCR](https://github.com/PaddlePaddle/PaddleOCR).

As duas portas são publicadas **apenas em `127.0.0.1`** (`5001` para o Docling, `8081` para o PaddleOCR, porque a API em `npm run dev` ocupa a 8080). Isso existe para o modo sem Docker, em que a API Go roda no host e precisa alcançá-las; dentro da rede do Compose elas continuam sendo `docling:5001` e `paddleocr:8080`. Em produção, não publicar nenhuma das duas. O julgamento usa a API da TypeSafe (`TYPESAFE_API_KEY`), que é externa e exige HTTPS — é o único serviço desta arquitetura que recebe conteúdo extraído dos documentos, e isso precisa constar do contrato com o escritório antes de documentos reais. Com `AI_REVIEW_ENABLED=true`, a API recusa iniciar sem chave e sem ao menos um extrator.

**Estado da homologação.** A API da TypeSafe foi chamada com chave real: `POST /v1/systemone` respondeu `200` com `jev-1.13.0` e o formato esperado pelo cliente Go. A imagem do Docling foi baixada e iniciada localmente. O container do PaddleOCR **não foi construído**: as versões fixadas no Dockerfile não foram verificadas contra o índice do PyPI e o build é longo. Acurácia, latência e custo por documento continuam sem medição, e os limiares seguem sem calibração.

## Por que compilar MinIO

A release oficial Community `RELEASE.2025-10-15T17-29-55Z` corrige um problema de segurança e orienta compilar containers da fonte. O repositório encontra-se arquivado em abril de 2026. `infra/minio.Dockerfile` fixa essa release em vez de depender de uma imagem antiga sem essa correção. Isso é um recurso **local**, não garantia de manutenção futura. Para piloto, escolher storage compatível com S3 mantido, privado e criptografado. [Release oficial, consulta em 19/10/2026](https://github.com/minio/minio/releases/tag/RELEASE.2025-10-15T17-29-55Z).

MySQL `8.0` acompanha correções de sua linha. ClamAV usa a imagem **oficial Debian `1.4.6-debian`**, com índice multi-arquitetura fixado em `sha256:c7df83a4c9794aeb43b7cff296fc9f42a0f47cbf0e96704510e7e5b57b2f4913`, verificado diretamente no registry; inclui Linux AMD64, ARM64 e PPC64LE. A variante Alpine `1.4_base` não ofereceu ARM64 no teste e foi substituída, permitindo execução nativa em Apple Silicon. Atualizar tag+digest exige nova homologação de scanner e assinaturas. [Fontes oficiais das imagens ClamAV](https://github.com/Cisco-Talos/clamav-docker).

Mailpit está fixado em `v1.31.6`, conferido na [release oficial](https://github.com/axllent/mailpit/releases/tag/v1.31.6). Build, subida e integração do conjunto local foram executados nesta entrega, conforme o registro abaixo; homologação de produção continua separada.

## Desenvolvimento sem Docker

O modo alternativo é uma escolha explícita: API Go com `APP_ENV=development`, `STORE_DRIVER=json`, diretório de dados local privado e arquivos sintéticos. Não é um fallback silencioso quando MySQL falha.

No diretório `apps/api`, após instalar Go e dependências:

```sh
APP_ENV=development STORE_DRIVER=json DATA_DIR=../../.local-data \
  WEB_ORIGIN=http://localhost:5173 ALLOW_UNSCANNED_UPLOADS=true \
  go run ./cmd/server
```

`ALLOW_UNSCANNED_UPLOADS=true` permite somente experimentar o fluxo com arquivos sintéticos e marca a ausência de varredura. Deixar SMTP/S3/ClamAV sem configuração não significa que estejam funcionando. O envio de e-mail deve falhar explicitamente sem servidor; nunca indicar aceitação sem SMTP.

JSON/local não valida SQL, transações, concorrência, bucket S3, scanner ou e-mail. A integração local com MySQL+storage+scanner+SMTP foi executada separadamente. Antes de piloto real, repetir a homologação no ambiente escolhido, testar restauração e fechar os gates de `docs/05-seguranca-e-piloto.md`.

## Configuração de produção

### SMTP de produção (Hostinger)

O transporte alcança TLS de dois modos, escolhidos por `SMTP_TLS_MODE`: `starttls` eleva uma conexão em texto claro (porta 587, a maioria dos provedores) e `implicit` fala TLS desde o primeiro byte (porta 465, a recomendada pela Hostinger). Em branco, a porta decide — 465 é `implicit`. Ler o greeting de uma porta de TLS implícito por socket comum nunca funciona, e é por isso que o modo é configuração e não descoberta. Nos dois modos o certificado é verificado contra as raízes do sistema e o hostname configurado; não existe opção de ignorar a verificação.

Para a Hostinger: `SMTP_HOST=smtp.hostinger.com`, `SMTP_PORT=465`, `SMTP_TLS_MODE=implicit`, `SMTP_USERNAME` com o **endereço completo** da caixa, `SMTP_PASSWORD` com a senha **da caixa** (não a do painel) e `SMTP_FROM` igual à caixa autenticada ou a um alias dela — um remetente diferente é recusado pelo provedor, e o erro aparece como `remetente recusado pelo provedor` na entrega. Verificado em 21/10/2026 contra o servidor real, sem credenciais: as portas 465 e 587 respondem em TLS 1.3 com certificado válido (`CN=hostinger.com`, verificação OK) e anunciam `AUTH PLAIN LOGIN`, que é o mecanismo usado pelo cliente. O limite anunciado é de 48,8 MB por mensagem; as mensagens do DocPing são texto com um link, nunca anexos.

`SMTP_FROM` é independente de `SMTP_USERNAME`: autentique com o endereço da caixa e use o alias como remetente — por exemplo `SMTP_USERNAME=tiago@docping.com.br` com `SMTP_FROM=notificacao@docping.com.br`. É assim que as notificações saem de um endereço de produto, e não de um endereço pessoal.

Para enviar **uma** mensagem real e conferir o caminho completo, existe um teste opt-in que lê a senha de um arquivo, para o segredo não passar por linha de comando:

```sh
printf '%s' '<senha da caixa>' > ~/.docping-smtp-pass && chmod 600 ~/.docping-smtp-pass
cd apps/api && SMTP_LIVE_TO=<seu endereço> SMTP_LIVE_USERNAME=<caixa> \
  SMTP_LIVE_FROM=<alias> SMTP_LIVE_PASSWORD_FILE=~/.docping-smtp-pass \
  go test ./internal/app -run HostingerSMTPLive -v
```

### Por que não a Mail API da Hostinger

A Hostinger também oferece uma API HTTP (`api.mail.hostinger.com`, do Agentic Mail). Ela **não** é usada, por três motivos verificados em 21/10/2026 contra a API real: o token é escopado a uma caixa específica e o envio é `POST /api/v1/mailboxes/{resourceId}/send`, sem campo de remetente no schema — então um **alias não pode ser o remetente**, porque alias não é caixa e não tem `resourceId`; cada mensagem enviada é copiada para `INBOX.Sent` da caixa, o que com volume transacional enche uma caixa de pessoa; e o corpo documentado é apenas texto. O `resourceId` de uma caixa, se precisar dele algum dia, vem de `GET /api/v1/me` com o token.

Não foi enviada nenhuma mensagem real na verificação de handshake: credenciais, aceitação do alias como remetente, entrega na caixa de entrada e reputação (SPF, DKIM, DMARC do domínio) dependem do teste opt-in acima.

Separar secrets do repositório, `APP_ENV=production`, `STORE_DRIVER=mysql`, DSN com TLS, origem HTTPS, bucket privado com criptografia, credencial de mínimo privilégio, scanner atualizado e provedor SMTP autenticado com transporte seguro. O modo JSON, storage local e aceite de arquivo não verificado devem ser recusados na inicialização de produção. TLS/CSRF, backups, retenção, alertas e teste de recuperação são requisitos adicionais; este Compose não os fornece.

Não publicar portas de MySQL, console S3, Mailpit ou ClamAV. Um proxy HTTPS pode servir aplicação e API na mesma origem; sem microserviços nem fila adicional. Monólito e jobs devem continuar idempotentes ao reiniciar.

## Validação desta sessão

Em 19–20/10/2026, Compose validou com e sem o perfil `web`; o daemon Docker foi iniciado e o conjunto foi compilado e subiu saudável. O frontend Nginx respondeu HTTP 200 em `http://localhost:5173` e o proxy `/api/health` também respondeu 200.

`node scripts/integration-smoke.mjs --mysql --eicar` passou **18 verificações** contra API Go, MySQL 8, MinIO, ClamAV e Mailpit reais. Incluiu cadastro/sessão, cliente/modelo/solicitação persistidos, CSRF, isolamento entre escritórios, PDF/PNG passando pelo scanner e storage, rejeição da assinatura inofensiva EICAR embutida em PDF sem persistência, bloqueio de acesso anônimo, e-mail aceito e presente no Mailpit, correção sem descartar outro arquivo, aprovação/conclusão, ZIP, código sensível por navegador, revogação e `TestMySQLRoundTrip` real sem skip. Relatório: `test-results/integration-smoke.json`.

Esse teste usa arquivos sintéticos e SMTP local. Não comprova TLS/SSE do ambiente comercial, entrega externa de e-mail, restauração de backup ou experiência visual/mobile. A checagem de configuração SSE de produção usa mock em três casos e precisa de homologação em storage real com criptografia. Fluxos de navegador permanecem sem validação: a tentativa Playwright falhou ao iniciar Chromium (`EPERM`), antes de abrir telas; a ferramenta de navegação também bloqueou o endereço após autorização. Consulte os resultados finais em `docs/06-validacao-da-entrega.md`.
