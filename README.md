# DocPing

Peça uma vez. Acompanhe tudo. Um monólito modular para coletar e conferir documentos de clientes de escritórios contábeis.

**Estado: base funcional do MVP, com fluxo completo implementado. Ainda não homologado para documentos reais ou pilotos pagos.** O frontend compila e os testes de código/API passam. A integração local com Docker, MySQL, MinIO, Mailpit e ClamAV passou. Validação visual/Playwright, infraestrutura de produção e recuperação de backups permanecem pendentes. Não há declaração de conformidade LGPD ou prontidão de produção.

## Executar localmente

Requisitos: Node.js 20.19+ (ou 22+), npm e Go 1.26+.

```sh
npm ci
npm run dev
```

Abra **http://localhost:5173**. A API atende em `http://localhost:8080`, acessada pelo proxy `/api` do Vite. Use o endereço `localhost` indicado: a origem de escrita é validada pelo servidor.

O comando escolhe explicitamente `APP_ENV=development STORE_DRIVER=json`, grava metadados atomicamente em `apps/api/.data/state.json` e mantém arquivos privados em `apps/api/.data/objects/`. O bypass local de antivírus fica identificado como `unscanned_development`. Use apenas arquivos fictícios nesse modo. Produção nunca usa esse fallback e recusa essas configurações.

Na página inicial, **Explorar demonstração** cria um escritório isolado com empresas e PDFs fictícios, identificado na interface. Não existe login demonstrativo automático, senha compartilhada nem envio externo de e-mail na demonstração. **Criar meu escritório** inicia uma organização vazia com três modelos prontos e conduz à primeira solicitação. Não pede cartão nem simula pagamentos.

O Vite recarrega o frontend automaticamente. Reinicie `npm run dev` após alterar Go. `Ctrl+C` encerra os processos do comando.

## Desenvolvimento com MySQL, armazenamento e e-mail

Com o Docker em execução:

```sh
cp -n .env.example .env
# Substitua os valores locais change-me antes de subir os serviços.
docker compose up --build -d
npm run dev:web
```

O comando de cópia preserva um `.env` existente. Nesta sessão ele já foi criado com valores aleatórios locais. Não troque senhas de um banco persistido sem atualizar também as credenciais no MySQL.

Serviços: API `8080` (ou `API_HOST_PORT`), MySQL `3306`, MinIO `9000`/console `9001`, Mailpit `8025` e ClamAV na rede interna. As portas publicadas ficam em loopback. A API executa a migração versionada na inicialização; se o scanner ainda estiver atualizando, o upload falha com mensagem e pode ser repetido. Não libera arquivos sem verificação nesse modo.

Para executar também o frontend em container:

```sh
docker compose --profile web up --build -d
```

Nesse caso, não execute o Vite na mesma porta. Nesta máquina, o `.env` local usa `API_HOST_PORT=18080` para evitar conflito; o Nginx continua em `http://localhost:5173` e usa a API pela rede interna. Com Vite separado, configure `API_PROXY_TARGET=http://127.0.0.1:18080 npm run dev:web` quando usar essa porta alternativa. Detalhes em [infra/README.md](infra/README.md), incluindo a distribuição local do MinIO e os requisitos distintos de produção. `.env` não entra no Git nem no contexto de build.

## Fluxos implementados

- Landing curta, login, criação do escritório, recuperação/redefinição de senha e onboarding dentro da primeira solicitação.
- Central operacional, filtros por estado/competência, busca por cliente/CNPJ, progresso derivado dos documentos e prioridades de revisão/atraso.
- Clientes com criação, edição, arquivamento, histórico e importação CSV com prévia, validação por linha e gravação atômica.
- Modelos editáveis, ordenação de itens e cópia dos requisitos preservada em cada solicitação.
- Criação individual ou em lote em três etapas; links temporários revogáveis; WhatsApp manual; tentativa SMTP com estado verdadeiro.
- Portal sem cadastro, upload independente com progresso real, retry, deduplicação por conteúdo, foto/arquivo, retomada dos envios já salvos e código adicional para solicitações sensíveis.
- Prévia privada de PDF/JPG/PNG, histórico de versões, aprovação, correção com motivo por arquivo e conclusão validada no servidor. HEIC permite envio/download; conversão de prévia fica pendente.
- Recorrências mensais idempotentes no fuso de São Paulo, último dia para meses curtos, snapshots, pausar/reativar e histórico de execução.
- Lembretes configuráveis, registro de tentativas e falhas de comunicação visíveis no painel; exportação ZIP autorizada.
- Análise automática opcional de cada arquivo: Docling extrai o texto (PaddleOCR cobre foto e digitalização sem camada de texto) e o modelo Jev julga seis critérios; o veredito é composto em código e pode aprovar, devolver com motivo ou escalar para revisão humana. Desligada por padrão e configurável por escritório.
- Argon2id, sessões HttpOnly/SameSite/Secure em produção, CSRF, isolamento por escritório, tokens de alta entropia com hash, expiração, auditoria, limites e scanner que falha fechado.

Preço e limite do plano não ficam em componentes. A cobrança do piloto é externa; não existe checkout simulado. A política de retenção pode ser registrada, mas não há rotina de eliminação automática pronta: isso está explicitado na interface e nos gates de lançamento.

## Estrutura

```text
apps/web       React + TypeScript + Vite + EasyUI + Tailwind
apps/api       Go, API REST, jobs, análise de documentos, persistência e migrações
infra          Containers e operação local
docs           Produto, arquitetura, experiência, aceite e segurança
scripts        Execução conjunta de desenvolvimento
```

TanStack Query controla os dados remotos; React Hook Form e Zod validam autenticação; o restante dos formulários usa controles acessíveis e validação autoritativa no backend. React Router organiza as telas, carregadas por rota. EasyUI Button, Form e Animated Tabs foram instalados pelo registry oficial do shadcn e personalizados; os componentes acessíveis complementares seguem os mesmos tokens.

## Primeira entrega de produto

1. [Posicionamento e análise dos concorrentes](docs/01-produto-e-competidores.md): matriz dos dez critérios, evidências públicas e limitações de acesso, especialmente MandaDocs.
2. [Arquitetura e modelo de dados](docs/02-arquitetura-e-dados.md): monólito, persistência, estados, fronteiras de autorização e decisões.
3. [Fluxo, mapa das telas e design system](docs/03-experiencia-e-design.md): objetivos, estados, responsividade e acessibilidade.
4. [Backlog por fases e critérios de aceite](docs/04-backlog-e-aceite.md).
5. [Segurança e condições para o piloto](docs/05-seguranca-e-piloto.md).
6. [Contrato efetivamente implementado](docs/api-contract.md) e [OpenAPI 3.1](apps/api/openapi.json).
7. [Validações executadas e pendências](docs/06-validacao-da-entrega.md).

Metas de 60 segundos para criar uma solicitação e até duas ações para aprovação são **metas de projeto**. Não foram medidas com usuários reais. Links/recorrência/revisão não são apresentados como diferenciais exclusivos.

## Verificações

```sh
npm run check
npm run build
npm test
cd apps/api
go test ./...
go test -race ./...
go vet ./...
```

Os testes Go usam HTTP real de teste, arquivos sintéticos, fixture SMTP e fixture do protocolo ClamAV. Cobrem autenticação, CSRF, segregação, uploads, correção/reenvio, fechamento, limites, concorrência, expiração/revogação, CSV, recorrência e lembretes. Não substituem a homologação com os serviços de produção.

A análise automática tem testes de unidade da composição do veredito, dos clientes HTTP (Docling, PaddleOCR e TypeSafe em servidores de teste) e do worker que aplica ou não o resultado. **Os serviços reais — docling-serve, PaddleOCR e a API da TypeSafe — não foram executados nesta entrega:** não há medição de acurácia, latência, custo por documento nem calibração dos limiares em documentos reais. Antes de ligar o modo automático em um escritório, isso precisa ser feito.

Teste MySQL opt-in, somente contra banco descartável e isolado:

```sh
MYSQL_TEST_DSN='<dsn-do-banco-de-teste>' go test ./internal/persistence -run MySQL -v
```

### Integração local real
+
+Com os containers saudáveis:
+
+```sh
+node scripts/integration-smoke.mjs --mysql --eicar
+```
+
+Foram executados e passaram **18 cenários** contra MySQL, MinIO, ClamAV e Mailpit reais: upload de PDF/PNG verificado, arquivos privados, SMTP, isolamento, correção por arquivo, reenvio, conclusão, ZIP, OTP por navegador, revogação e rejeição de uma fixture EICAR incorporada em PDF, sem persistir o arquivo. O teste direto MySQL também passou, sem skip. O script lê o `.env` sem imprimir segredos e aceita somente endpoints locais. Relatório sanitizado em `test-results/integration-smoke.json`.
+
+A fixture EICAR usa um arquivo incorporado em PDF válido; a primeira tentativa com assinatura apenas anexada após o EOF não era adequada e foi corrigida antes da rodada aprovada. Esse teste de assinatura não prova detecção de todas as ameaças.
+
+### Interface ponta a ponta

Os testes Playwright estão escritos em `apps/web/e2e`. Os fluxos não puderam ser validados nesta entrega: a política salva de acesso do navegador bloqueou `http://localhost:5173`, inclusive depois de autorização explícita na conversa. Uma tentativa do runner falhou ao iniciar o Chromium, antes de abrir as telas; os sete casos foram listados com sucesso após corrigir o repasse de argumentos do comando. O build e os testes sem navegador foram concluídos sem contornar esse bloqueio.

Após liberar o endereço nas permissões do navegador, inicie o app contra dados de teste e execute:

```sh
npx playwright install chromium
npm run test:e2e
```

Há projetos desktop 1440 px e mobile 390/360 px, jornada de cadastro até conclusão, fluxo de correção, snapshots de modelos, recorrências, overflow e teclado. Para gerar a captura **real** de demonstração usada na landing após a validação:

```sh
CAPTURE_PREVIEW=1 npm run test:e2e -- --project=desktop-chromium layout.spec.ts
```

Enquanto essa captura não existe, a landing oferece acesso à demonstração e não apresenta uma imagem fabricada como screenshot.

## Antes de receber documentos reais

Homologar a implantação completa, backups e restauração, criptografia do armazenamento, provedor de e-mail, scanner e limites em infraestrutura real; executar os testes visuais/mobile/teclado e testes de usabilidade; concluir privacidade/termos/contratos e procedimentos de retenção/exclusão. A persistência MySQL inicial usa colunas/constraints relacionais com snapshots de agregados JSON e lê o estado em transações: precisa de consultas por escritório/paginação e validação de carga antes de escala. Não use o modo JSON local como serviço para clientes.
