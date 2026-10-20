# Backlog por fases e critérios de aceite

Este é o plano completo do MVP. A marcação de concluído depende de execução, persistência, tratamento de erros e evidência de teste. Os itens não marcados não devem ser tratados como entregues por estarem neste documento.

## Sequência de implementação

| Fase | Resultado verificável | Trabalho prioritário | Condição para avançar |
|---|---|---|---|
| 1. Fundação e identidade | Repositório executável | React/TS/Vite, EasyUI oficial, tokens, Go, configuração, migração MySQL, Compose e contratos | Build e testes de base executados; modo local explícito; nenhum segredo versionado |
| 2. Design funcional | Fluxo navegável central | Início, solicitações, detalhe, revisão e portal; dados sintéticos separados | Desktop/mobile e estados de vazio/erro verificados; botões com ação |
| 3. Fluxo persistido | Solicitar→enviar→corrigir→aprovar→concluir | Conta/sessão, organização, clientes, modelos com snapshot, solicitação, link limitado, arquivo privado e auditoria | E2E com correção e teste cruzado entre duas organizações |
| 4. Produtividade | Rotina mensal praticável | Job mensal idempotente, e-mail, lembretes, WhatsApp manual, CSV com prévia, exportação e filtros | Casos de data/duplicidade, falha de provedor, conclusão e importação validados |
| 5. Segurança e piloto | Ambiente candidato ao piloto real | TLS, scanner, storage privado, backup+restauração, permissões, usabilidade, documentos jurídicos e operação | Todos os bloqueadores no checklist resolvidos com evidência |

Pilotagem assistida permite contratação e cobrança externas. Plano, preço e limites vêm do backend/configuração; o valor inicial sugerido de R$ 99/30 dias é proposta comercial, não checkout ou pagamento implementado. Trinta empresas é limite inicial configurável. Nenhum botão pode confirmar cobrança simulada.

## Aceite dos fluxos críticos

| Fluxo | Cenário e resultado obrigatório |
|---|---|
| Conta e sessão | Cadastro válido cria escritório e membro; senha é Argon2id; cookie HttpOnly/SameSite, Secure sob HTTPS; login inválido e rate limit não revelam cadastro; logout invalida sessão |
| Isolamento | Com duas organizações, trocar IDs em listagem, detalhe, upload, revisão, exportação e download nunca retorna registro/bytes da outra; consultar URL de objeto também não permite acesso público |
| Cliente | Nome obrigatório, CNPJ opcional preserva letras, contato validado; criar/editar persiste após recarregar; arquivar remove de novas seleções e mantém solicitações históricas |
| CSV | Antes de importar, mostrar colunas, dados e erro por linha; linhas inválidas não entram silenciosamente; confirmação indica exatamente criadas/rejeitadas; replay não duplica sem aviso |
| Modelo | Criar/editar/reordenar/arquivar; itens obrigatórios e opcionais persistem; mudar modelo depois não altera solicitações existentes |
| Nova solicitação | Selecionar cliente(s), modelo, competência e prazo; criação transacional por solicitação, resultado explícito do lote; nenhuma solicitação fica sem itens por falha parcial |
| Compartilhar | Link aleatório de alta entropia dá acesso apenas à solicitação, tem validade e revogação; copiar informa falha de clipboard; `wa.me` informa abertura manual, nunca entrega |
| Upload | PDF/JPG/PNG e HEIC suportado com pipeline seguro, até limite configurado; validação real do conteúdo, tamanho, nome e quantidade na API; erro preserva itens já enviados; scanner impede liberar conteúdo suspeito/não verificado |
| Retomada | Enviar primeiro arquivo, fechar navegador e reabrir link válido: item e arquivo persistem; segundo envio não depende do primeiro; tentativa duplicada não cria versão indistinguível |
| Portal sensível | Solicitação sensível exige código válido e temporário antes de listar conteúdo e subir arquivo; tentativas e reenvio de código limitados; verificação vinculada ao navegador por credencial separada; segundo navegador com o mesmo link continua bloqueado; demonstração não envia código externo |
| Revisão | Somente membro autorizado abre conteúdo limpo; aprovar confirma servidor; motivo da correção obrigatório; responsável/data/versão auditados; falha não deixa aprovação visual falsa |
| Reenvio | Cliente vê motivo, substitui apenas o item recusado; arquivo anterior e auditoria continuam disponíveis ao escritório; arquivo recusado não conta como aprovado |
| Conclusão | Item obrigatório pendente/corrigindo bloqueia conclusão; opcionais não enviados não bloqueiam, mas opcional já enviado precisa terminar a revisão/correção; conclusão/cancelamento bloqueia novos uploads e lembretes; alteração exige reabertura autorizada |
| Recorrência | Reexecutar regra/cliente/competência cria no máximo uma solicitação; preservar sensibilidade e documentos customizados; cliente arquivado/limites são tratados; meses de 28/29/30/31 dias e virada do ano em São Paulo passam; prazo civil independe do horário de criação; pausa preserva histórico |
| E-mail | Provedor aceita → registrar `accepted`; falha → registrar erro e opção de repetir; SMTP sem webhook não gera `delivered`; nenhuma comunicação mostra sucesso antes de retorno real |
| Lembrete | Configurar/desativar; dedupe por solicitação/tipo/agendamento; reavaliar estado antes de enviar; concluída/cancelada não recebe; falha/incerteza fica auditável |
| Exportação | Arquivos autorizados da organização, nomes seguros e inclusão apenas de versões permitidas; sem links privados permanentes; erro parcial explícito; não exceder recursos sem limite |
| Limites e plano | API aplica limite por organização, solicitação e arquivo, mesmo que o frontend seja burlado; exceder mostra motivo útil e não cria estado incompleto |

**HEIC** não pode ser declarado suportado apenas por constar no `accept` do seletor. É necessário validar formato, inspecionar com scanner e gerar prévia em processo isolado/limitado ou informar honestamente a ausência da prévia. Falha de conversão não pode servir conteúdo como imagem confiável.

## Validação mínima

Backend: autenticação, autorização, isolamento, clientes, snapshot de modelos, solicitações, arquivo, aprovação/correção/conclusão, expiração/revogação, datas, concorrência/idempotência, lembretes e estados de entrega. Usar banco MySQL real no teste de integração; teste JSON não substitui isso.

Frontend: formulário e validação, navegação, erro real da API, mudança de estado confirmada, portal e retomada. Playwright deve executar criação de escritório→cliente→solicitação→link→upload→correção→reenvio→aprovação→conclusão, com arquivo sintético. Repetir portal em 360/390/430 px e desktop, verificar foco, teclado, nomes acessíveis e movimento reduzido.

Operação: subir Compose, aplicar migrações em banco vazio e existente, enviar e-mail ao Mailpit, testar scanner benigno e EICAR em ambiente isolado, verificar bucket privado e restaurar backup. Usabilidade mede metas de 60 segundos/duas ações; teste automatizado sozinho não comprova preferência dos usuários.

## Fora do escopo

OCR, classificação por IA, integração oficial WhatsApp, ERP/financeiro/CRM, assinatura, app nativo, gateway de pagamento, construtor condicional, analytics complexo e microserviços. Não adicionar um deles para compensar falhas no fluxo principal.
