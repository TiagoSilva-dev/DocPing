# Experiência central, telas e sistema visual

Especificação inicial do MVP. As telas só são concluídas quando o comportamento persistido e os estados de erro passam pelos critérios de aceite; este mapa não representa um inventário de funções já entregues.

## O caminho mais curto

**Solicitar → Receber → Conferir → Concluir.**

1. Contador abre **Nova solicitação**. Escolhe cliente existente ou cadastra nome, responsável e canal no próprio fluxo.
2. Escolhe **Fechamento mensal**, ajusta a lista simples e informa competência e prazo.
3. Confere o resumo e cria. Copia o link, envia e-mail ou abre WhatsApp com mensagem pronta. Cada canal mostra seu resultado real.
4. Cliente abre o link no celular. Vê quem pediu, o prazo e os itens. Envia um arquivo por vez, com confirmação persistida. Pode sair e voltar.
5. Contador abre **Aguardando revisão**. Visualiza, aprova ou informa correção, avança ao próximo. O cliente corrige só o item afetado.
6. API conclui quando todos os obrigatórios estão aprovados. Não cobra depois da conclusão. O contador pode configurar a repetição mensal com a mesma lista.

Cadastro do escritório conduz a esse caminho, sem tutorial longo nem cobrança prévia. O modo com dados sintéticos deve ser identificado como demonstração e usar ambiente/organização separados. Landing page usa telas reais e nunca depoimentos ou ganhos inventados.

## Navegação

Desktop: **Início, Solicitações, Clientes, Modelos, Configurações**. Recorrências ficam em Solicitações como visão secundária, para preservar cinco entradas principais. Revisão se abre no contexto da solicitação e oferece “Próximo documento”. Mobile do escritório usa navegação compacta; portal do cliente tem apenas o contexto da solicitação e seus documentos.

Início é uma fila operacional. Ordenar por risco de prazo e ação necessária: vencido com pendência, correção, arquivo aguardando análise, prazo próximo. Cada linha tem cliente, competência, quantidade, prazo e ação. Indicadores são contagens dos dados reais; vazio é uma orientação útil, sem gráficos decorativos.

## Contrato comum a todas as telas

- Carregamento: indicador discreto, `aria-busy`; manter o contexto e impedir submissão duplicada.
- Erro de rede: mensagem simples com ação de repetir; preservar campos preenchidos. Erro de validação ao lado do campo e resumo focável.
- Sem permissão: explicar indisponibilidade com retorno seguro. Sessão expirada retorna ao login e conserva somente um destino interno validado.
- Teclado: ordem lógica, foco visível, link para conteúdo, botão com nome acessível; campos com `label` real. Diálogo retém e devolve foco, Escape fecha quando não perde envio.
- Leitor de tela: `h1` único, regiões nomeadas, mensagens de envio em `aria-live`, tabelas semânticas e ações identificadas pelo documento/cliente.
- Responsividade: conteúdo útil em 360 px, sem rolagem horizontal da página; tabelas podem ter rolagem interna identificada ou linhas compactas. Alvos de toque do portal de pelo menos 44×44 px.
- Cor não é o único sinal de estado. Contraste precisa ser verificado nos pares efetivamente renderizados, inclusive foco, controles e mensagens. Preferência de movimento reduzido remove transições não essenciais.

## Mapa de telas

Os estados comuns acima se aplicam a cada linha. A última coluna explicita o comportamento adicional de acessibilidade, mobile e retorno.

| Tela | Objetivo, dados e ação principal | Vazio / carregamento / falha específica | Mobile, retorno e acessibilidade |
|---|---|---|---|
| 1. Landing | Explicar coleta; prévia real; **Conhecer o DocPing** leva ao cadastro/piloto | Prévia com alternativa textual; sem métricas falsas; link de contato só quando configurado | Seções lineares, CTA legível; âncoras com nomes claros |
| 2. Login | E-mail e senha; **Entrar** | Credenciais inválidas sem revelar conta; processamento com botão desabilitado | Mostrar senha com nome acessível; retorno à landing e recuperação |
| 3. Cadastro | Nome do escritório, nome do usuário, e-mail e senha; **Criar escritório** | E-mail/formato/senha; limite e falha da API preservam entrada | Autocomplete correto; termos/privacidade antes de cadastro comercial; retorno login |
| 4. Onboarding | Primeiro cliente, modelo, competência e prazo; **Criar primeira solicitação** | Sem cliente: pequeno formulário; sem modelo: modelo inicial; erro preserva etapa | Progresso textual; voltar etapa conserva dados; opção explorar quando real |
| 5. Início | Atenção, pendências, revisão, fechamento; **Nova solicitação** | Escritório novo: criar primeira; sem pendência: estado de dia em ordem; contagens não viram zero em erro | Lista prioritária; foco na mudança de rota; retorno contextual do detalhe |
| 6. Solicitações | Busca cliente/CNPJ, competência, progresso, prazo e atividade; **Nova solicitação** | Sem resultado: limpar filtros; sem registros: criar; erro não parece lista vazia | Filtros nomeados, tabela/linhas legíveis; filtro fica na URL quando aplicável |
| 7. Criar solicitação | Para quem → documentos → resumo; **Criar e compartilhar** | Cliente arquivado, item vazio, competência/prazo inválido, envio parcial em lote | Etapas anunciadas; reordenação acessível por botões, não apenas arrastar; voltar conserva dados |
| 8. Detalhe | Cliente, período, itens, progresso, comunicação e histórico; **Revisar documentos** | Nenhum envio: compartilhar; expirado: gerar acesso autorizado; conflito de versão recarrega | Cabeçalho compacto; retorno à listagem e posição; progresso com texto |
| 9. Revisão | PDF/imagem privada, data, arquivo e motivo; **Aprovar** / **Solicitar correção** | Sem pendente: voltar ao detalhe; prévia indisponível: download autorizado; scanner bloqueia conteúdo | Prévia com título e alternativa; motivo obrigatório; próximo arquivo anuncia nome |
| 10. Clientes | Empresa, CNPJ, contato, ativo e histórico; **Novo cliente** | Sem registros: cadastrar/importar; CSV mostra erros por linha antes de confirmar | Busca e formulário curtos; arquivamento com consequência; voltar ao fluxo de origem |
| 11. Modelos | Nome, itens, obrigatoriedade e instruções; **Salvar modelo** | Modelos iniciais; item sem nome impede salvar; falha mantém edição | Subir/descer itens por botão; voltar à lista com alerta se houver perda real |
| 12. Recorrências | Modelo, participantes, dia, prazo, competência e histórico; **Criar recorrência** | Sem regras: explicar exemplo; duplicidade/falha por cliente visível; execução não simulada | Pausar/reativar com nome acessível; retorno Solicitações; timezone explícito |
| 13. Configurações | Escritório, preferências, limites e retenção; **Salvar alterações** | Valores atuais na carga; falha não confirma salvamento | Grupos com legendas; alterações perigosas autorizadas; retorno Início |
| 14. Portal público | Solicitante, cliente, competência, prazo, progresso e itens; **Enviar arquivo** | Sem arquivo: falta enviar; progresso real por upload; rede: repetir só arquivo falho | 360–430 px; câmera/seletor; estado anunciado; sem navegação de escritório |
| 15. Erro/expiração/conclusão | Explicar estado e próximo passo permitido | Link expirado/revogado: falar com escritório por contato configurado; concluída: comprovante textual | Sem dados sensíveis antes de validar acesso; foco no título; retorno não expõe área interna |

Não renderizar botões cuja ação ainda não exista. A apresentação de recurso futuro deve ser informativa e identificada, sem simular salvamento.

## Sistema visual inicial

Identidade: superfícies neutras em preto, branco e cinza, texto denso com espaço suficiente para leitura. A interface não usa cor de marca: a cor fica reservada aos sinais de estado, de modo que verde, âmbar e vermelho signifiquem sempre a mesma coisa e sejam vistos de longe numa lista. Cor nunca é o único sinal — ícone e rótulo textual acompanham todo estado. DM Sans para a interface; fonte de sistema como fallback. IBM Plex Mono é opcional em metadados técnicos, nunca necessária para compreender a tarefa. Ícones Lucide de um único estilo; ícones decorativos são ocultados de tecnologias assistivas.

| Token | Valor inicial | Aplicação |
|---|---|---|
| canvas | `#FAFAFA` | Fundo geral |
| surface | `#FFFFFF` | Áreas de conteúdo e diálogos |
| text | `#141414` | Texto e títulos |
| text-secondary | `#6B6B6B` | Apoio, sempre validar tamanho e contraste |
| primary | `#141414` | Ação principal e foco |
| primary-soft | `#F0F0F0` | Seleção sutil com texto escuro |
| border | `#E5E5E5` | Divisores; controles podem exigir borda mais escura |
| success | `#3F7D4F` em `#EEF5EF` | Aprovado/concluído |
| review | `#4A6A8A` em `#EEF2F7` | Aguardando revisão |
| warning | `#A9701F` em `#FDF4E6` | Prazo/correção, com rótulo textual |
| danger | `#A8453A` em `#FDF2F1` | Erro, falha e rejeição |
| spacing | 4, 8, 12, 16, 24, 32, 48 px | Escala única; evitar valores casuais |
| radius | 6 px campo, 10 px painel, 14 px diálogo | Arredondamento contido |
| shadow | `0 5px 18px rgb(20 20 20 / 3%)` | Somente superfícies elevadas |
| text-size | 12 metadado, 14 corpo compacto, 16 corpo/portal, 20/28/36 títulos | Corpo nunca reduzido para caber |
| control | 40 px desktop, ≥44 px portal | Botões e campos com rótulos visíveis |
| motion | 180 ms ease-out | Opacidade e deslocamentos pequenos; reduzir conforme preferência |

A conversão da paleta anterior para a escala neutra preservou a luminância de cada tom, então a hierarquia tonal das telas continua a mesma; apenas os tokens de estado mantiveram matiz. Os tokens de status são ponto de partida, sujeitos à verificação de contraste da implementação — ainda não medida nos pares renderizados. Hierarquia usa peso, alinhamento e espaço antes de adicionar cor. O progresso representa obrigatórios aprovados e mostra também a quantidade recebida, sem confundir “enviado” com “concluído”.

## EasyUI

A documentação e o registry oficiais devem confirmar nome, dependências e implementação antes de instalar um componente. Nesta fundação, priorizar **Button**; Form é útil quando compatível com a versão atual. Não instalar galerias de animação para preencher a aplicação. O componente instalado pertence ao código do projeto e é adaptado aos tokens.

Command Menu, Morphing Dialog, Notification Stack, Animated Tabs, Expandable Search e Smooth Accordion são candidatos condicionais, não requisitos de instalação. Usá-los só se reduzirem esforço na tarefa; todo efeito respeita movimento reduzido. Campos, listas, upload e tabelas podem ser componentes React semânticos próprios quando o registry não atende ao domínio. Ant Design, Material UI e Bootstrap não entram na aplicação.

Na implementação desta entrega, **Button, Form e Animated Tabs** foram instalados pelo registry oficial, revisados e adaptados à identidade do DocPing. Os componentes restantes não foram instalados indiscriminadamente. A validação visual em navegador e de acessibilidade interativa ainda está pendente; componentes semânticos e CSS sozinhos não comprovam esses critérios.

Fontes oficiais: [EasyUI](https://easyui.site/), [introdução](https://easyui.site/docs/introduction/), [repositório](https://github.com/Surajmaurya1/easyui). Registro da consulta em 19/10/2026 e componentes efetivamente instalados acompanham o README.
