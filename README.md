# ms-json-format

Microserviço para salvar e compartilhar JSONs formatados. Não requer cadastro para compartilhar — qualquer pessoa pode criar e compartilhar um JSON via link.

## Funcionalidades

- Criar, visualizar e deletar JSONs formatados sem precisar de conta
- Usuários autenticados podem criar JSONs privados e vinculá-los a organizações
- Compartilhamento em tempo real via Socket.io
- Limpeza automática de JSONs anônimos expirados (7 dias)

## Stack

- **Runtime:** Go + Fiber v3
- **Banco:** MongoDB
- **Auth:** JWT
- **Tempo real:** Socket.io (servidor `zishang520/socket.io` v3, protocolo v4: clientes `socket.io-client` 4.x)

## Variáveis de ambiente

Crie um arquivo `.env` na raiz:

```env
PORT=8000
MONGODB_URI=mongodb+srv://<user>:<password>@cluster.mongodb.net/
MONGO_DATABASE=dbitems
JWT_SECRET=<mínimo 32 caracteres; gere com: openssl rand -base64 48>
JSON_ENCRYPTION_KEY=<gere com: openssl rand -base64 32>

# Verificação de email (Amazon SES)
APP_URL=https://seu-front.com          # o link enviado é APP_URL/verify-email?token=...
SES_FROM=Json Format <no-reply@seu-dominio.com>   # identidade verificada no SES
AWS_REGION=us-east-1
# AWS_ACCESS_KEY_ID= / AWS_SECRET_ACCESS_KEY=      # ou AWS_PROFILE / role IAM (precisa de ses:SendEmail)
# SES_CONFIGURATION_SET=                # opcional
# MAIL_DEV_LOG=true                     # dev: sem SES_FROM, registra o email no log

# Atrás de proxy/load balancer (nginx, Cloudflare, Traefik...)
# SOCKET_CORS_ORIGINS=https://seu-front.com   # origens aceitas no websocket (padrão: todas)
# PROXY_HEADER=X-Real-IP                # header com o IP real do cliente, definido pelo proxy
# TRUSTED_PROXIES=172.16.0.0/12         # IPs/CIDRs dos proxies; o header só é aceito vindo deles
```

O conteúdo dos JSONs é salvo criptografado (AES-256-GCM) com `JSON_ENCRYPTION_KEY`.
Guarde essa chave fora do banco e dos backups: **se ela for perdida, os JSONs não podem ser recuperados.**

> Mudanças necessárias no front para esta versão: [docs/FRONTEND_MIGRATION.md](docs/FRONTEND_MIGRATION.md)

## Autenticação

- `POST /auth` → `{ token, refresh_token }`. Cada login cria uma **sessão** no banco (coleção `sessions`).
- `token` (access, JWT de 24h) vai em `Authorization: Bearer ...` e no websocket (`io(url, { auth: { token } })`).
  Ele carrega o id da sessão: depois de um logout, deixa de valer na hora.
- `refresh_token` é opaco (`<sessão>.<segredo>`); o banco guarda só o SHA-256 do segredo.
- `POST /refresh` com `{ refresh_token }` → novo par. Cada refresh token é de **uso único**: sempre guarde o novo.
  Reusar um token já trocado (após 30s) é tratado como roubo e encerra todas as sessões do usuário.
  A sessão expira após 7 dias sem renovação.
- `POST /logout` com `{ refresh_token }` encerra a sessão; `POST /logout-all` (autenticado) encerra todas.

### Verificação de email

- O cadastro envia um link `APP_URL/verify-email?token=...` (válido por 24h, uso único).
  A página do front deve enviar `POST /verify-email` com `{ token }`.
- `POST /verify-email/resend` (autenticado) reenvia o link (no máximo 1 por minuto).
- `GET /user/me` informa `email_verified`.
- Só contas com email confirmado podem ser adicionadas a organizações. Contas criadas antes
  da verificação existir são tratadas como verificadas.

## Links (slug)

Cada item tem um `slug` aleatório de 22 caracteres (128 bits), usado nos links e na API:
`GET/PUT/DELETE /items/:slug` e `join` do websocket. O `POST /items` devolve `{ InsertedID, slug }`.
O `_id` do Mongo é previsível e **não** abre itens criados depois do slug.

Itens antigos: rode `go run ./cmd/migrate-slugs` (gera slug para eles). Os links antigos
pelo `_id` continuam funcionando só para esses itens até `LEGACY_ID_LINKS=false`.

## Visualização única

Crie com `POST /items` e `{ "json": "...", "view_once": true }` (público ou de organização;
privado é recusado com `VIEW_ONCE_NOT_ALLOWED_FOR_PRIVATE`).

- `GET /items/:slug` devolve só o aviso (`view_once: true`, `json` vazio) e não consome nada,
  então prévias de link (WhatsApp, Slack...) são seguras. O front deve mostrar um botão "revelar".
- `POST /items/:slug/reveal` entrega o conteúdo e **apaga o item** na mesma operação; com acessos
  simultâneos só um recebe. Depois disso o link responde 404.
- Não pode ser editado (`VIEW_ONCE_IMMUTABLE`), não entra no websocket e listagens não trazem o conteúdo.
- Expira em 7 dias se nunca for revelado (mesmo tendo dono).

## Rate limit

Limites por janela fixa, em memória (com várias instâncias, cada uma conta separadamente).
Excedido o limite, a resposta é `429 { "message": "TOO_MANY_REQUESTS" }` com `Retry-After`.

| Rota | Limite |
|---|---|
| Qualquer rota | 300/min por IP |
| `POST /auth` | 20/min por IP e 10 falhas/15min por conta |
| `POST /register` | 5/hora por IP |
| `POST /refresh` | 30/min por IP |
| `POST /verify-email` | 10/min por IP |
| `POST /items` | 30/min por usuário (ou IP, se anônimo) |
| `PUT /items/:id` | 120/min por usuário (ou IP, se anônimo) |
| `POST /items/:id/reveal` | 30/min por usuário (ou IP, se anônimo) |
| Websocket | 30 `join`/min e 30 `events`/s por conexão |

**Atrás de proxy**, configure `PROXY_HEADER` e `TRUSTED_PROXIES`: sem isso todos os clientes
chegam com o IP do proxy e compartilham o mesmo limite. Prefira um header que o proxy
sobrescreve (`X-Real-IP`, `CF-Connecting-IP`); com `X-Forwarded-For` o primeiro IP é controlado pelo cliente.

## Rodando localmente

```bash
go run cmd/main.go
```

## Rodando com Docker Compose

```bash
docker compose up
```

| Serviço | URL                   |
|---------|-----------------------|
| API     | http://localhost:8000 |

## API

### Público (sem autenticação)

| Método | Rota         | Descrição                        |
|--------|--------------|----------------------------------|
| POST   | `/items`     | Cria um JSON (anônimo ou logado) |
| GET    | `/items/:id` | Visualiza um JSON                |
| POST   | `/auth`      | Login                            |
| POST   | `/register`  | Cadastro                         |
| POST   | `/refresh`   | Renova o token JWT               |

### Autenticado (Bearer token)

| Método | Rota                              | Descrição                    |
|--------|-----------------------------------|------------------------------|
| GET    | `/items`                          | Lista seus JSONs             |
| DELETE | `/items/:id`                      | Deleta um JSON               |
| GET    | `/user/me`                        | Dados do usuário logado      |
| GET    | `/user/settings`                  | Configurações                |
| PUT    | `/user/settings`                  | Atualiza configurações       |
| GET    | `/organization`                   | Lista suas organizações      |
| POST   | `/organization`                   | Cria uma organização         |
| DELETE | `/organization/:id`               | Deleta uma organização       |
| POST   | `/organization/:id/users`         | Adiciona usuário à org       |
| DELETE | `/organization/:id/users/:userId` | Remove usuário da org        |
