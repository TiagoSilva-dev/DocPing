# DocPing — proposta de produto e pesquisa competitiva

Data de consulta: **19/10/2026**, fuso America/Sao_Paulo. Esta é a primeira entrega de produto; metas e hipóteses abaixo não são resultados de testes nem alegações comerciais comprovadas. O estado da implementação está no README e no checklist de piloto.

## Posicionamento

DocPing atende inicialmente escritórios contábeis com 10–100 empresas clientes. A unidade de trabalho é uma solicitação de documentos de um cliente em uma competência. A promessa é: **“Peça uma vez. Acompanhe tudo. Pare de cobrar manualmente.”**

A oportunidade é reduzir o esforço do fechamento mensal: entrar, enxergar o atraso, conferir o que chegou e seguir ao próximo documento. Solicitação por link, portal sem cadastro, lembretes, modelos e recorrência são requisitos competitivos, não diferenciais exclusivos. IA, ERP, CRM e integração oficial de WhatsApp ficam fora do MVP.

## Método e limites

Foram consultados sites e artigos oficiais públicos. Não foram criadas contas de concorrentes, contratados planos nem acessadas áreas autenticadas. Passos descritos em manuais são evidência documental, não contagem observada de cliques. Tempo, sucesso de tarefa, acessibilidade, desempenho e qualidade visual do produto autenticado permanecem **não avaliados**.

O site do **MandaDocs** falhou ao abrir com e sem `www`, em HTTPS, e também em HTTP. A busca pública não retornou conteúdo oficial suficiente para inspecionar o produto nesta sessão. Isso é uma limitação de acesso da pesquisa, **não evidência de indisponibilidade do serviço**. As funções atribuídas a ele no brief — link, WhatsApp, portal sem login, lembretes, aprovação/recusa, modelos, recorrência, exportação e IA — são tratadas como informações fornecidas pelo solicitante, sem confirmação independente. Nenhum problema de usabilidade foi inferido dessa ausência.

## Comparação nos dez critérios

| Critério | MandaDocs | Nibo: evidência pública | Conta Azul: evidência pública | Decisão para o DocPing |
|---|---|---|---|---|
| 1. Primeiro cliente | Não observável | Cadastro individual/importação e cadastro de contatos documentados [N1, N2] | Ecossistema conecta empresa e contador; primeiro cadastro não testado [C1] | Nome, responsável e canal; CNPJ opcional; criar durante a solicitação |
| 2. Criar solicitação | Tempo e fluxo não observáveis | Protocolo de obrigações documentado; finalidade não equivale à coleta mensal [N3] | Solicitação cliente→contador e tickets de esclarecimento documentados; tarefas distintas [C2, C4] | Três etapas, modelo pronto e revisão final |
| 3. Enviar cobrança | Etapas não observáveis | Atendimento WhatsApp e envio de protocolo documentados; sem medir cliques [N3, N4] | Envio de anexos via WhatsApp documentado para Importações, não cobrança documental genérica [C5] | Copiar link, SMTP ou abrir `wa.me`; abertura não significa entrega |
| 4. Encontrar pendências | Não observável | Site anuncia controle de obrigações e painel central; eficácia não testada [N0] | Atendimento oferece filtros e situação; não prova central de coleta documental [C3] | Uma lista de atenção ordenada por atraso, correção e revisão |
| 5. Cliente no celular | Portal sem login informado no brief; não observado | Contatos usam Nibo Empresa; visualização deslogada é configurável [N2, N5] | Ticket pode ser respondido por link; upload móvel não testado [C4] | Portal web 360–430 px, sem cadastro, persistência a cada arquivo |
| 6. Aprovar rapidamente | Aprovação informada; velocidade desconhecida | Protocolo/conferência tem documentação, mas não houve teste equivalente [N3] | Atendimento aceita/recusa uma solicitação, não necessariamente arquivo individual [C3] | Aprovar arquivo exibido em uma ação, com confirmação do servidor |
| 7. Estados e próximo passo | Não observável | Protocolos e comunicações têm estados documentados [N3, N4] | Solicitações mostram situação e notificam por e-mail [C2, C3] | Texto de estado e próximo passo; cor apenas como apoio |
| 8. Repetir mensalmente | Recorrência informada; configuração não observada | Controle de obrigações não permite concluir equivalência exata de coleta recorrente [N0] | Há envio automático de NFC-e ao contador, de escopo específico [C6] | Modelo+clientes+dia+prazo; competência anterior; sem duplicar |
| 9. Consistência visual | Não avaliada | Páginas públicas não permitem avaliar o produto autenticado | Páginas públicas não permitem avaliar o produto autenticado | Sistema próprio com EasyUI, teste visual desktop e mobile |
| 10. Configuração | Não observável | Modal de cliente reúne contatos, responsabilidades, obrigações e configurações [N5] | Vínculo com parceiro e configuração de importações aparecem nos manuais [C1, C5] | Implantar o fluxo de coleta com apenas escritório, cliente e modelo |

Não há vencedor medido. A conclusão é uma **hipótese de foco**: DocPing pode ser preferido por escritórios que desejam resolver a coleta sem operar módulos adicionais. Nibo e Conta Azul oferecem escopos mais amplos; isso não prova que sejam mais difíceis de usar. Contra MandaDocs, a hipótese depende principalmente de execução e usabilidade, e exige comparação prática autorizada.

## Plano de validação

Convidar cinco profissionais de pequenos escritórios e cinco pessoas que enviam documentos, com arquivos sintéticos. Pedir as mesmas tarefas em ambiente controlado; comparar concorrentes somente com contas autorizadas e fluxos equivalentes. Registrar conclusão sem ajuda, tempo, erros, retornos, cliques e percepção de esforço. Não usar tempo de leitura do manual como tempo de tarefa.

| Meta de projeto | Como medir |
|---|---|
| Primeira solicitação em até 60 s após os dados necessários estarem disponíveis | Cronometrar do início do fluxo à criação confirmada; separar tempo de cadastro |
| Identificar clientes atrasados em uma tela | Pedir os atrasados do mês sem instruir filtro; registrar erro e navegação |
| Aprovar em no máximo duas ações | A partir do documento visível; contar ações e falhas do servidor |
| Enviar no celular sem cadastro | Navegador real, 360/390/430 px, câmera e seletor de arquivos |
| Retomar envio incompleto | Enviar um arquivo, fechar, reabrir link válido e verificar persistência |
| Repetir sem recadastrar itens | Gerar competência seguinte e verificar cópia do modelo e ausência de duplicata |

## Fontes primárias

- M0 — [MandaDocs](https://www.mandadocs.com.br/): tentativa de acesso; conteúdo não recuperado.
- N0 — [Nibo — soluções](https://www.nibo.com.br/).
- N1 — [Introdução ao Nibo Obrigações](https://ajuda.nibo.com.br/pt-BR/articles/7157220-introducao-ao-nibo-obrigacoes).
- N2 — [Como cadastrar os contatos](https://ajuda.nibo.com.br/pt-BR/articles/8916513-como-cadastrar-os-contatos).
- N3 — [Protocolos](https://ajuda.nibo.com.br/pt-BR/articles/7829239-protocolos).
- N4 — [Nibo via WhatsApp Web](https://ajuda.nibo.com.br/pt-BR/articles/12556333-nibo-via-whatsapp-web).
- N5 — [Novas abas do modal de clientes](https://ajuda.nibo.com.br/pt-BR/articles/16201947-conheca-as-novas-abas-do-modal-de-clientes).
- C0 — [Conta Azul — produto](https://contaazul.com/).
- C1 — [Vantagens do contador conectado](https://ajuda.contaazul.com/hc/pt-br/articles/8652428651277-Conex%C3%A3o-com-parceiros-quais-s%C3%A3o-as-vantagens-de-ter-o-contador-conectado).
- C2 — [Enviar mensagens e documentos ao contador](https://ajuda.contaazul.com/hc/pt-br/articles/8266654678541-Conex%C3%A3o-com-parceiros-enviar-mensagens-e-documentos-para-o-contador).
- C3 — [Atendimento de solicitações de clientes](https://ajuda.contaazul.com/hc/pt-br/articles/360047724712-Conta-Azul-Mais-atendimento-de-solicita%C3%A7%C3%B5es-de-clientes).
- C4 — [Mister Contador: pedir informações por ticket](https://ajuda.contaazul.com/hc/pt-br/articles/48798705192717-Mister-Contador-como-pedir-informa%C3%A7%C3%B5es-ao-cliente-pelo-Ticket).
- C5 — [Importações: documentos via WhatsApp](https://ajuda.contaazul.com/hc/pt-br/articles/44909955929613-Conta-Azul-Mais-Importa%C3%A7%C3%B5es-como-enviar-documentos-e-texto-via-WhatsApp).
- C6 — [Enviar NFC-e para o contador](https://ajuda.contaazul.com/hc/pt-br/articles/7880880703885-NFC-e-como-enviar-para-o-contador).
