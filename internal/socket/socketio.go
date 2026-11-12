package socket

import (
	"api-go/internal/access"
	"api-go/internal/auth"
	"api-go/internal/repositories"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/zishang520/socket.io/servers/socket/v3"
	"github.com/zishang520/socket.io/v3/pkg/types"
)

type notification struct {
	Data  string      `json:"data"`
	Room  socket.Room `json:"room"`
	Event string      `json:"event"`
}

// Erros devolvidos ao cliente (via ack ou evento "error_message").
const (
	errInvalidPayload = "INVALID_PAYLOAD"
	errNotFound       = "NOT_FOUND" // item inexistente ou sem permissão (não revela qual)
	errNotJoined      = "NOT_JOINED"
	errSessionEnded   = "SESSION_ENDED" // logout/revogação: a conexão é encerrada
	errRateLimited    = "TOO_MANY_REQUESTS"
)

// Limites por conexão (janela fixa). "events" é folgado por causa da edição ao vivo.
const (
	maxEventsPerSecond = 30
	maxJoinsPerMinute  = 30
)

type windowCounter struct {
	start time.Time
	n     int
}

func (w *windowCounter) allow(max int, window time.Duration) bool {
	now := time.Now()
	if now.Sub(w.start) >= window {
		w.start, w.n = now, 0
	}
	w.n++
	return w.n <= max
}

// Intervalo máximo entre revalidações da sessão em eventos frequentes
// ("events"). O "join" sempre revalida.
const revalidateEvery = 30 * time.Second

// connState guarda a identidade da conexão. O token é guardado para revalidar
// a sessão: depois de um logout/revogação a conexão deixa de ser autorizada.
type connState struct {
	mu            sync.Mutex
	validate      TokenValidator
	token         string
	userID        string
	lastValidated time.Time
	events        windowCounter
	joins         windowCounter
	// rooms mapeia a referência usada pelo cliente (slug ou _id antigo) para a
	// sala canônica do item, para que todos os clientes do item se encontrem.
	rooms map[string]socket.Room
}

func (st *connState) setRoom(ref string, room socket.Room) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.rooms == nil {
		st.rooms = map[string]socket.Room{}
	}
	st.rooms[ref] = room
}

func (st *connState) room(ref string) (socket.Room, bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	r, ok := st.rooms[ref]
	return r, ok
}

func (st *connState) allowEvent() bool {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.events.allow(maxEventsPerSecond, time.Second)
}

func (st *connState) allowJoin() bool {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.joins.allow(maxJoinsPerMinute, time.Minute)
}

// currentUser devolve o usuário da conexão revalidando a sessão quando
// necessário. ok=false significa que a sessão caiu e a conexão deve ser encerrada.
func (st *connState) currentUser(force bool) (userID string, ok bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.token == "" {
		return "", true // anônimo
	}
	if !force && time.Since(st.lastValidated) < revalidateEvery {
		return st.userID, true
	}
	if st.validate == nil {
		return "", false
	}
	if _, err := st.validate(st.token); err != nil {
		return "", false
	}
	st.lastValidated = time.Now()
	return st.userID, true
}

// TokenValidator valida o access token e a sessão (ver middleware.Auth.ValidateToken).
type TokenValidator func(token string) (*auth.Claims, error)

type server struct {
	items repositories.ItemsRepository
	orgs  repositories.OrganizationRepository
}

// Server é o servidor websocket em execução.
type Server struct {
	io   *socket.Server
	http *http.Server
	errs chan error
}

// Err recebe um erro se o servidor HTTP do websocket parar sozinho (ex.:
// porta em uso). Não recebe nada num Shutdown normal.
func (s *Server) Err() <-chan error { return s.errs }

// Shutdown desconecta os clientes (eles reconectam na próxima instância) e
// encerra o servidor HTTP, respeitando o prazo de ctx.
func (s *Server) Shutdown(ctx context.Context) error {
	s.io.Close(nil)
	return s.http.Shutdown(ctx)
}

// Start sobe o servidor websocket em addr. corsOrigins vazio aceita qualquer origem.
func Start(addr string, items repositories.ItemsRepository, orgs repositories.OrganizationRepository, validate TokenValidator, corsOrigins []string) *Server {
	srv := &server{
		items: items,
		orgs:  orgs,
	}

	opts := socket.DefaultServerOptions()
	origins := allowedOrigins(corsOrigins)
	opts.SetCors(&types.Cors{Origin: origins})
	if list, restricted := origins.([]string); restricted {
		// CORS só é aplicado pelo navegador; aqui o próprio servidor recusa o
		// handshake de origens fora da lista. Sem Origin (apps nativos,
		// servidores) a conexão é aceita.
		opts.SetAllowRequest(func(ctx *types.HttpContext) error {
			origin := ctx.Request().Header.Get("Origin")
			if origin == "" || slices.ContainsFunc(list, func(o string) bool { return strings.EqualFold(o, origin) }) {
				return nil
			}
			return errors.New("origin not allowed")
		})
	}
	io := socket.NewServer(nil, opts)
	mux := http.NewServeMux()
	mux.Handle("/socket.io/", io.ServeHandler(nil))
	running := &Server{
		io: io,
		// Só ReadHeaderTimeout/IdleTimeout: Read/WriteTimeout derrubariam as
		// conexões websocket, que são longas por natureza.
		http: &http.Server{
			Addr:              addr,
			Handler:           mux,
			ReadHeaderTimeout: 10 * time.Second,
			IdleTimeout:       120 * time.Second,
		},
		errs: make(chan error, 1),
	}
	go func() {
		if err := running.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			running.errs <- fmt.Errorf("websocket: %w", err)
		}
	}()

	nsp := io.Of("/items", nil)

	// Autenticação opcional no handshake: o cliente envia
	// io(url, { auth: { token: "<jwt>" } }). Sem token (ou token inválido) a
	// conexão segue como anônima e só consegue entrar em salas de itens públicos.
	nsp.Use(func(client *socket.Socket, next func(*socket.ExtendedError)) {
		st := &connState{validate: validate}
		if token := handshakeToken(client); token != "" {
			if claims, err := validate(token); err == nil {
				st.token = token
				st.userID = claims.ID
				st.lastValidated = time.Now()
			}
		}
		client.SetData(st)
		next(nil)
	})

	nsp.On("connection", func(clients ...any) {
		client, ok := clients[0].(*socket.Socket)
		if !ok {
			return
		}

		client.On("join", safe(func(args ...any) {
			args, ack := splitAck(args)
			if !stateOf(client).allowJoin() {
				reply(client, ack, errRateLimited)
				return
			}
			room, ok := firstString(args)
			if !ok {
				reply(client, ack, errInvalidPayload)
				return
			}
			userID, ok := stateOf(client).currentUser(true)
			if !ok {
				sessionEnded(client, ack)
				return
			}
			canonical, ok := srv.canJoin(userID, room)
			if !ok {
				reply(client, ack, errNotFound)
				return
			}
			stateOf(client).setRoom(room, canonical)
			client.Join(canonical)
			reply(client, ack, "")
		}))

		client.On("events", safe(func(args ...any) {
			args, ack := splitAck(args)
			if !stateOf(client).allowEvent() {
				reply(client, ack, errRateLimited)
				return
			}
			events, ok := parseNotification(args)
			if !ok || events.Event == "" || events.Room == "" {
				reply(client, ack, errInvalidPayload)
				return
			}
			if _, ok := stateOf(client).currentUser(false); !ok {
				sessionEnded(client, ack)
				return
			}
			// Só transmite para salas em que o cliente entrou (e foi autorizado no join).
			room, joined := stateOf(client).room(string(events.Room))
			if !joined || !client.Rooms().Has(room) {
				reply(client, ack, errNotJoined)
				return
			}
			client.Broadcast().To(room).Emit(events.Event, events.Data)
			reply(client, ack, "")
		}))

		client.On("disconnect", func(data ...any) {
			log.Println("closed", data)
		})
	})

	return running
}

// allowedOrigins converte SOCKET_CORS_ORIGINS (já separado em config). Vazio
// = "*": aceita qualquer origem, como na v2. É seguro porque a autenticação vai
// no token do handshake (não em cookies); restrinja aos domínios do front se
// quiser uma camada extra. Sem CORS configurado a v3 recusaria o websocket de
// qualquer outro domínio.
func allowedOrigins(origins []string) any {
	if len(origins) == 0 {
		return "*"
	}
	return origins
}

// canJoin aplica as mesmas regras da API HTTP. ref é o que o cliente usa no
// link (slug, ou _id em links antigos). Devolve a sala canônica do item.
func (s *server) canJoin(userID, ref string) (socket.Room, bool) {
	_id, err := s.items.ResolveRef(ref)
	if err != nil {
		return "", false
	}
	item, err := s.items.FindOne(_id, false)
	// Itens de visualização única são imutáveis: não há edição ao vivo.
	if err != nil || item.ViewOnce || !access.CanAccessItem(s.orgs, userID, item) {
		return "", false
	}
	return socket.Room("item:" + _id.Hex()), true
}

func stateOf(client *socket.Socket) *connState {
	if st, ok := client.Data().(*connState); ok {
		return st
	}
	return &connState{}
}

// sessionEnded avisa o cliente e encerra a conexão (saindo de todas as salas).
func sessionEnded(client *socket.Socket, ack socket.Ack) {
	reply(client, ack, errSessionEnded)
	client.Disconnect(false)
}

func handshakeToken(client *socket.Socket) string {
	token, _ := client.Handshake().Auth["token"].(string)
	return strings.TrimPrefix(token, "Bearer ")
}

// splitAck separa o callback de ack (se o cliente enviou um) dos argumentos.
func splitAck(args []any) ([]any, socket.Ack) {
	if n := len(args); n > 0 {
		if ack, ok := args[n-1].(socket.Ack); ok {
			return args[:n-1], ack
		}
	}
	return args, nil
}

func firstString(args []any) (string, bool) {
	if len(args) == 0 {
		return "", false
	}
	str, ok := args[0].(string)
	return str, ok && str != ""
}

// parseNotification aceita o payload como string JSON (formato atual do front)
// ou como objeto.
func parseNotification(args []any) (notification, bool) {
	var n notification
	if len(args) == 0 {
		return n, false
	}
	var raw []byte
	switch v := args[0].(type) {
	case string:
		raw = []byte(v)
	case map[string]any:
		b, err := json.Marshal(v)
		if err != nil {
			return n, false
		}
		raw = b
	default:
		return n, false
	}
	if err := json.Unmarshal(raw, &n); err != nil {
		return n, false
	}
	return n, true
}

// reply responde pelo ack, se houver; erros sem ack vão no evento "error_message".
func reply(client *socket.Socket, ack socket.Ack, errMsg string) {
	if ack != nil {
		if errMsg == "" {
			ack([]any{map[string]any{"ok": true}}, nil)
		} else {
			ack([]any{map[string]any{"ok": false, "error": errMsg}}, nil)
		}
		return
	}
	if errMsg != "" {
		client.Emit("error_message", errMsg)
	}
}

// safe impede que um panic em um handler derrube o processo inteiro
// (a biblioteca de socket não faz recover).
func safe(fn func(...any)) func(...any) {
	return func(args ...any) {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("socket handler panic", "panic", r, "stack", string(debug.Stack()))
			}
		}()
		fn(args...)
	}
}
