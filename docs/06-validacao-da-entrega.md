# Validação da entrega

Registro de 19–20/10/2026, fuso America/Sao_Paulo. Todos os clientes, organizações, e-mails e documentos usados nos testes são sintéticos. Não foram enviados e-mails a destinatários externos. A aceitação SMTP foi comprovada apenas no Mailpit local.

## Resultado

O fluxo funcional implementado passou em testes de código e em **18 cenários contra serviços reais de desenvolvimento**. O frontend compila, é empacotado em container e responde via Nginx com o proxy da API funcionando. **A experiência visual/mobile ainda não foi validada no navegador; esta entrega não está liberada para documentos reais ou piloto pago.**

Duas mudanças posteriores a este registro **não estão cobertas pelos números acima**: a conversão da paleta para preto/branco/cinza e a análise automática de documentos (Docling + PaddleOCR + Jev). Para elas, veja "Mudanças posteriores" no fim deste documento.

## Evidências executadas

| Verificação | Resultado | Alcance |
|---|---|---|
| `npm run check` | Passou | TypeScript do aplicativo/configuração |
| `npm test` | 10 testes passaram | CSV com aspas/BOM/separadores/multilinha, datas civis em São Paulo e falhas de comunicação por última tentativa |
| `npm run build` | Passou | TypeScript + Vite; páginas carregadas por rota; artefatos em `apps/web/dist` |
| `npm audit --json` | Zero vulnerabilidades informadas | Dependências npm instaladas, não auditoria integral do produto |
| `go test -race -count=1 -json ./...` | 28 testes/subtestes passaram, sem corrida reportada | 20 testes de nível superior; nessa rodada MySQL foi explicitamente pulado por falta de DSN, e depois executado com sucesso na integração real |
| Testes focados posteriores de storage/autenticação | Passaram | Três cenários SSE-S3 no transporte de teste: confirmação AES256, ausência de confirmação com descarte e desenvolvimento sem SSE; metadata de ambiente e autenticação |
| `go vet ./...` | Passou | Análise estática do Go |
| Compose, com e sem perfil `web` | Configuração passou; imagens construídas e serviços iniciados | API, MySQL 8, MinIO, Mailpit, ClamAV e Nginx reais no Docker local |
| `node scripts/integration-smoke.mjs --mysql --eicar` | **18 cenários passaram** | Fluxo completo descrito abaixo; relatório sanitizado versionado em [validation/integration-smoke.json](validation/integration-smoke.json) |
| Comando `npm run dev` com API em porta alternativa | Passou | Vite e API responderam, proxy encaminhou corretamente e `Ctrl+C` não deixou processos nas portas de teste |
| HTTP do frontend e proxy | `200 OK` e `{status:ok}` | Nginx serviu `http://localhost:5173`; `/api/health` alcançou a API |
| `npm run test:e2e -- --list` | Sete casos descobertos | Confirma a preparação/discovery da suíte; não prova que os fluxos passaram |

O build emitiu avisos sobre comentários de pureza em uma dependência Zod. Foram avisos do empacotador; a compilação concluiu sem erros. Não se alterou código da dependência para ocultá-los.

## Integração com serviços reais

O script executou, sem mocks de API/storage/SMTP/scanner:

1. Saúde dos serviços locais.
2. Cadastro, cookie de sessão e organização real vazia.
3. Cliente e modelo reutilizável persistidos.
4. Bloqueio de mutação sem CSRF.
5. Solicitação, snapshot e link público persistidos.
6. PDF verificado pelo ClamAV e recuperado integralmente do MinIO privado.
7. PNG verificado e salvo independentemente.
8. Bloqueio de arquivo sem autenticação e objeto MinIO sem assinatura/autorização.
9. Bloqueio de consulta, alteração e download por outro escritório.
10. Aceitação SMTP e presença da mensagem no Mailpit.
11. Rejeição da fixture EICAR incorporada em PDF, sem gravar o arquivo no item.
12. Correção dirigida a um arquivo, preservando o arquivo não afetado e o histórico.
13. Aprovação, conclusão e bloqueio de novos uploads.
14. ZIP autorizado contendo apenas versões atuais.
15. Solicitação sensível: código recebido no Mailpit e autorização ligada ao navegador que confirmou.
16. Token de uma solicitação sem acesso ao arquivo de outra.
17. Revogação retirando acesso ao portal e aos arquivos.
18. `TestMySQLRoundTrip` contra MySQL real, sem skip.

A primeira fixture antivírus apenas anexava EICAR depois do EOF de um PDF e não foi detectada. A assinatura EICAR canônica foi rejeitada pelo ClamAV direto; a fixture foi corrigida para um EmbeddedFile de PDF válido e então o endpoint rejeitou o upload. O relatório aprovado refere-se à rodada final. Não se interpretou a primeira tentativa como evidência de rejeição e não se apresenta este teste de assinatura como garantia de detecção universal.

## Ajustes verificados na infraestrutura

- Docker Desktop estava desligado e foi iniciado. A variante ClamAV Alpine não tinha manifesto ARM64; foi substituída pela imagem oficial Debian com tag/digest multi arquitetura verificados.
- Uma tag específica de container Go não estava publicada; o build passou com a linha oficial `golang:1.26-alpine`, previamente verificada no registry.
- A porta de API no host tornou-se configurável. O `.env` desta máquina usa `API_HOST_PORT=18080`; o Nginx em `5173` usa `api:8080` internamente. Configurações de outros serviços não foram modificadas para liberar uma porta.
- O `.env` local contém valores aleatórios e tem permissão `0600`; é ignorado pelo Git e pelos contextos de build. Nenhum segredo integra o relatório.

## Navegador e testes de interface

O acesso CUA a `http://localhost:5173` foi negado inicialmente e novamente após autorização explícita do usuário, por uma política salva de permissão. Não se contornou a restrição por outra superfície de navegação.

Ao tentar somente listar os testes, um erro no repasse de argumentos npm acionou o runner. O Chromium não conseguiu iniciar no ambiente; os sete casos falharam na inicialização, antes de abrir as telas. O comando foi corrigido, e a listagem posterior encontrou os sete casos corretamente. **Nenhuma jornada de UI, captura visual, inspeção de sobreposição, foco ou leitor de tela foi aprovada a partir dessas tentativas.**

A suíte preparada cobre cadastro até conclusão, upload no portal sem login em contexto separado, correção/reenvio, snapshot do modelo, CNPJ, recorrência, teclado/Escape e larguras 1440, 390 e 360 px. Após liberar efetivamente o acesso ao navegador:

```sh
docker compose --profile web up -d
npm run test:e2e
```

A captura para a landing só deve ser produzida da aplicação real:

```sh
CAPTURE_PREVIEW=1 npm run test:e2e -- --project=desktop-chromium layout.spec.ts
```

Sem captura, a landing mostra acesso à demonstração e uma indicação textual; não apresenta um mock como screenshot real.

## Pendências antes do piloto

Executar e corrigir a suíte visual/teclado/mobile, testar dispositivos reais e medir as metas de usabilidade; homologar HTTPS, SMTP transacional externo, SSE-S3 e criptografia do banco/backups em infraestrutura de produção; executar backup/restauração e procedimentos de retenção/eliminação; preparar e revisar aviso de privacidade, termos e contratos; medir carga e evoluir a persistência por agregados/paginação. HEIC tem envio/download, mas não conversão para prévia. Os controles e integrações locais comprovados não eliminam essas pendências.

## Encerramento do ambiente

Ao final, o Vite/API de teste e os containers do projeto foram parados, preservando banco, objetos, mensagens, assinaturas e o `.env` local. Para retomar o pacote completo, execute `docker compose --profile web up -d` na raiz e abra `http://localhost:5173`. O Docker Desktop de uso geral não foi encerrado, e serviços de outros projetos não foram alterados.

## Mudanças posteriores a 20/10/2026

### Paleta neutra

Conversão mecânica das 692 cores em uso para uma escala neutra preservando a luminância de cada tom, com os tokens de estado (aprovado, em revisão, correção, erro) mantidos em matiz. `npm run check`, `npm test` e `npm run build` passaram depois da conversão.

**Não validado:** contraste medido nos pares efetivamente renderizados, aparência em navegador e em telas móveis, e revisão visual das sete folhas de estilo. A conversão é determinística, mas não substitui olhar as telas.

### Análise automática de documentos

| Verificação | Resultado | Alcance |
|---|---|---|
| `go test ./internal/analysis` | Passou | Composição do veredito, truncamento, normalização de confiança e os três clientes HTTP contra `httptest` |
| `go test ./internal/app -run Analysis` | Passou | Worker: aplica em `auto`, apenas registra em `suggest`, não aplica com modo ausente, não aprova em falha, não sobrepõe decisão humana, não vaza a análise ao portal |
| `go test -race ./...` | Passou | Conjunto completo, sem corrida reportada |
| `go vet ./...` | Passou | Análise estática |
| `docker compose config` | Passou | Serviços `docling` e `paddleocr` sob o perfil `ai` |
| `npm run check` / `npm test` / `npm run build` | Passaram | Tipos, 10 testes de lógica pura e empacotamento com o painel de análise |

#### Execução contra serviços reais, 20/10/2026

O pipeline completo foi executado com Docling real (`quay.io/docling-project/docling-serve:v1.37.0`, container local, arm64 nativo) e a API da TypeSafe com chave real. Documentos sintéticos, gerados para o teste.

| Cenário | Entrada | Veredito | Aplicado | Resultado |
|---|---|---|---|---|
| Documento errado | DAS da "ACME" num item que pedia *Notas fiscais* da "Horizonte Arquitetura" | `rejected` conf. 0,76 | Sim | Item → `correction_required` com motivo; `tipo_correto` 0,16 e `titular` divergente conf. 1,00 |
| Documento correto, porém incerto | NFS-e da Horizonte, layout achatado | `review` conf. 0,50 | Não | Item permaneceu `uploaded`; `integridade` 0,76 abaixo do piso 0,85 |
| Documento correto e completo | Extrato bancário da Lima Consultoria, 09/2026 | `approved` conf. 0,76 | Sim | Item → `approved`; todos os sinais ≥ 0,94, exceto `problema` em 0,76 |

Medições: Docling converteu o PDF de texto em **0,84 s**; a chamada ao Jev consumiu **704 tokens de entrada** no exemplo isolado. A imagem do Docling ocupa **15,4 GB**. A resposta real (`jev-1.13.0`) bate campo a campo com o que o cliente Go desserializa.

Os três caminhos do fluxo se comportaram como projetado, incluindo a escalada: o caso intermediário **não** foi aprovado automaticamente.

**Não validado, e necessário antes de ligar o modo automático em um escritório:**

1. O container `paddleocr` **não foi construído**. As versões fixadas em `infra/paddleocr.Dockerfile` não foram verificadas contra o índice do PyPI, e o caminho de fallback do `Chain` só tem cobertura por teste de unidade. Na prática o Docling já faz OCR, então o fluxo funciona sem ele.
2. Nenhum teste com documento **digitalizado ou fotografado**. Todos os PDFs do teste têm camada de texto, então o OCR propriamente dito nunca foi exercitado — só o parser.
3. Os limiares continuam **sem calibração**. A amostra foi de três documentos sintéticos, o que não mede taxa de falso positivo nem de falso negativo. O caso 2 mostra que o piso de integridade reprova documentos legítimos de layout incomum; em um escritório real isso significa fila de revisão maior que o necessário. **`suggest` continua sendo o modo defensável até existir uma amostra real medida.**
4. Acurácia, latência e custo por documento em volume real seguem sem medição. O teste usou PDFs de 1 a 2 KB.
5. O envio de conteúdo extraído a um serviço externo não foi avaliado sob a ótica de LGPD nem incluído nos gates de `docs/05-seguranca-e-piloto.md`.
6. O painel de análise não foi visto em navegador.

## Lacuna conhecida: retenção não é aplicada

`Organization.RetentionDays` é validado (30 a 3650 dias), persistido e editável em
Configurações, **mas nenhum job o aplica**. Não existe expurgo de documentos vencidos:
as chamadas a `Objects.Delete` removem objetos órfãos quando um upload falha e os
objetos das organizações que o job de limpeza expira (demonstrações e cadastros não
confirmados) — nenhuma delas olha o prazo de retenção.

Na prática, hoje o ajuste não produz efeito algum. Isso tem duas consequências:

1. A interface promete ao usuário um controle que não existe. Enquanto não houver
   expurgo, o campo não deveria sugerir descarte automático.
2. A landing page **não** afirma que documentos são descartados no fim do prazo — a
   afirmação foi removida em 20/10/2026, justamente por não ser verdadeira. Não a
   reintroduza antes de o job existir.

Implementar o expurgo exige decidir o que acontece com solicitações concluídas, se o
registro de auditoria sobrevive ao documento e se o escritório é avisado antes. Até lá,
trate a retenção como declaração de intenção, não como garantia.

## Mudança posterior: limites de criação de conta (21/10/2026)

Não coberta pelos 18 cenários acima. Removida também a promessa de teste gratuito da
landing e da tela de cadastro, por decisão de produto: não haverá teste gratuito.

**O que foi verificado nesta rodada**

| Verificação | Resultado | Alcance |
|---|---|---|
| `go build ./... && go vet ./...` | Passou | Toda a API |
| `go test ./...` | Passou | Suíte completa, incluindo os seis testes novos |
| `go test -race ./...` | Passou, sem corrida reportada | Mesma suíte |
| `npm run check` e `npm test` | Passaram (14 testes) | TypeScript e lógica pura do frontend |
| `npm run build` | Passou | Empacotamento com a rota `/confirmar-email` |

Os testes novos cobrem: teto de demonstrações vivas com liberação de vaga após a
expiração; orçamento por IP por hora com `Retry-After: 3600`; expiração de uma
demonstração com exclusão de organização, usuário, clientes e solicitações, e a sessão
deixando de autenticar; cadastro que não abre sessão, login recusado com
`email_unverified`, confirmação pelo link abrindo a sessão, link de uso único e login
liberado depois; expurgo do cadastro nunca confirmado com preservação do confirmado de
mesma idade; e reenvio com intervalo mínimo e resposta idêntica para e-mail sem conta.
A confirmação de e-mail é exercitada ponta a ponta contra um servidor SMTP falso em
processo, lendo o link da mensagem entregue.

**Não validado**

1. Nenhum e-mail de confirmação foi entregue por um provedor real — só pelo SMTP falso
   dos testes. O corpo, o assunto e a chegada à caixa de entrada (SPF/DKIM, spam) seguem
   sem verificação.
2. As telas novas (aguardando confirmação, `/confirmar-email`) **não foram vistas em
   navegador**, como todo o restante da interface nesta entrega.
3. O job de limpeza nunca rodou em processo de longa duração: nos testes ele é chamado
   diretamente, não pelo `StartWorker`.

## Mudança posterior: MySQL real e SMTP da Hostinger (21/10/2026)

**Ordem de exclusão, confirmada em MySQL real.** O que o registro anterior listava como
não executado foi executado: um MySQL 8.0.46 descartável em container, banco criado só
para a rodada e removido depois.

| Verificação | Resultado |
|---|---|
| `MYSQL_TEST_DSN=… go test ./internal/persistence -run MySQL -v` | `TestMySQLRoundTrip` e `TestMySQLPurgeRespectsForeignKeys` passaram |

A migração `002_email_verification.sql` foi aplicada nessa rodada, pelo caminho normal de
inicialização do `Open`. A ordem dos `DELETE` de `purgeOrder` está agora **observada**
contra as chaves estrangeiras reais, não apenas derivada do schema.

**SMTP com TLS implícito (porta 465).** O transporte só sabia elevar uma conexão em texto
claro com STARTTLS. A porta recomendada pela Hostinger responde em TLS antes do greeting,
e nela o código anterior não conseguiria sequer ler a saudação. `SMTP_TLS_MODE` passou a
escolher o modo, com a porta como padrão.

| Verificação | Resultado | Alcance |
|---|---|---|
| `go test ./internal/app -run "SMTP\|TLSMode\|Hostinger" -v` | Passou | Entrega em TLS implícito contra servidor TLS local; recusa de queda para texto claro; recusa de certificado não confiável; resolução do modo por porta; `LoadConfig` com a forma Hostinger nas portas 465 e 587 e recusa de modo inválido |
| Sondagem do servidor real `smtp.hostinger.com`, portas 465 e 587 | TLS 1.3, `CN=hostinger.com`, verificação OK pelas raízes do sistema, `AUTH PLAIN LOGIN`, `SIZE 48811212` | Handshake e EHLO pelo mesmo caminho de código do produto, **sem credenciais e sem enviar mensagem** |
| `go test -race ./...` e `go test ./...` | Passaram | Suíte completa |

**Envio real, executado em 21/10/2026.** `TestHostingerSMTPLive` (opt-in, lê a senha de
um arquivo) enviou uma mensagem por `smtp.hostinger.com:465` em modo `implicit`,
autenticando como a caixa e com um **alias** no remetente. O provedor autenticou e
aceitou a mensagem (`250` após `DATA`). Isso prova as credenciais, o TLS implícito ponta
a ponta e que a Hostinger aceita alias como remetente de uma caixa autenticada.

**Não validado**

1. A **chegada à caixa de entrada** não foi conferida por este registro: a aceitação pelo
   provedor não é entrega. O cabeçalho recebido — remetente efetivo, resultado de SPF,
   DKIM e DMARC do domínio — precisa ser lido na caixa de destino.
2. Nenhuma mensagem do produto em si foi enviada pela Hostinger: o teste chama
   `sendEmail` diretamente. Os e-mails de confirmação de cadastro e de lembrete nunca
   passaram por um provedor real.
3. A API HTTP de e-mail da Hostinger (`api.mail.hostinger.com`) **não foi implementada**,
   e a decisão agora tem base verificada contra a API real: o token é escopado a uma
   caixa, o envio é por `resourceId` de caixa e não existe campo de remetente no schema,
   então um alias não pode ser o remetente — alias não é caixa. Além disso cada envio é
   copiado para `INBOX.Sent` da caixa. O motivo está em `infra/README.md`.
4. O envio real pela Hostinger depende do teste opt-in `TestHostingerSMTPLive`, que
   **ainda não foi executado**: exige a senha da caixa, que não está nesta máquina.
