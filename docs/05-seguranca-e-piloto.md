# Segurança, privacidade e liberação de piloto

**Esta entrega não certifica prontidão de produção nem conformidade plena com a LGPD.** A aplicação lida com documentos empresariais e pessoais. Critérios abaixo precisam de evidência no ambiente escolhido; testes unitários e execução local não substituem revisão operacional, jurídica nem restauração de backup.

O modo JSON/local existe para desenvolver com arquivos sintéticos. Se `ALLOW_UNSCANNED_UPLOADS=true` estiver habilitado, o resultado é explicitamente não verificado para desenvolvimento; isso impede considerar o ambiente apto a documentos reais.

## Modelo de ameaça mínimo

Ativos: bytes dos documentos, identidade dos clientes, credenciais/sessões, tokens de portal e histórico de revisão. Fronteiras: navegador↔API, usuário↔organização, link público↔solicitação, API↔storage e scanner, API↔SMTP. Principais ameaças: IDOR entre escritórios, vazamento de link, upload malicioso, sessão roubada, abuso de tentativas, mensagens indevidas e perda de dados.

Token público é uma credencial: quem recebe o link obtém o escopo concedido até expirar/revogar. Não colocá-lo em analytics, logs, erros, cabeçalhos de referência para terceiros ou títulos de página. Rotas do portal não carregam trackers. Token aleatório com hash no banco deve ser emitido com entropia criptográfica; troca de link revoga a credencial anterior quando solicitado. Solicitação sensível exige verificação adicional por código e falha fechada.

## Gate para qualquer piloto com dados reais

Estado desta lista: **pendente de homologação no ambiente de piloto**. Existem controles implementados e testes locais descritos abaixo; isso não libera os gates de infraestrutura, operação e experiência. Marcar apenas após executar e registrar data, responsável, versão e resultado.

| Gate | Evidência exigida | Impacto se faltar |
|---|---|---|
| HTTPS e cookies | TLS válido; redirecionamento; HttpOnly, Secure, SameSite; sem conteúdo misto; CORS/origin/CSRF validados | Bloqueia piloto |
| Credenciais | Argon2id parametrizado; recuperação com expiração e uso único; rate limit; logout/revogação testados | Bloqueia piloto |
| Isolamento | Duas organizações testadas em todas as rotas e downloads; vínculos filhos não cruzam escritórios | Bloqueia piloto |
| Portal | Hash de token, entropia, expiração, revogação, rate limit; código adicional para sensíveis | Bloqueia piloto |
| Arquivos privados | Bucket sem leitura pública, chaves opacas, autorização antes de bytes/URL curta, nenhum segredo no browser | Bloqueia piloto |
| Scanner e quarentena | ClamAV atualizado; benigno/EICAR isolado; erro de scanner não libera arquivo; quarentena inacessível | Bloqueia piloto |
| Tipos e limites | MIME real, extensão, 20 MB/configuração, nomes seguros, limite por organização/item, HEIC tratado com segurança | Bloqueia piloto |
| Criptografia em repouso | Banco, objetos e backups criptografados pelo provedor; política de chaves e acesso documentada | Bloqueia piloto |
| Jobs e e-mail | SQL real com duplicidade/virada do ano; SMTP real aceito; falhas claras; sem lembrete de concluída | Bloqueia piloto |
| Concorrência | Upload lento, scanner e SMTP indisponíveis não bloqueiam login/listagem/revisão de outro escritório; transações curtas e erro/rollback verificáveis | Bloqueia piloto com vários escritórios |
| Backup e recuperação | Banco+objetos restaurados em ambiente isolado; RPO/RTO acordados e demonstrados | Bloqueia piloto |
| Retenção e eliminação | Prazo configurado, expurgo verificável, tratamento dos backups, autorização de eliminação e direito de titular | Bloqueia piloto |
| Auditoria e observação | Eventos relevantes, acesso restrito, logs sem conteúdo/tokens; alertas de falha de storage/scanner/job/SMTP | Bloqueia piloto |
| Termos e operação | Aviso de privacidade, termos, contrato com escritório/provedores, contato e processo de incidente/solicitação de titular | Bloqueia piloto comercial |
| Experiência real | E2E persistido e mobile, teclado/foco/contraste; testes com pessoas usando arquivo sintético | Bloqueia declaração de metas atingidas |
| Comercial honesto | Limites no backend; cobrança do piloto externa; nenhum pagamento, integração ou depoimento fictício | Bloqueia oferta se houver simulação |

Compose é uma base local; não fornece sozinho TLS, criptografia de volumes, políticas de acesso de produção, entrega real por e-mail, backups nem recuperação. O MinIO local não é recomendação de infraestrutura comercial mantida. Não publicar o Compose local diretamente na internet.

## Procedimentos de segurança a implementar e validar

- Recuperação de senha não revela existência do e-mail; código/token é curto em validade, limitado em tentativas e armazenado por hash. Sessões antigas devem poder ser revogadas.
- API deriva organização da sessão; não confia em `organization_id` enviado pelo cliente. Toda consulta e relação inclui escopo. Acesso não autorizado retorna resultado que não revela o recurso.
- Upload usa limite de corpo, tempo, quota, identificador idempotente e detecção de conteúdo. Não executar conversores com acesso à rede, shell ou caminhos escolhidos pelo usuário. Prévia mantém autorização e cabeçalhos restritivos.
- Varredura indisponível gera erro ou conserva quarentena; não marca limpo para seguir o fluxo. Antivírus reduz risco, mas não prova ausência de todo conteúdo malicioso.
- Comunicação respeita preferências e conclusão. Dados pessoais não entram em métricas. Eventos operacionais registram IDs, tipo e tempos, sem cópia de documentos.
- Eliminação não é arquivamento. Solicitação de eliminação exige escopo, autorização e registro de cumprimento. Rotina remove objeto, metadados permitidos e tokens; retenções legais/contratuais devem ser definidas com apoio adequado.
- Incidente: conter acesso (revogar sessão/link/chave), preservar auditoria mínima, avaliar abrangência, acionar responsável e seguir procedimento aprovado de comunicação. Não enviar mensagens externas automaticamente sem autorização.

## Registro da entrega

O README registra comandos executados e limitações encontradas. Para liberar o piloto, acrescentar neste documento um registro com: commit/versão; ambiente; testes executados; configuração de storage/scanner/SMTP; evidência de restauração; riscos restantes; responsável que aceitou o risco. Até lá, usar apenas dados sintéticos claramente identificados.

## Evidências desta entrega — 19–20/10/2026

| Área | Resultado conhecido | Limite da evidência |
|---|---|---|
| Revisão estática da API | Sessão devolve campos permitidos sem hash de senha; mutações autenticadas usam CSRF; arquivos/tokens são autorizados por organização/solicitação; produção recusa JSON, storage local, MySQL sem TLS e bypass de scanner | Controles também exercitados no ambiente local; sem teste de penetração ou homologação de produção |
| Regressões de segurança | Corrigidos OTP compartilhado entre navegadores, código externo na demonstração e perda de sensibilidade na recorrência; adicionados testes específicos | Segundo navegador exige sua própria confirmação; demonstração continua isolada, sem SMTP externo |
| Integridade e concorrência | Corrigidos snapshot da recorrência, prazo civil após 21h e substituição só do arquivo afetado; upload/SMTP ocorrem fora da trava transacional | Testes locais não medem capacidade de múltiplos escritórios em MySQL real |
| Suíte Go | `go test -race -count=1 -json ./...` passou: 20 testes de nível superior, 28 incluindo subtestes; `go vet ./...` passou. `TestMySQLRoundTrip`, inicialmente ignorado nessa suíte, foi executado depois com MySQL real e passou | Fixtures de teste continuam distintas dos serviços reais; contagens finais após os testes SSE estão no relatório de validação |
| Frontend | Build confirmado e testes unitários executados; listagem dos sete casos Playwright confirmada | Os sete casos tentados falharam ao lançar Chromium (`EPERM`), antes de abrir telas. Fluxos UI, visual, foco/teclado/leitor de tela e celular real não foram validados |
| Infraestrutura real local | Compose subiu saudável com API Go, MySQL 8, MinIO, ClamAV, Mailpit e frontend Nginx. HTTP 200 do frontend e do proxy de health. Smoke `--mysql --eicar` passou 18 verificações, inclusive round trip MySQL sem skip | Arquivos sintéticos e SMTP de teste; não comprova entrega externa, TLS de produção, recuperação ou escala |
| Storage e scanner | PDF/PNG validados pelo ClamAV real e persistidos no MinIO privado; assinatura inofensiva EICAR embutida em PDF rejeitada pela API sem persistir. Acesso anônimo/cruzado negado; código sensível recebido via Mailpit e limitado ao navegador verificado | Checagem SSE de produção validada em três casos com mock, sem comprovar criptografia no provedor real |
| Privacidade e operação | Gates e procedimentos documentados | Criptografia em repouso, restauração, eliminação/retenção operacional, termos/contratos e resposta a titulares continuam pendentes |

A política salva de acesso da ferramenta de navegação bloqueou `http://localhost:5173` duas vezes mesmo após autorização explícita. Houve também tentativa de execução Playwright, interrompida por `EPERM` no lançamento do Chromium. A entrega não contém alegação de passagem dos fluxos UI nem captura fabricada. As metas de 60 segundos/duas ações e preferência frente a concorrentes permanecem sem medição com usuários. Evidências de integração: `test-results/integration-smoke.json`; comandos e resultados finais: `docs/06-validacao-da-entrega.md`.
