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
- **Tempo real:** Socket.io
- **Observabilidade:** OpenTelemetry → Grafana Tempo (traces) + Prometheus (métricas) + Grafana (dashboard)

## Variáveis de ambiente

Crie um arquivo `.env` na raiz:

```env
PORT=8000
MONGODB_URI=mongodb+srv://<user>:<password>@cluster.mongodb.net/
MONGO_DATABASE=dbitems
JWT_SECRET=seu-secret-aqui
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4318
```

## Rodando localmente

```bash
go run cmd/main.go
```

## Rodando com Docker Compose

```bash
docker compose up
```

| Serviço    | URL                    |
|------------|------------------------|
| API        | http://localhost:8000  |
| Grafana    | http://localhost:3000  |
| Prometheus | http://localhost:9090  |
| Tempo      | http://localhost:3200  |

O dashboard **HTTP Requests** é provisionado automaticamente no Grafana com painéis de request rate, error rate, latência P50/P99 e distribuição de status codes por rota.

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
