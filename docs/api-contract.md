# Contrato implementado da API DocPing

Base `/api`; JSON `camelCase`; IDs são strings opacas; instantes RFC3339 UTC; competência `AAAA-MM`; datas civis `AAAA-MM-DD` no fuso America/Sao_Paulo. Listagens retornam arrays completos nesta primeira versão. Não existe paginação de servidor ainda. Erros usam `{ "error": { "code": "validation_error", "message": "Mensagem em português." } }`. Validação 422, autenticação 401, acesso de outro escritório 404, CSRF/origem 403, conflito de estado 409, link expirado/revogado 410, excesso de arquivo 413, limite de tentativas 429, scanner/storage indisponível 503. Mutações bem sucedidas retornam 200 (inclusive criação); resposta sem corpo de domínio usa `{ "ok": true }`.

## Sessão e autenticação

Sessão autenticada usa cookie `docping_session`, HttpOnly, SameSite=Lax, Secure em produção, validade de 24h; o servidor guarda somente SHA-256 do token. Mutações autenticadas exigem `X-CSRF-Token` obtido na sessão. API valida `Origin` contra a lista de origens permitidas e rejeita `Sec-Fetch-Site: cross-site` apenas quando o `Origin` não está nessa lista. A lista começa em `WEB_ORIGIN` (origem canônica, usada nos links) e aceita origens extras em `WEB_ORIGINS` (separadas por vírgula). Cada valor é normalizado para `esquema://host[:porta]`; caminho, query ou esquema diferente de http/https são erro de configuração na inicialização. Em desenvolvimento a lista também recebe os apelidos de loopback equivalentes (`localhost`, `127.0.0.1`, `[::1]`) e a porta da própria API, para que abrir o app em `127.0.0.1` não devolva `origin_rejected`; em produção a lista é exata e todas as origens precisam ser HTTPS. Em desenvolvimento a mensagem de 403 nomeia as origens aceitas; em produção permanece genérica.

Rate limiting por IP usa o endereço da conexão. `X-Forwarded-For` só é considerado quando o par direto está em `TRUSTED_PROXY_CIDRS` (lista de IPs/CIDRs, vazia por padrão), tomando a primeira entrada fora dessas faixas; sem isso, todo o tráfego atrás de um proxy compartilharia um único limite. Para desenvolvimento use o proxy `/api` do Vite; o cliente HTTP inclui `credentials: 'include'`.

`GET /session`, `POST /auth/login`, `POST /auth/demo` e `POST /auth/verify-email` retornam:

```json
{
  "environment": "development",
  "user": { "id": "...", "name": "Mariana", "email": "mariana@example.com" },
  "organization": {
    "id": "...", "name": "Aurora Contabilidade", "demo": false,
    "clientLimit": 30, "plan": "founder_pilot", "email": "",
    "retentionDays": 365, "reminderDays": [3, 6], "remindersEnabled": true,
    "createdAt": "2026-10-20T02:00:00Z"
  },
  "csrfToken": "...",
  "billing": {
    "enabled": true, "entitled": true, "plan": "escritorio", "status": "active",
    "periodEnd": "2026-11-22T18:51:00Z", "cancelAtPeriodEnd": false,
    "manageable": true, "activeRequests": 12,
    "limits": { "activeRequests": 65, "storageBytes": 32212254720, "clients": 30 }
  }
}
```

`billing.enabled` é falso onde `STRIPE_SECRET_KEY` não está configurada, e aí `entitled` é sempre verdadeiro: a instalação não cobra e nada é bloqueado. `entitled` é o único campo que decide algo; os limites são para a tela mostrar, nunca para ela aplicar.

- `POST /auth/register`: `{organizationName,name,email,password}`. Senha de 12 a 128 caracteres, Argon2id (64MiB,3 iterações,1 thread), e-mail único. Cria escritório vazio com três modelos iniciais; não cria clientes/documentos de demonstração. **Não devolve sessão:** responde `{status:"verification_sent",email}` e envia o link de confirmação. O escritório só fica acessível depois da confirmação. Sem SMTP configurado — situação possível apenas em desenvolvimento, já que produção exige SMTP — a conta nasce confirmada e a resposta é a sessão, como antes.
- `POST /auth/verify-email`: `{token}`. Confirma o endereço e abre a sessão, de uso único, válido por 24h. `410 verification_expired` para link usado, expirado, inexistente ou emitido para um e-mail que a conta não usa mais. Link frontend `/confirmar-email?token=...`.
- `POST /auth/resend-verification`: `{email}`; resposta genérica `{message}` em qualquer caso, inclusive para e-mail sem conta. Um link novo invalida o anterior e há intervalo mínimo de 2 minutos entre envios.
- `POST /auth/login`: `{email,password}`. Mensagem genérica para credenciais inválidas. `403 email_unverified` quando a senha está correta mas o e-mail nunca foi confirmado — verificado depois da senha, para a resposta não revelar o estado de uma conta a quem não tem a credencial.
- `POST /auth/demo`: cria um escritório de demonstração isolado, sem credenciais compartilhadas. Os documentos são PDFs sintéticos gerados sob autorização no endpoint de arquivo, marcados `sample:true`; e-mails externos ficam desativados. Não há sessão demo implícita. A sessão e o portal incluem `environment` (`development` ou `production`) para identificar claramente espaços locais antes do primeiro upload. Limites próprios: `DEMO_RATE_PER_HOUR` por IP (3 por padrão, `429` com `Retry-After: 3600`) e `DEMO_MAX_ACTIVE` demonstrações vivas ao mesmo tempo (200 por padrão, `503 demo_unavailable`). Cada demonstração é apagada integralmente após `DEMO_TTL_HOURS` (24h).
- `POST /auth/logout`: exige sessão+CSRF, revoga a sessão e limpa o cookie.
- `POST /auth/forgot-password`: `{email}`; resposta genérica `{message}`. O link é persistido antes da tentativa SMTP, e invalidado em caso de falha; expira em 30min. Link frontend `/redefinir-senha?token=...`.
- `POST /auth/reset-password`: `{token,password}`; invalida todas as sessões do usuário. Receber o link prova o controle do endereço, então também confirma um e-mail pendente — sem isso a conta ficaria com senha nova e ainda sem poder entrar.

### Expiração de escritórios não confirmados

Um job em processo (a cada minuto, junto do worker de lembretes) apaga por completo — organização, usuários, sessões, clientes, modelos, solicitações, tokens, entregas, auditoria e objetos no bucket — as organizações que existem sem ninguém ter pagado nem provado nada: demonstrações com mais de `DEMO_TTL_HOURS` e cadastros cujo e-mail nunca foi confirmado com mais de `UNVERIFIED_TTL_HOURS` (168h). Os objetos são removidos antes do estado: falhar ali deixa o estado intacto para a próxima rodada, enquanto apagar o estado primeiro perderia o único registro de quais objetos remover. Uma organização com um usuário confirmado nunca é tocada, e uma organização sem nenhum usuário também não — não é um cadastro abandonado conhecido, e apagá-la esconderia um defeito.

## Clientes

`Client` = `{id,organizationId,name,cnpj,contactName,email,phone,active,createdAt}`. Nome obrigatório de até 180 caracteres; responsável até120; telefone até30; e-mail opcional válido. CNPJ opcional armazenado em string, normalizado sem pontuação, suporta 12 caracteres alfanuméricos e dois verificadores numéricos. Não aplica checksum nesta entrega. CNPJ único por escritório; limite do plano conta clientes ativos. Não há exclusão física.

- `GET /clients`: `Client[]`, inclui arquivados.
- `GET /clients/{id}`: `Client`.
- `POST /clients`: `{name,cnpj?,contactName?,email?,phone?}` → `Client` ativo.
- `PUT /clients/{id}`: objeto com os campos editáveis. Campos ausentes do corpo mantêm o valor atual; enviar string vazia limpa o campo. Mantém ID/escritório/criação. Arquivar exige `active:false` explícito — omitir `active` nunca arquiva.
- `POST /clients/import`: `{csv:string,commit:boolean}` → `{clients:Client[],errors:[{row:number,message:string}],imported:number}`. Cabeçalhos `name,cnpj,contactName,email,phone` ou `nome,cnpj,responsavel,email,telefone`; vírgula ou ponto e vírgula. Limite1000 linhas/JSON1MiB. `commit:false` valida/prévia; `commit:true` grava atomicamente apenas se não houver erros. Linha0 representa limite global. As IDs só são atribuídas na gravação.

## Modelos

`Template` = `{id,organizationId,name,description,active,createdAt,items:TemplateItem[]}`. `TemplateItem` = `{id,name,instruction,required,maxFiles}`. De1 a30 itens, ao menos um obrigatório, maxFiles de1 a20, nome até180, instrução até1000. Itens têm ordem de array. Cada solicitação recebe cópia independente desses requisitos.

- `GET /templates` → `Template[]`, inclui arquivados, cada um com `active`; ativos primeiro, depois por nome. O cliente filtra: somente modelos ativos podem iniciar solicitação ou recorrência.
- `GET /templates/{id}` → `Template`.
- `POST /templates`: `{name,description,items}` → `Template`.
- `PUT /templates/{id}`: `{name?,description?,items?,active?}` → `Template`; edição nunca altera solicitações anteriores. Campos ausentes mantêm o valor atual. Arquivar usa `active:false` e reativar `active:true`.

## Solicitações e revisão

`Request` = `{id,organizationId,clientId,clientName,templateId,title,competence,dueDate,status,items,createdAt,updatedAt,recurring,scheduleId?,sensitive}`.

`RequestItem` = campos de TemplateItem + `{status,correctionReason?,correctionFileId?,files:DocumentFile[],reviews:Review[]}`.

`DocumentFile` = `{id,name,mimeType,size,createdAt,scanStatus,superseded,sample?,analysis?}`. Chaves de storage e hashes do conteúdo nunca saem da API. `scanStatus` é `clean`, `sample` (sintético) ou `unscanned_development` (somente bypass explícito no desenvolvimento). Histórico de arquivos substituídos permanece e usa `superseded:true`.

`Review` = `{action:'approve'|'correct',reason?,source?,userId,createdAt}`. `source:'jev'` identifica decisão tomada pela análise automática; nesse caso `userId` é vazio.

`Analysis` = `{status,verdict?,confidence,reason?,signals?,extractor?,model?,characters?,applied,error?,requestedAt?,completedAt?}`. `status` é `queued,running,completed,failed,disabled`; `verdict` é `approved,review,rejected`. `AnalysisSignal` = `{key,label,kind:'noul'|'score'|'choice',answer?,value,confidence}`. A análise só aparece em respostas do escritório: `PortalRequest` a remove, junto com o `userId` das revisões. Estados de item: `pending,uploaded,approved,correction_required`. Estado de solicitação é derivado, com prioridade correção/revisão, e inclui `pending,in_progress,under_review,correction_required,completed,cancelled`. `under_review` significa que existe documento para analisar, mesmo que outros estejam pendentes. Obrigatórios aprovados concluem somente se nenhum item enviado aguarda revisão/correção; opcionais sem envio não impedem conclusão. Encerradas não aceitam uploads.

- `GET /requests` → `Request[]` ordenadas por atividade desc; frontend filtra localmente.
- `POST /requests`: `{clientIds:string[],templateId,title,competence,dueDate,items?:TemplateItem[],recurring?:boolean,sensitive?:boolean}` → `Request[]`. Lote atômico, clientes ativos do escritório. `items` substitui a cópia do modelo para este lote. `recurring:true` também cria agenda mensal, mesmo dia local de hoje, vencimento baseado no prazo informado. A agenda preserva o snapshot dos itens usados, inclusive customizações pontuais, e a classificação sensível. Alterar o modelo não muda uma agenda já configurada; selecionar outro modelo na agenda cria um novo snapshot. `sensitive:true` exige cliente com e-mail e confirmação por código no portal.
- `GET /requests/{id}` → `Request`.
- `POST /requests/{id}/share` → `{url,token,expiresAt}`. URL frontend `/p/{token}`. Token aleatório256bits, apenas hash em banco; validade30dias. Cada compartilhamento cria token novo sem revogar anteriores.
- `POST /requests/{id}/revoke`: revoga todos os links existentes; novo compartilhamento emite outro.
- `POST /requests/{id}/cancel`: cancela, revoga links, preserva arquivos/histórico. Concluída não pode ser cancelada.
- `POST /requests/{requestID}/items/{itemID}/review`: `{action:'approve'|'correct',reason?,fileId?}` → `Request`. Só item `uploaded` pode ser revisado; motivo obrigatório para correção, até1000. Com vários arquivos ativos, `fileId` deve identificar o arquivo afetado; com apenas um ele é selecionado automaticamente. Somente esse arquivo é substituído no reenvio; os demais permanecem ativos. Correção gera tentativa real de e-mail, resultado visível em notificações.
- `POST /requests/{requestID}/items/{itemID}/analyze` → `Request`. Reenfileira a análise automática do arquivo ativo do item. Exige `AI_REVIEW_ENABLED=true` no servidor e `aiReview` diferente de `off` na organização (`409 analysis_disabled`); só item `uploaded` com arquivo liberado pelo antivírus (`409 invalid_transition` / `409 file_quarantined`). O enfileiramento ocorre após o commit.
- `POST /requests/{id}/notify`: `{channel:'email'|'whatsapp_opened'}` → `Delivery`. Só aceita solicitação aberta; limita repetição a1min. `accepted` significa aceitação SMTP, não entrega/leitura. `failed` inclui motivo sanitizado. WhatsApp manual registra exclusivamente `opened`.
- `GET /requests/{id}/export`: ZIP autorizado dos arquivos atuais liberados; nomes saneados; mantém documentos privados. Sem arquivos retorna422.

## Portal e arquivos privados

- `GET /portal/{token}` → `{environment:string,request:Request,organization:{name,demo},client:{name},maxUploadBytes:number}`. Link inválido, expirado, revogado ou solicitação cancelada retorna410.
- `POST /portal/{token}/items/{itemID}/upload`: multipart campo `file` → `Request` atualizado. Limite padrão20MiB; valida extensão e assinatura PDF/JPEG/PNG/HEIC, nome saneado, SHA-256 para retry idempotente, limite por item. Leitura da conexão móvel, ClamAV síncrono e storage ocorrem fora da transação de banco. O commit revalida token, cookie sensível, estados e quotas; objetos sem metadados confirmados são removidos. Os bytes ficam privados antes de serem disponibilizados. Scanner indisponível/inconclusivo falha fechado; malware retorna422. Upload de correção marca somente o arquivo indicado pelo escritório como substituído; preserva os demais arquivos ativos e o histórico. HEIC é recebido/download; conversão para prévia não implementada. A validação de HEIC aceita as marcas ISO-BMFF que o iPhone grava (`heic`, `heix`, `heim`, `heis`, `hevc`, `hevx`, `hevm`, `hevs`, `mif1`, `msf1`), sempre exigindo extensão `.heic`; antes somente `heic`/`heix`/`hevc`/`hevx` passavam, o que recusava fotos de versões recentes do iOS.
- `GET /files/{fileID}`: arquivo com sessão do mesmo escritório **ou** `?token={portalToken}` pertencente exatamente à solicitação. Arquivo nunca é público no storage. PDF/JPEG/PNG inline; HEIC attachment. Range requests não são implementadas nesta primeira versão.
- `POST /portal/{token}/code`: solicita código no e-mail do cliente; válido10min, no máximo cinco tentativas de conferência. O reenvio não espera o código expirar: os três primeiros envios ficam a um minuto de distância e, a partir daí, a espera passa a dez minutos (`429 code_recent`); uma confirmação bem-sucedida zera essa contagem. Só para solicitações sensíveis. Escritórios de demonstração não emitem códigos/e-mails externos.
- `POST /portal/{token}/verify`: `{code}` → `{verified:boolean,message?}`. Confirmação vale1h para este navegador em cookie HttpOnly separado por link; apenas o hash da credencial é persistido. Outro navegador com o mesmo link continua exigindo código; tentativa inválida retorna200 com `verified:false` para persistir contador. GET/upload/download sensível retornam403 `identity_required` até confirmar. O frontend desta entrega pode não expor essa opção avançada.

## Recorrência, notificações e escritório

`Schedule` = `{id,organizationId,name,templateId,clientIds,dayOfMonth,dueDays,active,lastRun?,nextRun?,createdAt,sensitive,items?}`.

- `GET /schedules` → `Schedule[]`.
- `POST /schedules`: `{name,templateId,clientIds,dayOfMonth,dueDays,active}` → `Schedule`.
- `PUT /schedules/{id}`: mesmos campos parciais → `Schedule`; pausar funciona mesmo se cliente/modelo foi arquivado.
- `GET /schedules/{id}/runs` → `ScheduleRun[]`, `{id,organizationId,scheduleId,clientId,competence,requestId,createdAt}`.
- Worker no monólito roda no início e a cada minuto. Competência é o mês anterior à execução; dia inexistente vira último dia. Restrição MySQL e trava de escrita impedem duplicação agenda/cliente/competência. Agendas novas começam na próxima execução futura; não há retroatividade de meses perdidos. Primeira mensagem usa SMTP e WhatsApp permanece manual. Em falha SMTP, tentativa é registrada e não há repetição automática silenciosa; usuário pode tentar manualmente. Worker bloqueia instâncias concorrentes via advisory lock MySQL.
- Lembretes seguem `reminderDays` relativos à criação, não são enviados após conclusão/cancelamento e usam chave lógica por solicitação/regra. Em retomada após inatividade somente o lembrete mais recente devido é tentado, evitando uma rajada de mensagens antigas.
- `GET /notifications` → `Delivery[]`: `{id,organizationId,requestId,channel,kind,status,error?,createdAt}`. Tentativas SMTP são persistidas como `pending` antes da chamada externa e o resultado é gravado após aceitação/falha. SMTP não mantém trava de banco; tentativas interrompidas por mais de5min viram `failed`, sem reenvio cego. Uma solicitação encerrada antes do despacho não é enviada.
- `GET /audit` → eventos `{id,organizationId,userId?,action,entityId,createdAt}`; sem conteúdo documental/tokens/credenciais.
- `GET /organization` → Organization.
- `PUT /organization`: `{name,email,remindersEnabled,reminderDays,retentionDays,aiReview}`. `aiReview` é `off`, `suggest` ou `auto`; omitido vira `suggest`. Dias únicos1..60, até5; retenção30..3650. `plan/clientLimit/demo` são controlados pelo servidor. Retenção é política registrada; eliminação automática e fluxo de direitos dos titulares ainda exigem implementação operacional antes de dados reais.
- `GET /health` → `{status:'ok'}` significa processo atendendo, sem alegar integridade de integrações externas.

## Análise automática de documentos

Desligada por padrão. Sem `AI_REVIEW_ENABLED=true` nenhum arquivo é analisado e todo documento continua indo para revisão humana — o comportamento anterior, sem mudança.

Pipeline, sempre **fora** da transação de upload e após o commit: o arquivo aceito é enfileirado em uma fila em memória do próprio binário (sem Redis, sem fila externa, dois workers, 256 jobs; fila cheia descarta o job e o escritório pode pedir de novo). O worker extrai o texto com Docling (`POST /v1/convert/file`, Markdown, OCR ligado) e, se o resultado tiver menos de240 caracteres, repete com PaddleOCR (`POST /ocr`, PaddleX), que é o caso da foto de documento sem camada de texto. O texto é truncado em `AI_MAX_CHARS` preservando início e fim, com marcação explícita do corte.

O texto vira estado de uma única chamada ao TypeSafe System One (`POST /v1/systemone`) com seis julgamentos independentes: `tipo_correto` (noul), `integridade` (noul), `legibilidade` (score0..3), `competencia` (choice), `titular` (choice) e `problema` (choice). O modelo julga; **a decisão é composta em código** em `internal/analysis/review.go`:

- **Reprovado** quando há impedimento declarado com confiança ≥ `AI_REJECT_CONFIDENCE`, quando `tipo_correto` ≤0,20, ou quando a legibilidade é nula com confiança. O motivo é a frase em português que o cliente lê no portal.
- **Aprovado** apenas quando todos os critérios concordam: tipo ≥0,90, integridade ≥0,85, legibilidade ≥2,0 e competência/titular/problema com confiança ≥ `AI_APPROVE_CONFIDENCE`. `competencia`/`titular` com resposta `ausente` não impedem aprovação, porque muitos documentos válidos não os informam.
- **Revisão** em todo o resto, incluindo resposta faltando, análise falhada ou probabilidade próxima de0,5. Falha de extração ou do modelo nunca aprova nada.

Aplicar o veredito depende da organização: `auto` move o item (aprova, ou marca `correction_required` com o motivo e dispara o e-mail de correção como efeito pós-commit); `suggest` apenas registra. Tudo é revalidado dentro da transação de escrita — pessoa que revisou antes do worker terminar prevalece, e o veredito tardio fica somente registrado (`applied:false`). Organização gravada antes do recurso lê como `suggest`.

`GET /audit` registra `analysis.requested`, `analysis.completed`/`analysis.failed` e, quando aplicado, `item.approve`/`item.correct` sem `userId`.

Limites conhecidos: os limiares são ponto de partida e precisam ser calibrados nos documentos do próprio escritório; a fila é em memória e um reinício perde jobs enfileirados (o item permanece `uploaded`, visível para revisão); o veredito é sobre o texto extraído, não sobre a imagem — assinatura, carimbo e adulteração visual não são verificados.

## Assinaturas e planos

Desligado por padrão. Sem `STRIPE_SECRET_KEY` nenhuma rota de cobrança existe, nenhum espaço é bloqueado e o cadastro nasce como `founder_pilot`, exatamente como antes de haver dinheiro.

Com a chave configurada, três planos mensais em BRL são vendidos por Stripe Checkout — `essencial` (R$97), `escritorio` (R$237) e `rede` (R$487) —, cada um ligado ao seu preço por `STRIPE_PRICE_*`. Um escritório criado a partir daí nasce **sem plano** e só abre quando a Stripe confirmar a assinatura.

- `POST /billing/checkout`: `{plan}` → `{url}`. Exige sessão e CSRF. Abre o Checkout hospedado (modo assinatura, locale pt-BR, coleta de CNPJ, cupons habilitados) e responde com a URL da Stripe. `422` para plano que esta instalação não vende, `409 demo_organization` para a demonstração, `409 already_subscribed` para quem já tem assinatura viva, `503 billing_disabled` sem Stripe configurada. Limite de10 por minuto por IP.
- `POST /billing/portal` → `{url}`. Abre o portal da própria Stripe, onde se troca o cartão, muda de plano e cancela. `409 no_subscription` para quem nunca assinou. Nenhum dado de cartão passa por este produto.
- `POST /billing/webhook`: entrada da Stripe. **Não** tem sessão, CSRF nem origem — é a única rota assim, e o que a protege é a assinatura `Stripe-Signature`, verificada por HMAC-SHA256 sobre `timestamp.corpo` com tolerância de5 minutos. Falha devolve `400` sem detalhe. Eventos tratados: `checkout.session.completed`, `checkout.session.async_payment_succeeded`, `customer.subscription.created/updated/deleted`, `invoice.paid`, `invoice.payment_failed`, `charge.dispute.created`, `charge.refunded` e `radar.early_fraud_warning.created`. Tipo não tratado é respondido `200` e ignorado; `500` só quando a falha é nossa, porque é o que faz a Stripe reentregar.

O que o webhook grava fica no payload da organização: `subscriptionStatus` (vocabulário da própria Stripe), `stripeCustomerId`, `stripeSubscriptionId`, `currentPeriodEnd`, `cancelAtPeriodEnd` e `billingUpdatedAt`. A organização é encontrada pelo **customer id**, não por metadado; `client_reference_id` é apenas o resgate quando o cliente ainda não foi gravado. Entrega mais antiga que `billingUpdatedAt` é descartada — entregas repetem e chegam fora de ordem, e aplicar uma antiga fecharia um escritório que acabou de pagar. Preço desconhecido grava o status e **não** escolhe plano.

O acesso segue o status: `active`, `trialing` e `past_due` abrem o escritório — a Stripe ainda está tentando o cartão em `past_due`, e cortar ali seria cortar por um atraso de bandeira. `canceled`, `unpaid` e `incomplete` fecham: toda rota autenticada responde `402 subscription_required`, exceto `GET /session`, `GET /organization`, `POST /auth/logout` e as duas rotas de cobrança, que são o que a tela de assinatura precisa. O portal do cliente (token público) continua funcionando: quem envia documento não é quem deixou de pagar. Demonstração e `founder_pilot` nunca são bloqueados.

Disputa, estorno e alerta de fraude gravam `billingAlert` e uma entrada de auditoria, e **não** suspendem ninguém: documento de cliente no meio de um fechamento não é garantia de cobrança.

Os limites do plano são aplicados pelo servidor, não só anunciados: solicitações **ativas** ao mesmo tempo (15/65/200 — concluída ou cancelada libera a vaga) e armazenamento (5/30/120 GB, sempre dentro do teto da instalação). `422 plan_limit` recusa o lote inteiro quando ele cruzaria a linha, e a recorrência para de criar no limite em vez de ser o caminho que o contorna. `founder_pilot` e `demo` não têm números próprios: valem os limites da instalação, como antes.

## Persistência e limites desta entrega

MySQL8 é o padrão, sem fallback. Migração `apps/api/migrations/001_initial.sql` cria tabelas relacionais de organizações, usuários, membros, clientes, modelos/itens, solicitações/itens/arquivos, tokens, agendas/execuções, notificações e auditoria. Ownership, índices de busca e unicidade de recorrência são colunas/constraints relacionais; snapshots JSON guardam agregados para evolução de campos. A versão inicial lê os agregados em cada transação e faz upserts sob trava; isso tem custo crescente e precisa de queries SQL por escritório antes de escala. DDL é idempotente e registrado em schema_migrations; a próxima evolução deve adicionar nova migração.

Desenvolvimento sem Docker é opção explícita `APP_ENV=development STORE_DRIVER=json`; gravação atômica+fsync de arquivo0600. Storage local privado só existe em development. `ALLOW_UNSCANNED_UPLOADS=true` só funciona em development e fica visível nos metadados; nunca marca arquivo como limpo. Produção recusa configuração sem HTTPS frontend, MySQL, S3 HTTPS, scanner e uma forma de enviar e-mail — `RESEND_API_KEY` (HTTPS) ou SMTP com TLS. O envio por HTTPS existe porque a plataforma pode bloquear a porta de SMTP: na Railway abaixo do plano Pro a conexão expira sem erro do provedor. Quotas configuráveis de storage são1GiB por escritório e500MiB por solicitação por padrão, incluindo histórico. Backup/restauração do MySQL+objetos, configuração operacional de chaves/SSE do provedor, política de eliminação, scanner em infraestrutura e entrega SMTP real ainda precisam ser homologados. MySQL em produção exige `tls=true` com certificado verificado. Cada PUT S3 em produção solicita SSE-S3 (`AES256`), verifica a confirmação em HEAD e remove/falha antes de publicar metadados se o provedor não confirmar. Desenvolvimento MinIO não exige SSE; criptografia dos discos MySQL/backups e gestão das chaves ainda são responsabilidades operacionais não homologadas. Não declarar prontidão de produção/LGPD.
