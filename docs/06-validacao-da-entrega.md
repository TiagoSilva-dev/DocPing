# Validação da entrega

Registro de 19–20/10/2026, fuso America/Sao_Paulo. Todos os clientes, organizações, e-mails e documentos usados nos testes são sintéticos. Não foram enviados e-mails a destinatários externos. A aceitação SMTP foi comprovada apenas no Mailpit local.

## Resultado

O fluxo funcional implementado passou em testes de código e em **18 cenários contra serviços reais de desenvolvimento**. O frontend compila, é empacotado em container e responde via Nginx com o proxy da API funcionando. **A experiência visual/mobile ainda não foi validada no navegador; esta entrega não está liberada para documentos reais ou piloto pago.**

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
