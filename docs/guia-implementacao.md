# App do Casal — Guia de Implementação (Go + Postgres)

Oct 24, 2026 · @thiago

## Visão geral

Este guia leva você do zero até a Fase 0 do app (contas, casal, lembretes com push) em Go e PostgreSQL, para fazer no VS Code, um passo de cada vez.

**Decisões já tomadas**

| Tema | Decisão |
| --- | --- |
| Linguagem | Go 1.24 ou mais novo |
| Estilo | Monólito com uma pasta por feature (handler, service, repository) |
| Banco | PostgreSQL 17 rodando em Docker |
| Acesso ao banco | `database/sql` com o driver `pgx` e SQL escrito à mão |
| Rotas HTTP | `net/http` com o `ServeMux` da biblioteca padrão (sem framework) |
| Senhas | `crypto/pbkdf2` da biblioteca padrão |
| Sessões | Token aleatório em cookie, guardado como hash no banco |
| Isolamento por casal | `household_id` em tudo, mais RLS do Postgres como segunda trava |
| Push | `webpush-go` |
| Migrações | `goose`, via linha de comando |

**Dependências externas (só três)**

- `github.com/jackc/pgx/v5` (driver do Postgres, usado pelo `database/sql`)
- `github.com/SherClockHolmes/webpush-go` (envio de Web Push)
- `github.com/pressly/goose/v3` (apenas como ferramenta de linha de comando, não entra no código)

**Como ler este guia**

- Cada passo termina com algo que você consegue rodar e testar.
- Os trechos de código são um ponto de partida: escreva, adapte e entenda cada um. Eles **não foram compilados nem executados** onde este guia foi escrito, então rode `go vet ./...` e os testes conforme avança e corrija o que aparecer.
- O nome do módulo usado é `casal`. Troque pelo nome real do produto quando definir.

## Pré-requisitos e VS Code

Você precisa de Go, Docker e algumas extensões do VS Code; o resto é instalado por projeto.

**Instalar**

- **Go 1.24 ou mais novo** (`go version` para conferir). O `crypto/pbkdf2` da biblioteca padrão só existe a partir do 1.24.
- **Docker** com o Docker Compose (`docker compose version`).
- **Git**.
- **psql** (cliente do Postgres), opcional mas útil para conferir o banco.
- **goose**, para as migrações: `go install github.com/pressly/goose/v3/cmd/goose@latest`.

**Extensões do VS Code**

| Extensão | Para quê |
| --- | --- |
| Go (do time do Go) | Autocompletar, `gopls`, depurar, rodar testes pelo editor |
| Docker | Ver e controlar os contêineres |
| SQLTools + driver PostgreSQL | Consultar o banco sem sair do editor |
| REST Client (ou Thunder Client) | Testar a API com arquivos `.http` |
| EditorConfig | Manter a formatação igual entre máquinas |

Ao abrir um arquivo `.go`, aceite a instalação das ferramentas do Go que o VS Code oferecer (`gopls`, `dlv`, `staticcheck`).

**Configuração sugerida** (`.vscode/settings.json`)

```json
{
  "go.formatTool": "gofmt",
  "editor.formatOnSave": true,
  "go.lintTool": "staticcheck",
  "go.testFlags": ["-count=1"]
}
```

**Conferir se está tudo certo**

```bash
go version
docker compose version
goose -version
```

## Estrutura de pastas

O código fica em uma pasta por feature dentro de `internal/`, e cada pasta tem seu próprio handler, service e repository.

```text
casal/
├── cmd/
│   └── api/
│       └── main.go            # ponto de entrada: liga tudo e sobe o servidor
├── internal/
│   ├── config/
│   │   └── config.go          # lê variáveis de ambiente
│   ├── httpx/
│   │   └── httpx.go           # JSON, erros e middlewares comuns
│   ├── auth/
│   │   ├── handler.go         # rotas: cadastro, login, logout, /me
│   │   ├── service.go         # regras de negócio
│   │   ├── repository.go      # SQL de usuários e sessões
│   │   ├── password.go        # hash e verificação de senha
│   │   ├── middleware.go      # descobre o usuário pela sessão
│   │   └── service_test.go
│   ├── household/
│   │   ├── handler.go         # criar casal, convidar, aceitar
│   │   ├── service.go
│   │   ├── repository.go
│   │   └── service_test.go
│   ├── reminder/
│   │   ├── handler.go         # criar, listar, resolver, adiar
│   │   ├── service.go
│   │   ├── repository.go
│   │   ├── worker.go          # dispara os lembretes vencidos
│   │   └── service_test.go
│   └── push/
│       ├── push.go            # interface Sender e tipos
│       ├── webpush.go         # implementação com webpush-go
│       ├── handler.go         # salvar a assinatura do aparelho
│       └── repository.go
├── migrations/
│   └── 00001_init.sql
├── docker-compose.yml
├── .env.example
├── .gitignore
├── Makefile
└── go.mod
```

**Regras desta estrutura**

1. Uma feature **nunca** lê nem escreve nas tabelas de outra. Quando precisar de algo, chama uma função pública do pacote dela. A única exceção é um JOIN somente leitura na tabela users, só para mostrar nomes.
2. O `handler` só traduz HTTP em chamadas ao `service` e de volta para JSON. Sem regra de negócio.
3. O `service` guarda as regras e depende de uma **interface** de repositório, o que permite testá-lo sem banco.
4. O `repository` é o único lugar com SQL.
5. Tudo em `internal/` é privado ao projeto: ninguém de fora consegue importar.
6. As features ligadas ao módulo opcional (Marmita, Finanças) entram depois como pastas novas, sem mexer nestas.

## Postgres com Docker

O banco roda em um contêiner, com os dados guardados em um volume para não se perderem quando você desligar.

**`docker-compose.yml`**

```yaml
services:
  db:
    image: postgres:17
    container_name: casal-db
    restart: unless-stopped
    environment:
      POSTGRES_USER: ${DB_USER}
      POSTGRES_PASSWORD: ${DB_PASSWORD}
      POSTGRES_DB: ${DB_NAME}
    ports:
      - "5432:5432"
    volumes:
      - casal-data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${DB_USER} -d ${DB_NAME}"]
      interval: 5s
      timeout: 3s
      retries: 10

volumes:
  casal-data:
```

**`.env.example`** (copie para `.env` e preencha; o `.env` não vai para o Git)

```bash
DB_USER=casal
DB_PASSWORD=troque-esta-senha
DB_NAME=casal
DATABASE_URL=postgres://casal:troque-esta-senha@localhost:5432/casal?sslmode=disable

HTTP_ADDR=:8080
COOKIE_SECURE=false
SESSION_TTL_HOURS=720

VAPID_PUBLIC_KEY=
VAPID_PRIVATE_KEY=
VAPID_SUBJECT=mailto:voce@exemplo.com
```

**`.gitignore`**

```text
.env
/bin/
*.log
```

**`Makefile`** (atalhos; use tabulação, não espaços, antes de cada comando)

```makefile
include .env
export

MIGRATIONS=./migrations

db-up:
	docker compose up -d db

db-down:
	docker compose down

db-psql:
	docker compose exec db psql -U $(DB_USER) -d $(DB_NAME)

migrate-up:
	goose -dir $(MIGRATIONS) postgres "$(DATABASE_URL)" up

migrate-down:
	goose -dir $(MIGRATIONS) postgres "$(DATABASE_URL)" down

migrate-status:
	goose -dir $(MIGRATIONS) postgres "$(DATABASE_URL)" status

run:
	go run ./cmd/api

test:
	go test ./... -count=1

vet:
	go vet ./...
```

**Subir o banco**

```bash
cp .env.example .env
make db-up
make db-psql      # deve abrir o prompt do Postgres; \q para sair
```

No Windows, rode estes comandos no WSL2 ou no Git Bash, e use o Docker Desktop com a integração do WSL ligada.

## Banco: migração inicial

A primeira migração cria as sete tabelas da Fase 0, e todo o resto do produto se apoia nelas.

**Tabelas**

| Tabela | Guarda |
| --- | --- |
| `users` | Cada pessoa, com e-mail, nome, senha (hash) e fuso horário |
| `households` | O casal |
| `household_members` | Quem pertence a qual casal (cada pessoa em no máximo um) |
| `household_invites` | Convites por link, com validade e uso único |
| `sessions` | Sessões de login (guarda só o hash do token) |
| `reminders` | Lembretes de uma pessoa para outra |
| `push_subscriptions` | Os aparelhos que aceitaram receber avisos |

**`migrations/00001_init.sql`**

```sql
-- +goose Up
-- +goose StatementBegin

CREATE TABLE users (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email         text        NOT NULL,
    name          text        NOT NULL,
    password_hash text        NOT NULL,
    timezone      text        NOT NULL DEFAULT 'America/Sao_Paulo',
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_email_lower_idx ON users (lower(email));

CREATE TABLE households (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE household_members (
    household_id uuid        NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    user_id      uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    joined_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (household_id, user_id),
    UNIQUE (user_id)  -- cada pessoa pertence a um casal por vez
);

CREATE TABLE household_invites (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    household_id uuid        NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    token_hash   text        NOT NULL UNIQUE,
    created_by   uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    expires_at   timestamptz NOT NULL,
    accepted_by  uuid        REFERENCES users (id) ON DELETE SET NULL,
    accepted_at  timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX household_invites_household_idx ON household_invites (household_id);

CREATE TABLE sessions (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash text        NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sessions_user_idx ON sessions (user_id);

CREATE TABLE reminders (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    household_id uuid        NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    created_by   uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    recipient_id uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    body         text        NOT NULL CHECK (length(body) BETWEEN 1 AND 500),
    remind_at    timestamptz NOT NULL,
    status       text        NOT NULL DEFAULT 'scheduled'
                 CHECK (status IN ('scheduled', 'sent', 'resolved', 'canceled')),
    sent_at      timestamptz,
    resolved_at  timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now()
);
-- o agendador procura só os pendentes, por isso o índice parcial
CREATE INDEX reminders_due_idx ON reminders (remind_at) WHERE status = 'scheduled';
CREATE INDEX reminders_household_recipient_idx ON reminders (household_id, recipient_id, status);

CREATE TABLE push_subscriptions (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    endpoint   text        NOT NULL UNIQUE,
    p256dh     text        NOT NULL,
    auth       text        NOT NULL,
    user_agent text        NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX push_subscriptions_user_idx ON push_subscriptions (user_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS push_subscriptions;
DROP TABLE IF EXISTS reminders;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS household_invites;
DROP TABLE IF EXISTS household_members;
DROP TABLE IF EXISTS households;
DROP TABLE IF EXISTS users;
-- +goose StatementEnd
```

**Aplicar e conferir**

```bash
make migrate-up
make db-psql
# dentro do psql:
\dt
```

Deve listar oito tabelas (as sete acima mais `goose_db_version`, que o goose cria para controlar as versões).

## Passo 1: projeto, configuração e health check

O objetivo deste passo é um servidor que sobe, conecta no banco e responde em `/healthz`.

**Criar o projeto**

```bash
mkdir casal && cd casal
git init
go mod init casal
mkdir -p cmd/api internal/config internal/httpx internal/auth internal/household internal/reminder internal/push migrations
go get github.com/jackc/pgx/v5@latest
```

Copie para a raiz os arquivos `docker-compose.yml`, `.env.example`, `.gitignore` e `Makefile` das seções anteriores e suba o banco com `make db-up`.

**`internal/config/config.go`**

```go
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DatabaseURL     string
	HTTPAddr        string
	CookieSecure    bool
	SessionTTL      time.Duration
	VAPIDPublicKey  string
	VAPIDPrivateKey string
	VAPIDSubject    string
}

func Load() (Config, error) {
	cfg := Config{
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		HTTPAddr:        getenv("HTTP_ADDR", ":8080"),
		VAPIDPublicKey:  os.Getenv("VAPID_PUBLIC_KEY"),
		VAPIDPrivateKey: os.Getenv("VAPID_PRIVATE_KEY"),
		VAPIDSubject:    getenv("VAPID_SUBJECT", "mailto:voce@exemplo.com"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL é obrigatória")
	}

	secure, err := strconv.ParseBool(getenv("COOKIE_SECURE", "false"))
	if err != nil {
		return Config{}, fmt.Errorf("COOKIE_SECURE inválida: %w", err)
	}
	cfg.CookieSecure = secure

	hours, err := strconv.Atoi(getenv("SESSION_TTL_HOURS", "720"))
	if err != nil || hours <= 0 {
		return Config{}, fmt.Errorf("SESSION_TTL_HOURS inválida")
	}
	cfg.SessionTTL = time.Duration(hours) * time.Hour

	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
```

O Go não lê o `.env` sozinho. O `Makefile` já exporta as variáveis, então rode sempre com `make run`.

**`internal/httpx/httpx.go`** (respostas JSON e middlewares comuns)

```go
package httpx

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

type ErrorBody struct {
	Error string `json:"error"`
}

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

func Error(w http.ResponseWriter, status int, msg string) {
	JSON(w, status, ErrorBody{Error: msg})
}

// Decode lê o corpo JSON com limite de 1 MB e rejeita campos desconhecidos.
func Decode(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

type Middleware func(http.Handler) http.Handler

// Chain aplica os middlewares de fora para dentro: o primeiro da lista é o mais externo.
func Chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

func Recover(log *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Error("panic", "err", rec, "path", r.URL.Path)
					Error(w, http.StatusInternalServerError, "erro interno")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func Logging(log *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			log.Info("request", "method", r.Method, "path", r.URL.Path,
				"status", rec.status, "dur", time.Since(start).String())
		})
	}
}
```

**`cmd/api/main.go`** (versão inicial, só com o health check)

```go
package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // registra o driver "pgx" no database/sql

	"casal/internal/config"
	"casal/internal/httpx"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("falha fatal", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := db.PingContext(r.Context()); err != nil {
			httpx.Error(w, http.StatusServiceUnavailable, "banco indisponível")
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	handler := httpx.Chain(mux, httpx.Recover(log), httpx.Logging(log))
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	log.Info("servidor iniciado", "addr", cfg.HTTPAddr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
```

**Testar**

```bash
make run
# em outro terminal:
curl -i http://localhost:8080/healthz
```

Deve responder `200` com `{"status":"ok"}`. Pare o banco (`make db-down`) e repita: deve responder `503`.

## Passo 2: contas e sessões

Depois deste passo, uma pessoa consegue se cadastrar, entrar, sair e consultar quem ela é em `/v1/me`.

**Como funciona**

1. No cadastro e no login, o servidor gera um token aleatório de 32 bytes.
2. O token vai para o navegador em um cookie `HttpOnly` (o JavaScript da página não consegue ler).
3. No banco fica só o **hash** (SHA-256) do token, então um vazamento do banco não entrega sessões válidas.
4. Cada requisição protegida lê o cookie, calcula o hash e procura a sessão ainda válida.

**`internal/auth/password.go`**

```go
package auth

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

const (
	pbkdf2Iterations = 600_000 // recomendação atual da OWASP para PBKDF2-HMAC-SHA256
	saltLen          = 16
	keyLen           = 32
)

var b64 = base64.RawStdEncoding

// HashPassword devolve "pbkdf2-sha256$iterações$salt$chave".
func HashPassword(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, pbkdf2Iterations, keyLen)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s", pbkdf2Iterations, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

func VerifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" {
		return false
	}
	iter, err := strconv.Atoi(parts[1])
	if err != nil || iter <= 0 {
		return false
	}
	salt, err := b64.DecodeString(parts[2])
	if err != nil {
		return false
	}
	want, err := b64.DecodeString(parts[3])
	if err != nil {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, iter, len(want))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(got, want) == 1
}

// NewToken gera um token aleatório para o cookie e o hash que vai para o banco.
func NewToken() (raw, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, HashToken(raw), nil
}

func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
```

**`internal/auth/repository.go`**

```go
package auth

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

type User struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Timezone string `json:"timezone"`
}

var (
	ErrEmailTaken         = errors.New("e-mail já cadastrado")
	ErrNotFound           = errors.New("não encontrado")
	ErrInvalidCredentials = errors.New("e-mail ou senha inválidos")
	ErrInvalidInput       = errors.New("dados inválidos")
)

// Repository é o que o service precisa do banco. Em testes, troque por um fake.
type Repository interface {
	CreateUser(ctx context.Context, email, name, passwordHash string) (User, error)
	UserByEmail(ctx context.Context, email string) (User, string, error) // usuário e hash da senha
	CreateSession(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error
	UserBySession(ctx context.Context, tokenHash string, now time.Time) (User, error)
	DeleteSession(ctx context.Context, tokenHash string) error
}

type PostgresRepo struct{ db *sql.DB }

func NewPostgresRepo(db *sql.DB) *PostgresRepo { return &PostgresRepo{db: db} }

func (r *PostgresRepo) CreateUser(ctx context.Context, email, name, passwordHash string) (User, error) {
	var u User
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO users (email, name, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id, email, name, timezone`,
		email, name, passwordHash).Scan(&u.ID, &u.Email, &u.Name, &u.Timezone)

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
		return User{}, ErrEmailTaken
	}
	return u, err
}

func (r *PostgresRepo) UserByEmail(ctx context.Context, email string) (User, string, error) {
	var u User
	var hash string
	err := r.db.QueryRowContext(ctx, `
		SELECT id, email, name, timezone, password_hash
		FROM users WHERE lower(email) = lower($1)`, email).
		Scan(&u.ID, &u.Email, &u.Name, &u.Timezone, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, "", ErrNotFound
	}
	return u, hash, err
}

func (r *PostgresRepo) CreateSession(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO sessions (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		userID, tokenHash, expiresAt)
	return err
}

func (r *PostgresRepo) UserBySession(ctx context.Context, tokenHash string, now time.Time) (User, error) {
	var u User
	err := r.db.QueryRowContext(ctx, `
		SELECT u.id, u.email, u.name, u.timezone
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.expires_at > $2`, tokenHash, now).
		Scan(&u.ID, &u.Email, &u.Name, &u.Timezone)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

func (r *PostgresRepo) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = $1`, tokenHash)
	return err
}
```

**`internal/auth/service.go`**

```go
package auth

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"
)

type Service struct {
	repo       Repository
	sessionTTL time.Duration
	now        func() time.Time
	dummyHash  string // usado para gastar o mesmo tempo quando o e-mail não existe
}

func NewService(repo Repository, sessionTTL time.Duration) (*Service, error) {
	dummy, err := HashPassword("senha-que-nunca-sera-usada")
	if err != nil {
		return nil, err
	}
	return &Service{repo: repo, sessionTTL: sessionTTL, now: time.Now, dummyHash: dummy}, nil
}

// Register cria a conta e já abre uma sessão. Devolve o token bruto para o cookie.
func (s *Service) Register(ctx context.Context, email, name, password string) (User, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	name = strings.TrimSpace(name)
	if addr, err := mail.ParseAddress(email); err != nil || addr.Address != email {
		return User{}, "", fmt.Errorf("%w: e-mail inválido", ErrInvalidInput)
	}
	if name == "" || len(name) > 80 {
		return User{}, "", fmt.Errorf("%w: nome é obrigatório (até 80 caracteres)", ErrInvalidInput)
	}
	if len(password) < 8 || len(password) > 128 {
		return User{}, "", fmt.Errorf("%w: a senha precisa ter de 8 a 128 caracteres", ErrInvalidInput)
	}

	hash, err := HashPassword(password)
	if err != nil {
		return User{}, "", err
	}
	u, err := s.repo.CreateUser(ctx, email, name, hash)
	if err != nil {
		return User{}, "", err
	}
	token, err := s.openSession(ctx, u.ID)
	return u, token, err
}

func (s *Service) Login(ctx context.Context, email, password string) (User, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	u, hash, err := s.repo.UserByEmail(ctx, email)
	if errors.Is(err, ErrNotFound) {
		VerifyPassword(password, s.dummyHash) // mesmo custo de tempo, resposta igual
		return User{}, "", ErrInvalidCredentials
	}
	if err != nil {
		return User{}, "", err
	}
	if !VerifyPassword(password, hash) {
		return User{}, "", ErrInvalidCredentials
	}
	token, err := s.openSession(ctx, u.ID)
	return u, token, err
}

func (s *Service) Logout(ctx context.Context, rawToken string) error {
	return s.repo.DeleteSession(ctx, HashToken(rawToken))
}

func (s *Service) UserFromToken(ctx context.Context, rawToken string) (User, error) {
	return s.repo.UserBySession(ctx, HashToken(rawToken), s.now())
}

func (s *Service) SessionTTL() time.Duration { return s.sessionTTL }

func (s *Service) openSession(ctx context.Context, userID string) (string, error) {
	raw, hash, err := NewToken()
	if err != nil {
		return "", err
	}
	if err := s.repo.CreateSession(ctx, userID, hash, s.now().Add(s.sessionTTL)); err != nil {
		return "", err
	}
	return raw, nil
}
```

**`internal/auth/middleware.go`**

```go
package auth

import (
	"context"
	"net/http"

	"casal/internal/httpx"
)

const CookieName = "casal_session"

type ctxKey struct{}

func UserFromContext(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(ctxKey{}).(User)
	return u, ok
}

// Require rejeita com 401 quem não tem sessão válida e coloca o usuário no contexto.
func Require(svc *Service) httpx.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, err := r.Cookie(CookieName)
			if err != nil {
				httpx.Error(w, http.StatusUnauthorized, "faça login")
				return
			}
			u, err := svc.UserFromToken(r.Context(), c.Value)
			if err != nil {
				httpx.Error(w, http.StatusUnauthorized, "sessão inválida ou expirada")
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, u)))
		})
	}
}
```

**`internal/auth/handler.go`**

```go
package auth

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"casal/internal/httpx"
)

type Handler struct {
	svc    *Service
	log    *slog.Logger
	secure bool
}

func NewHandler(svc *Service, log *slog.Logger, cookieSecure bool) *Handler {
	return &Handler{svc: svc, log: log, secure: cookieSecure}
}

func (h *Handler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/auth/register", h.register)
	mux.HandleFunc("POST /v1/auth/login", h.login)
	mux.HandleFunc("POST /v1/auth/logout", h.logout)
	mux.Handle("GET /v1/me", Require(h.svc)(http.HandlerFunc(h.me)))
}

type credentials struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	u, token, err := h.svc.Register(r.Context(), in.Email, in.Name, in.Password)
	if err != nil {
		h.fail(w, err)
		return
	}
	h.setCookie(w, token)
	httpx.JSON(w, http.StatusCreated, u)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	u, token, err := h.svc.Login(r.Context(), in.Email, in.Password)
	if err != nil {
		h.fail(w, err)
		return
	}
	h.setCookie(w, token)
	httpx.JSON(w, http.StatusOK, u)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(CookieName); err == nil {
		_ = h.svc.Logout(r.Context(), c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: h.secure, SameSite: http.SameSiteLaxMode})
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	httpx.JSON(w, http.StatusOK, u)
}

func (h *Handler) setCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(h.svc.SessionTTL()),
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *Handler) fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		httpx.Error(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrEmailTaken):
		httpx.Error(w, http.StatusConflict, err.Error())
	case errors.Is(err, ErrInvalidCredentials):
		httpx.Error(w, http.StatusUnauthorized, err.Error())
	default:
		h.log.Error("erro em auth", "err", err)
		httpx.Error(w, http.StatusInternalServerError, "erro interno")
	}
}
```

**Ligar no `main.go`** (depois de criar o `mux`, antes do `handler := ...`)

```go
authSvc, err := auth.NewService(auth.NewPostgresRepo(db), cfg.SessionTTL)
if err != nil {
	return err
}
auth.NewHandler(authSvc, log, cfg.CookieSecure).Routes(mux)
```

Acrescente `"casal/internal/auth"` aos imports.

**Testar com um arquivo `requests.http`** (extensão REST Client; ela guarda os cookies entre as chamadas)

```http
@host = http://localhost:8080

### cadastro
POST {{host}}/v1/auth/register
Content-Type: application/json

{"email": "ana@exemplo.com", "name": "Ana", "password": "senha-forte-123"}

### quem sou eu
GET {{host}}/v1/me

### login
POST {{host}}/v1/auth/login
Content-Type: application/json

{"email": "ana@exemplo.com", "password": "senha-forte-123"}

### sair
POST {{host}}/v1/auth/logout
```

**Cuidados de segurança**

- `COOKIE_SECURE=true` assim que houver HTTPS (em produção é obrigatório).
- `SameSite=Lax` impede que outro site faça o navegador enviar o cookie em um `POST`, o que protege contra CSRF na API.
- Falta ainda **limitar tentativas de login** por e-mail e por IP e **apagar sessões expiradas** (um `DELETE FROM sessions WHERE expires_at < now()` periódico). Anote como pendências.

## Passo 3: casal e convite

Depois deste passo, uma pessoa cria o casal, gera um convite, e a outra entra por ele, com o limite de duas pessoas garantido pelo banco.

**Regras**

- Quem cria o casal já entra como primeiro membro.
- Cada pessoa pertence a no máximo um casal (garantido por `UNIQUE (user_id)` na tabela).
- O convite vale 7 dias, serve uma vez e é guardado como hash, como as sessões.
- Um casal nunca passa de dois membros, mesmo se dois convites forem aceitos ao mesmo tempo. Para isso, a aceitação trava a linha do casal (`FOR UPDATE`) dentro de uma transação.

**`internal/household/repository.go`**

```go
package household

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

const MaxMembers = 2

var (
	ErrNoHousehold        = errors.New("você ainda não tem um casal")
	ErrAlreadyInHousehold = errors.New("você já pertence a um casal")
	ErrInviteInvalid      = errors.New("convite inválido, expirado ou já usado")
	ErrHouseholdFull      = errors.New("o casal já tem duas pessoas")
)

type Member struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	JoinedAt time.Time `json:"joined_at"`
}

type Repository interface {
	CreateHousehold(ctx context.Context, userID string) (string, error)
	HouseholdOf(ctx context.Context, userID string) (string, error)
	Members(ctx context.Context, householdID string) ([]Member, error)
	IsMember(ctx context.Context, householdID, userID string) (bool, error)
	CreateInvite(ctx context.Context, householdID, createdBy, tokenHash string, expiresAt time.Time) error
	AcceptInvite(ctx context.Context, tokenHash, userID string, now time.Time) (string, error)
}

type PostgresRepo struct{ db *sql.DB }

func NewPostgresRepo(db *sql.DB) *PostgresRepo { return &PostgresRepo{db: db} }

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func (r *PostgresRepo) CreateHousehold(ctx context.Context, userID string) (string, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	var id string
	if err := tx.QueryRowContext(ctx, `INSERT INTO households DEFAULT VALUES RETURNING id`).Scan(&id); err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO household_members (household_id, user_id) VALUES ($1, $2)`, id, userID); err != nil {
		if isUniqueViolation(err) {
			return "", ErrAlreadyInHousehold
		}
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return id, nil
}

func (r *PostgresRepo) HouseholdOf(ctx context.Context, userID string) (string, error) {
	var id string
	err := r.db.QueryRowContext(ctx,
		`SELECT household_id FROM household_members WHERE user_id = $1`, userID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNoHousehold
	}
	return id, err
}

// Members faz um JOIN somente leitura em users, só para mostrar o nome.
func (r *PostgresRepo) Members(ctx context.Context, householdID string) ([]Member, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT u.id, u.name, m.joined_at
		FROM household_members m JOIN users u ON u.id = m.user_id
		WHERE m.household_id = $1
		ORDER BY m.joined_at`, householdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Member
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.ID, &m.Name, &m.JoinedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *PostgresRepo) IsMember(ctx context.Context, householdID, userID string) (bool, error) {
	var ok bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM household_members WHERE household_id = $1 AND user_id = $2)`,
		householdID, userID).Scan(&ok)
	return ok, err
}

func (r *PostgresRepo) CreateInvite(ctx context.Context, householdID, createdBy, tokenHash string, expiresAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO household_invites (household_id, token_hash, created_by, expires_at)
		VALUES ($1, $2, $3, $4)`, householdID, tokenHash, createdBy, expiresAt)
	return err
}

// AcceptInvite faz tudo em uma transação: valida o convite, trava o casal,
// confere a vaga e registra a entrada.
func (r *PostgresRepo) AcceptInvite(ctx context.Context, tokenHash, userID string, now time.Time) (string, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	var inviteID, householdID string
	var expiresAt time.Time
	var acceptedAt sql.NullTime
	err = tx.QueryRowContext(ctx, `
		SELECT id, household_id, expires_at, accepted_at
		FROM household_invites WHERE token_hash = $1 FOR UPDATE`, tokenHash).
		Scan(&inviteID, &householdID, &expiresAt, &acceptedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrInviteInvalid
	}
	if err != nil {
		return "", err
	}
	if acceptedAt.Valid || !expiresAt.After(now) {
		return "", ErrInviteInvalid
	}

	// A trava no casal impede que dois convites aceitos ao mesmo tempo passem de duas pessoas.
	if _, err := tx.ExecContext(ctx, `SELECT 1 FROM households WHERE id = $1 FOR UPDATE`, householdID); err != nil {
		return "", err
	}
	var count int
	if err := tx.QueryRowContext(ctx,
		`SELECT count(*) FROM household_members WHERE household_id = $1`, householdID).Scan(&count); err != nil {
		return "", err
	}
	if count >= MaxMembers {
		return "", ErrHouseholdFull
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO household_members (household_id, user_id) VALUES ($1, $2)`, householdID, userID); err != nil {
		if isUniqueViolation(err) {
			return "", ErrAlreadyInHousehold
		}
		return "", err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE household_invites SET accepted_by = $1, accepted_at = $2 WHERE id = $3`,
		userID, now, inviteID); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return householdID, nil
}
```

**`internal/household/service.go`**

```go
package household

import (
	"context"
	"time"

	"casal/internal/auth"
)

const inviteTTL = 7 * 24 * time.Hour

type Service struct {
	repo Repository
	now  func() time.Time
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo, now: time.Now}
}

func (s *Service) Create(ctx context.Context, userID string) (string, error) {
	return s.repo.CreateHousehold(ctx, userID)
}

func (s *Service) HouseholdOf(ctx context.Context, userID string) (string, error) {
	return s.repo.HouseholdOf(ctx, userID)
}

func (s *Service) Members(ctx context.Context, householdID string) ([]Member, error) {
	return s.repo.Members(ctx, householdID)
}

// AreMembers confere se todas as pessoas informadas pertencem ao casal.
// É a função pública que outras features usam, em vez de ler as tabelas daqui.
func (s *Service) AreMembers(ctx context.Context, householdID string, userIDs ...string) (bool, error) {
	for _, id := range userIDs {
		ok, err := s.repo.IsMember(ctx, householdID, id)
		if err != nil || !ok {
			return false, err
		}
	}
	return true, nil
}

// Invite gera um convite. Devolve o token bruto, que só existe neste momento.
func (s *Service) Invite(ctx context.Context, householdID, userID string) (string, time.Time, error) {
	members, err := s.repo.Members(ctx, householdID)
	if err != nil {
		return "", time.Time{}, err
	}
	if len(members) >= MaxMembers {
		return "", time.Time{}, ErrHouseholdFull
	}
	raw, hash, err := auth.NewToken()
	if err != nil {
		return "", time.Time{}, err
	}
	expires := s.now().Add(inviteTTL)
	if err := s.repo.CreateInvite(ctx, householdID, userID, hash, expires); err != nil {
		return "", time.Time{}, err
	}
	return raw, expires, nil
}

func (s *Service) Accept(ctx context.Context, userID, rawToken string) (string, error) {
	return s.repo.AcceptInvite(ctx, auth.HashToken(rawToken), userID, s.now())
}
```

**`internal/household/middleware.go`**

```go
package household

import (
	"context"
	"errors"
	"net/http"

	"casal/internal/auth"
	"casal/internal/httpx"
)

type ctxKey struct{}

func IDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(ctxKey{}).(string)
	return id, ok
}

// RequireHousehold deve vir depois de auth.Require. Descobre o casal da pessoa
// logada e o coloca no contexto: é daqui que vem o household_id de todas as consultas.
func RequireHousehold(svc *Service) httpx.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u, ok := auth.UserFromContext(r.Context())
			if !ok {
				httpx.Error(w, http.StatusUnauthorized, "faça login")
				return
			}
			id, err := svc.HouseholdOf(r.Context(), u.ID)
			if errors.Is(err, ErrNoHousehold) {
				httpx.Error(w, http.StatusForbidden, err.Error())
				return
			}
			if err != nil {
				httpx.Error(w, http.StatusInternalServerError, "erro interno")
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, id)))
		})
	}
}
```

**`internal/household/handler.go`**

```go
package household

import (
	"errors"
	"log/slog"
	"net/http"

	"casal/internal/auth"
	"casal/internal/httpx"
)

type Handler struct {
	svc *Service
	log *slog.Logger
}

func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// Routes recebe o middleware de login para não depender de como ele é construído.
func (h *Handler) Routes(mux *http.ServeMux, requireUser httpx.Middleware) {
	requireHH := RequireHousehold(h.svc)
	mux.Handle("POST /v1/household", httpx.Chain(http.HandlerFunc(h.create), requireUser))
	mux.Handle("GET /v1/household", httpx.Chain(http.HandlerFunc(h.get), requireUser, requireHH))
	mux.Handle("POST /v1/household/invites", httpx.Chain(http.HandlerFunc(h.invite), requireUser, requireHH))
	mux.Handle("POST /v1/household/invites/accept", httpx.Chain(http.HandlerFunc(h.accept), requireUser))
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFromContext(r.Context())
	id, err := h.svc.Create(r.Context(), u.ID)
	if err != nil {
		h.fail(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]string{"id": id})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, _ := IDFromContext(r.Context())
	members, err := h.svc.Members(r.Context(), id)
	if err != nil {
		h.fail(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"id": id, "members": members})
}

func (h *Handler) invite(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFromContext(r.Context())
	hid, _ := IDFromContext(r.Context())
	token, expires, err := h.svc.Invite(r.Context(), hid, u.ID)
	if err != nil {
		h.fail(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"token": token, "expires_at": expires})
}

func (h *Handler) accept(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Token string `json:"token"`
	}
	if err := httpx.Decode(w, r, &in); err != nil || in.Token == "" {
		httpx.Error(w, http.StatusBadRequest, "informe o token do convite")
		return
	}
	u, _ := auth.UserFromContext(r.Context())
	id, err := h.svc.Accept(r.Context(), u.ID, in.Token)
	if err != nil {
		h.fail(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"household_id": id})
}

func (h *Handler) fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInviteInvalid):
		httpx.Error(w, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrAlreadyInHousehold), errors.Is(err, ErrHouseholdFull):
		httpx.Error(w, http.StatusConflict, err.Error())
	case errors.Is(err, ErrNoHousehold):
		httpx.Error(w, http.StatusForbidden, err.Error())
	default:
		h.log.Error("erro em household", "err", err)
		httpx.Error(w, http.StatusInternalServerError, "erro interno")
	}
}
```

**Ligar no `main.go`** (logo abaixo da ligação do `auth`)

```go
requireUser := auth.Require(authSvc)
householdSvc := household.NewService(household.NewPostgresRepo(db))
household.NewHandler(householdSvc, log).Routes(mux, requireUser)
```

Acrescente `"casal/internal/household"` aos imports.

**Testar com dois usuários** (cada um com seu arquivo de cookies, para simular dois aparelhos)

```bash
H=http://localhost:8080
J='Content-Type: application/json'

# Ana cria conta e casal, e gera o convite
curl -s -c ana.txt -H "$J" -d '{"email":"ana@exemplo.com","name":"Ana","password":"senha-forte-123"}' $H/v1/auth/register
curl -s -b ana.txt -X POST $H/v1/household
curl -s -b ana.txt -X POST $H/v1/household/invites     # copie o "token" da resposta

# Bia cria conta e aceita o convite
curl -s -c bia.txt -H "$J" -d '{"email":"bia@exemplo.com","name":"Bia","password":"senha-forte-123"}' $H/v1/auth/register
curl -s -b bia.txt -H "$J" -d '{"token":"COLE-O-TOKEN-AQUI"}' $H/v1/household/invites/accept

# Os dois veem o mesmo casal
curl -s -b ana.txt $H/v1/household
curl -s -b bia.txt $H/v1/household
```

Tente também aceitar o mesmo convite de novo (deve dar `404`) e gerar um terceiro convite (deve dar `409`, porque o casal já está completo).

## Passo 4: lembretes e agendador

Depois deste passo, uma pessoa cria um lembrete para a outra, e um processo em segundo plano dispara os que já venceram, uma única vez cada.

**Ciclo de vida** (a coluna `status`)

1. `scheduled`: criado, esperando o horário.
2. `sent`: o agendador já avisou o destinatário.
3. `resolved`: o destinatário marcou como feito.
4. `canceled`: quem criou apagou.

Adiar volta o lembrete para `scheduled` com um novo horário.

**Como o agendador evita duplicidade**

Vários processos podem rodar ao mesmo tempo (por exemplo, duas instâncias do servidor). Cada um busca os vencidos com `FOR UPDATE SKIP LOCKED`: as linhas que um processo pegou ficam travadas e os outros as pulam, então cada lembrete é processado por um só. O aviso é enviado dentro da transação e o `status` só muda para `sent` no final. Se o servidor cair no meio, o lembrete volta a ser tentado (pode chegar duas vezes, mas não se perde).

**`internal/reminder/repository.go`**

```go
package reminder

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var (
	ErrInvalidInput = errors.New("dados inválidos")
	ErrNotFound     = errors.New("lembrete não encontrado")
)

type Reminder struct {
	ID            string     `json:"id"`
	HouseholdID   string     `json:"-"`
	CreatedBy     string     `json:"created_by"`
	CreatedByName string     `json:"created_by_name"`
	RecipientID   string     `json:"recipient_id"`
	Body          string     `json:"body"`
	RemindAt      time.Time  `json:"remind_at"`
	Status        string     `json:"status"`
	SentAt        *time.Time `json:"sent_at,omitempty"`
	ResolvedAt    *time.Time `json:"resolved_at,omitempty"`
}

type Repository interface {
	Create(ctx context.Context, r Reminder) (Reminder, error)
	List(ctx context.Context, householdID, userID string) ([]Reminder, error)
	Resolve(ctx context.Context, householdID, id, userID string, now time.Time) (Reminder, error)
	Snooze(ctx context.Context, householdID, id, userID string, remindAt time.Time) (Reminder, error)
	Cancel(ctx context.Context, householdID, id, userID string) error
	// ProcessDue pega até limit lembretes vencidos, chama fn para cada um e os marca como enviados.
	ProcessDue(ctx context.Context, now time.Time, limit int, fn func(context.Context, Reminder) error) (int, error)
}

type PostgresRepo struct{ db *sql.DB }

func NewPostgresRepo(db *sql.DB) *PostgresRepo { return &PostgresRepo{db: db} }

// Colunas lidas em todas as consultas. O JOIN em users é só leitura, para mostrar o nome.
const cols = `r.id, r.household_id, r.created_by, u.name, r.recipient_id, r.body,
	r.remind_at, r.status, r.sent_at, r.resolved_at`

type scanner interface{ Scan(dest ...any) error }

func scan(s scanner) (Reminder, error) {
	var r Reminder
	var sent, resolved sql.NullTime
	err := s.Scan(&r.ID, &r.HouseholdID, &r.CreatedBy, &r.CreatedByName, &r.RecipientID,
		&r.Body, &r.RemindAt, &r.Status, &sent, &resolved)
	if err != nil {
		return Reminder{}, err
	}
	if sent.Valid {
		r.SentAt = &sent.Time
	}
	if resolved.Valid {
		r.ResolvedAt = &resolved.Time
	}
	return r, nil
}

func (p *PostgresRepo) Create(ctx context.Context, in Reminder) (Reminder, error) {
	row := p.db.QueryRowContext(ctx, `
		WITH ins AS (
			INSERT INTO reminders (household_id, created_by, recipient_id, body, remind_at)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING *
		)
		SELECT `+cols+` FROM ins r JOIN users u ON u.id = r.created_by`,
		in.HouseholdID, in.CreatedBy, in.RecipientID, in.Body, in.RemindAt)
	return scan(row)
}

// List devolve os lembretes que a pessoa recebeu ou criou, os mais recentes primeiro.
func (p *PostgresRepo) List(ctx context.Context, householdID, userID string) ([]Reminder, error) {
	rows, err := p.db.QueryContext(ctx, `
		SELECT `+cols+`
		FROM reminders r JOIN users u ON u.id = r.created_by
		WHERE r.household_id = $1
		  AND (r.recipient_id = $2 OR r.created_by = $2)
		  AND r.status <> 'canceled'
		ORDER BY r.remind_at DESC
		LIMIT 100`, householdID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Reminder{}
	for rows.Next() {
		r, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Só o destinatário resolve. Note o household_id: ele isola um casal do outro.
func (p *PostgresRepo) Resolve(ctx context.Context, householdID, id, userID string, now time.Time) (Reminder, error) {
	row := p.db.QueryRowContext(ctx, `
		WITH upd AS (
			UPDATE reminders SET status = 'resolved', resolved_at = $4
			WHERE id = $1 AND household_id = $2 AND recipient_id = $3
			  AND status IN ('scheduled', 'sent')
			RETURNING *
		)
		SELECT `+cols+` FROM upd r JOIN users u ON u.id = r.created_by`,
		id, householdID, userID, now)
	r, err := scan(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Reminder{}, ErrNotFound
	}
	return r, err
}

func (p *PostgresRepo) Snooze(ctx context.Context, householdID, id, userID string, remindAt time.Time) (Reminder, error) {
	row := p.db.QueryRowContext(ctx, `
		WITH upd AS (
			UPDATE reminders SET status = 'scheduled', remind_at = $4, sent_at = NULL
			WHERE id = $1 AND household_id = $2 AND recipient_id = $3
			  AND status IN ('scheduled', 'sent')
			RETURNING *
		)
		SELECT `+cols+` FROM upd r JOIN users u ON u.id = r.created_by`,
		id, householdID, userID, remindAt)
	r, err := scan(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Reminder{}, ErrNotFound
	}
	return r, err
}

// Só quem criou cancela.
func (p *PostgresRepo) Cancel(ctx context.Context, householdID, id, userID string) error {
	res, err := p.db.ExecContext(ctx, `
		UPDATE reminders SET status = 'canceled'
		WHERE id = $1 AND household_id = $2 AND created_by = $3
		  AND status IN ('scheduled', 'sent')`, id, householdID, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *PostgresRepo) ProcessDue(ctx context.Context, now time.Time, limit int, fn func(context.Context, Reminder) error) (int, error) {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, `
		SELECT `+cols+`
		FROM reminders r JOIN users u ON u.id = r.created_by
		WHERE r.status = 'scheduled' AND r.remind_at <= $1
		ORDER BY r.remind_at
		LIMIT $2
		FOR UPDATE OF r SKIP LOCKED`, now, limit)
	if err != nil {
		return 0, err
	}
	var due []Reminder
	for rows.Next() {
		r, err := scan(rows)
		if err != nil {
			rows.Close()
			return 0, err
		}
		due = append(due, r)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close() // precisa fechar antes de usar a transação de novo

	for _, r := range due {
		if err := fn(ctx, r); err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE reminders SET status = 'sent', sent_at = $2 WHERE id = $1`, r.ID, now); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(due), nil
}
```

**`internal/reminder/service.go`**

```go
package reminder

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// Members é o que este pacote precisa da feature household.
type Members interface {
	AreMembers(ctx context.Context, householdID string, userIDs ...string) (bool, error)
}

type Service struct {
	repo    Repository
	members Members
	now     func() time.Time
}

func NewService(repo Repository, members Members) *Service {
	return &Service{repo: repo, members: members, now: time.Now}
}

func (s *Service) Create(ctx context.Context, householdID, creatorID, recipientID, body string, remindAt time.Time) (Reminder, error) {
	body = strings.TrimSpace(body)
	if body == "" || utf8.RuneCountInString(body) > 500 {
		return Reminder{}, fmt.Errorf("%w: o texto é obrigatório (até 500 caracteres)", ErrInvalidInput)
	}
	if remindAt.IsZero() {
		return Reminder{}, fmt.Errorf("%w: informe a data e a hora", ErrInvalidInput)
	}
	if remindAt.Before(s.now().Add(-time.Minute)) {
		return Reminder{}, fmt.Errorf("%w: o horário já passou", ErrInvalidInput)
	}
	if recipientID == "" {
		recipientID = creatorID
	}
	ok, err := s.members.AreMembers(ctx, householdID, creatorID, recipientID)
	if err != nil {
		return Reminder{}, err
	}
	if !ok {
		return Reminder{}, fmt.Errorf("%w: a pessoa não pertence ao casal", ErrInvalidInput)
	}
	return s.repo.Create(ctx, Reminder{
		HouseholdID: householdID,
		CreatedBy:   creatorID,
		RecipientID: recipientID,
		Body:        body,
		RemindAt:    remindAt.UTC(),
	})
}

func (s *Service) List(ctx context.Context, householdID, userID string) ([]Reminder, error) {
	return s.repo.List(ctx, householdID, userID)
}

func (s *Service) Resolve(ctx context.Context, householdID, id, userID string) (Reminder, error) {
	return s.repo.Resolve(ctx, householdID, id, userID, s.now())
}

func (s *Service) Snooze(ctx context.Context, householdID, id, userID string, minutes int) (Reminder, error) {
	if minutes < 1 || minutes > 24*60 {
		return Reminder{}, fmt.Errorf("%w: adie de 1 a 1440 minutos", ErrInvalidInput)
	}
	return s.repo.Snooze(ctx, householdID, id, userID, s.now().Add(time.Duration(minutes)*time.Minute))
}

func (s *Service) Cancel(ctx context.Context, householdID, id, userID string) error {
	return s.repo.Cancel(ctx, householdID, id, userID)
}
```

**`internal/reminder/handler.go`**

```go
package reminder

import (
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"casal/internal/auth"
	"casal/internal/household"
	"casal/internal/httpx"
)

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type Handler struct {
	svc *Service
	log *slog.Logger
}

func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

func (h *Handler) Routes(mux *http.ServeMux, requireUser, requireHousehold httpx.Middleware) {
	wrap := func(f http.HandlerFunc) http.Handler {
		return httpx.Chain(f, requireUser, requireHousehold)
	}
	mux.Handle("POST /v1/reminders", wrap(h.create))
	mux.Handle("GET /v1/reminders", wrap(h.list))
	mux.Handle("POST /v1/reminders/{id}/resolve", wrap(h.resolve))
	mux.Handle("POST /v1/reminders/{id}/snooze", wrap(h.snooze))
	mux.Handle("DELETE /v1/reminders/{id}", wrap(h.cancel))
}

// ids devolve o usuário e o casal da requisição (já validados pelos middlewares).
func ids(r *http.Request) (userID, householdID string) {
	u, _ := auth.UserFromContext(r.Context())
	hid, _ := household.IDFromContext(r.Context())
	return u.ID, hid
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		RecipientID string    `json:"recipient_id"`
		Body        string    `json:"body"`
		RemindAt    time.Time `json:"remind_at"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	uid, hid := ids(r)
	out, err := h.svc.Create(r.Context(), hid, uid, in.RecipientID, in.Body, in.RemindAt)
	if err != nil {
		h.fail(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, out)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	uid, hid := ids(r)
	out, err := h.svc.List(r.Context(), hid, uid)
	if err != nil {
		h.fail(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) resolve(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		httpx.Error(w, http.StatusNotFound, ErrNotFound.Error())
		return
	}
	uid, hid := ids(r)
	out, err := h.svc.Resolve(r.Context(), hid, id, uid)
	if err != nil {
		h.fail(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) snooze(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		httpx.Error(w, http.StatusNotFound, ErrNotFound.Error())
		return
	}
	var in struct {
		Minutes int `json:"minutes"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	uid, hid := ids(r)
	out, err := h.svc.Snooze(r.Context(), hid, id, uid, in.Minutes)
	if err != nil {
		h.fail(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) cancel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		httpx.Error(w, http.StatusNotFound, ErrNotFound.Error())
		return
	}
	uid, hid := ids(r)
	if err := h.svc.Cancel(r.Context(), hid, id, uid); err != nil {
		h.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		httpx.Error(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrNotFound):
		httpx.Error(w, http.StatusNotFound, err.Error())
	default:
		h.log.Error("erro em reminder", "err", err)
		httpx.Error(w, http.StatusInternalServerError, "erro interno")
	}
}
```

**`internal/reminder/worker.go`**

```go
package reminder

import (
	"context"
	"log/slog"
	"time"
)

// Notifier envia o aviso a todos os aparelhos de uma pessoa.
// A feature push implementa esta interface no passo 5.
type Notifier interface {
	Notify(ctx context.Context, userID, title, body string) error
}

type Worker struct {
	repo     Repository
	notifier Notifier
	log      *slog.Logger
	interval time.Duration
	batch    int
	now      func() time.Time
}

func NewWorker(repo Repository, notifier Notifier, log *slog.Logger) *Worker {
	return &Worker{repo: repo, notifier: notifier, log: log, interval: 15 * time.Second, batch: 50, now: time.Now}
}

// Run bloqueia até o contexto ser cancelado. Rode com: go worker.Run(ctx)
func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		w.tick(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w *Worker) tick(ctx context.Context) {
	for ctx.Err() == nil {
		n, err := w.repo.ProcessDue(ctx, w.now(), w.batch, w.send)
		if err != nil {
			w.log.Error("falha ao processar lembretes", "err", err)
			return
		}
		if n < w.batch {
			return // não há mais vencidos agora
		}
	}
}

// send nunca devolve erro de envio: uma falha de push não deve travar o lembrete
// (ele continua visível no app). Pendência: registrar tentativas e reenviar com espera.
func (w *Worker) send(ctx context.Context, r Reminder) error {
	title := "Lembrete de " + r.CreatedByName
	if r.CreatedBy == r.RecipientID {
		title = "Seu lembrete"
	}
	if err := w.notifier.Notify(ctx, r.RecipientID, title, r.Body); err != nil {
		w.log.Warn("aviso não enviado", "reminder", r.ID, "err", err)
	}
	return nil
}
```

**Ligar no `main.go`**, com um notificador de mentira que só escreve no log (o de verdade vem no passo 5):

```go
// ... depois de criar o householdSvc:
requireHousehold := household.RequireHousehold(householdSvc)

reminderRepo := reminder.NewPostgresRepo(db)
reminder.NewHandler(reminder.NewService(reminderRepo, householdSvc), log).
	Routes(mux, requireUser, requireHousehold)

go reminder.NewWorker(reminderRepo, logNotifier{log}, log).Run(ctx)
```

E, no fim do arquivo `main.go`, o notificador provisório:

```go
type logNotifier struct{ log *slog.Logger }

func (n logNotifier) Notify(_ context.Context, userID, title, body string) error {
	n.log.Info("notificação (simulada)", "user", userID, "title", title, "body", body)
	return nil
}
```

Acrescente `"casal/internal/reminder"` aos imports. O `ctx` é o mesmo do `signal.NotifyContext`, então o agendador para junto com o servidor.

**Testar** (continue com os arquivos de cookies `ana.txt` e `bia.txt` do passo anterior)

```bash
# Linux: um minuto à frente. No macOS: date -u -v+1M +%Y-%m-%dT%H:%M:%SZ
QUANDO=$(date -u -d '+1 minute' +%Y-%m-%dT%H:%M:%SZ)
BIA_ID=<cole o id da Bia, de GET /v1/household>

curl -s -b ana.txt -H "$J" -d "{\"recipient_id\":\"$BIA_ID\",\"body\":\"tirar a marmita do freezer\",\"remind_at\":\"$QUANDO\"}" $H/v1/reminders
curl -s -b bia.txt $H/v1/reminders
```

Espere até dois minutos: o log do servidor deve mostrar `notificação (simulada)`, e o `status` do lembrete passa a `sent`. Depois:

```bash
curl -s -b bia.txt -X POST $H/v1/reminders/ID-DO-LEMBRETE/resolve
```

Confira que a Ana **não** consegue resolver o lembrete da Bia (`404`), e que um terceiro usuário, em outro casal, não vê nada em `GET /v1/reminders`.

## Passo 5: Web Push

Este é o passo que decide se o produto funciona como PWA: ao final, o celular de cada um recebe o aviso mesmo com o app fechado, inclusive no iPhone.

**Como o Web Push funciona**

1. O servidor tem um par de chaves **VAPID**. A chave pública vai para o navegador, a privada fica só no servidor.
2. O navegador (com a permissão da pessoa) cria uma **assinatura**: um endereço no serviço de push da Apple, do Google ou da Mozilla, mais duas chaves de criptografia.
3. O app envia essa assinatura ao servidor, que a guarda na tabela `push_subscriptions`.
4. Para avisar, o servidor envia a mensagem criptografada a esse endereço, e o sistema do celular acorda o *service worker*, que mostra a notificação.

**Regras do iPhone** (as mais importantes do projeto)

- Precisa de iOS 16.4 ou mais novo.
- O app precisa estar **instalado na tela inicial** (Compartilhar > Adicionar à Tela de Início). No Safari comum, o push não funciona.
- A permissão só pode ser pedida depois de um toque da pessoa (por isso o botão "Ativar avisos").
- Toda mensagem recebida **deve mostrar uma notificação**. Se o service worker não mostrar nada, o iOS cancela a assinatura depois de algumas ocorrências.
- Exige HTTPS. Para testar no celular, use um endereço público com HTTPS (veja "Testar no celular" abaixo).

**Dependência**

```bash
go get github.com/SherClockHolmes/webpush-go@latest
```

**Gerar as chaves VAPID** (`cmd/vapid/main.go`)

```go
package main

import (
	"fmt"
	"log"

	webpush "github.com/SherClockHolmes/webpush-go"
)

func main() {
	priv, pub, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("VAPID_PUBLIC_KEY=%s\nVAPID_PRIVATE_KEY=%s\n", pub, priv)
}
```

Rode `go run ./cmd/vapid` e cole as duas linhas no `.env`. **Gere as chaves uma vez e não troque:** quem já assinou com a chave antiga deixa de receber.

**`internal/push/push.go`**

```go
package push

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
)

type Subscription struct {
	Endpoint string
	P256dh   string
	Auth     string
}

// Sender isola a biblioteca de Web Push. Em testes, use um fake.
type Sender interface {
	Send(ctx context.Context, sub Subscription, payload []byte) (status int, err error)
}

type Repository interface {
	Save(ctx context.Context, userID string, sub Subscription, userAgent string) error
	Delete(ctx context.Context, userID, endpoint string) error
	DeleteEndpoint(ctx context.Context, endpoint string) error
	ByUser(ctx context.Context, userID string) ([]Subscription, error)
}

type Service struct {
	repo   Repository
	sender Sender
	log    *slog.Logger
}

func NewService(repo Repository, sender Sender, log *slog.Logger) *Service {
	return &Service{repo: repo, sender: sender, log: log}
}

// Notify avisa todos os aparelhos da pessoa. Implementa reminder.Notifier.
func (s *Service) Notify(ctx context.Context, userID, title, body string) error {
	subs, err := s.repo.ByUser(ctx, userID)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]string{"title": title, "body": body, "url": "/"})
	if err != nil {
		return err
	}

	var firstErr error
	for _, sub := range subs {
		status, err := s.sender.Send(ctx, sub, payload)
		switch {
		case status == 404 || status == 410:
			// o aparelho cancelou ou a assinatura expirou: apague para não tentar de novo
			_ = s.repo.DeleteEndpoint(ctx, sub.Endpoint)
		case err != nil:
			if firstErr == nil {
				firstErr = err
			}
		case status >= 400:
			if firstErr == nil {
				firstErr = fmt.Errorf("serviço de push respondeu %d", status)
			}
		}
	}
	return firstErr
}

// Os endereços de push vêm do navegador, então o servidor só aceita os dos serviços
// conhecidos. Sem isto, alguém poderia fazer o servidor chamar qualquer endereço (SSRF).
var allowedHosts = []string{
	"fcm.googleapis.com",
	"push.services.mozilla.com",
	"web.push.apple.com",
	"notify.windows.com",
}

func validEndpoint(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	for _, h := range allowedHosts {
		if host == h || strings.HasSuffix(host, "."+h) {
			return true
		}
	}
	return false
}
```

**`internal/push/webpush.go`** (o único arquivo que conhece a biblioteca)

```go
package push

import (
	"context"

	webpush "github.com/SherClockHolmes/webpush-go"
)

type WebPushSender struct {
	subject, publicKey, privateKey string
}

func NewWebPushSender(subject, publicKey, privateKey string) *WebPushSender {
	return &WebPushSender{subject: subject, publicKey: publicKey, privateKey: privateKey}
}

func (w *WebPushSender) Send(ctx context.Context, sub Subscription, payload []byte) (int, error) {
	resp, err := webpush.SendNotificationWithContext(ctx, payload, &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys:     webpush.Keys{Auth: sub.Auth, P256dh: sub.P256dh},
	}, &webpush.Options{
		Subscriber:      w.subject,
		VAPIDPublicKey:  w.publicKey,
		VAPIDPrivateKey: w.privateKey,
		TTL:             3600,
		Urgency:         webpush.UrgencyHigh,
	})
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}
```

Se o compilador reclamar de algum nome, confira a versão instalada com `go doc github.com/SherClockHolmes/webpush-go`. Eu não consegui compilar este arquivo onde escrevi o guia.

**`internal/push/repository.go`**

```go
package push

import (
	"context"
	"database/sql"
)

type PostgresRepo struct{ db *sql.DB }

func NewPostgresRepo(db *sql.DB) *PostgresRepo { return &PostgresRepo{db: db} }

// Save grava ou atualiza a assinatura. Se o mesmo aparelho trocar de conta,
// o endereço passa a pertencer à nova pessoa.
func (r *PostgresRepo) Save(ctx context.Context, userID string, sub Subscription, userAgent string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO push_subscriptions (user_id, endpoint, p256dh, auth, user_agent)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (endpoint) DO UPDATE
		SET user_id = EXCLUDED.user_id, p256dh = EXCLUDED.p256dh,
		    auth = EXCLUDED.auth, user_agent = EXCLUDED.user_agent`,
		userID, sub.Endpoint, sub.P256dh, sub.Auth, userAgent)
	return err
}

func (r *PostgresRepo) Delete(ctx context.Context, userID, endpoint string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM push_subscriptions WHERE user_id = $1 AND endpoint = $2`, userID, endpoint)
	return err
}

func (r *PostgresRepo) DeleteEndpoint(ctx context.Context, endpoint string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM push_subscriptions WHERE endpoint = $1`, endpoint)
	return err
}

func (r *PostgresRepo) ByUser(ctx context.Context, userID string) ([]Subscription, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT endpoint, p256dh, auth FROM push_subscriptions WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Subscription
	for rows.Next() {
		var s Subscription
		if err := rows.Scan(&s.Endpoint, &s.P256dh, &s.Auth); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
```

**`internal/push/handler.go`**

```go
package push

import (
	"log/slog"
	"net/http"

	"casal/internal/auth"
	"casal/internal/httpx"
)

type Handler struct {
	svc         *Service
	vapidPublic string
	log         *slog.Logger
}

func NewHandler(svc *Service, vapidPublic string, log *slog.Logger) *Handler {
	return &Handler{svc: svc, vapidPublic: vapidPublic, log: log}
}

func (h *Handler) Routes(mux *http.ServeMux, requireUser httpx.Middleware) {
	mux.HandleFunc("GET /v1/push/vapid-key", h.vapidKey)
	mux.Handle("POST /v1/push/subscriptions", httpx.Chain(http.HandlerFunc(h.subscribe), requireUser))
	mux.Handle("DELETE /v1/push/subscriptions", httpx.Chain(http.HandlerFunc(h.unsubscribe), requireUser))
	mux.Handle("POST /v1/push/test", httpx.Chain(http.HandlerFunc(h.test), requireUser))
}

func (h *Handler) vapidKey(w http.ResponseWriter, r *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]string{"public_key": h.vapidPublic})
}

// O corpo é o que o navegador devolve em subscription.toJSON().
func (h *Handler) subscribe(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Endpoint       string `json:"endpoint"`
		ExpirationTime any    `json:"expirationTime"`
		Keys           struct {
			P256dh string `json:"p256dh"`
			Auth   string `json:"auth"`
		} `json:"keys"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	if !validEndpoint(in.Endpoint) || in.Keys.P256dh == "" || in.Keys.Auth == "" {
		httpx.Error(w, http.StatusBadRequest, "assinatura inválida")
		return
	}
	u, _ := auth.UserFromContext(r.Context())
	sub := Subscription{Endpoint: in.Endpoint, P256dh: in.Keys.P256dh, Auth: in.Keys.Auth}
	if err := h.svc.repo.Save(r.Context(), u.ID, sub, r.UserAgent()); err != nil {
		h.log.Error("salvar assinatura", "err", err)
		httpx.Error(w, http.StatusInternalServerError, "erro interno")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) unsubscribe(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Endpoint string `json:"endpoint"`
	}
	if err := httpx.Decode(w, r, &in); err != nil || in.Endpoint == "" {
		httpx.Error(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	u, _ := auth.UserFromContext(r.Context())
	if err := h.svc.repo.Delete(r.Context(), u.ID, in.Endpoint); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro interno")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// test manda um aviso para os aparelhos da própria pessoa. Útil para validar o push.
func (h *Handler) test(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFromContext(r.Context())
	if err := h.svc.Notify(r.Context(), u.ID, "Teste", "Funcionou! Este aviso veio do servidor."); err != nil {
		h.log.Warn("teste de push falhou", "err", err)
		httpx.Error(w, http.StatusBadGateway, "não consegui enviar o aviso")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

**Ligar no `main.go`.** Troque a linha `go reminder.NewWorker(..., logNotifier{log}, ...)` do passo anterior por:

```go
var notifier reminder.Notifier = logNotifier{log}
if cfg.VAPIDPublicKey != "" && cfg.VAPIDPrivateKey != "" {
	pushSvc := push.NewService(
		push.NewPostgresRepo(db),
		push.NewWebPushSender(cfg.VAPIDSubject, cfg.VAPIDPublicKey, cfg.VAPIDPrivateKey),
		log,
	)
	push.NewHandler(pushSvc, cfg.VAPIDPublicKey, log).Routes(mux, requireUser)
	notifier = pushSvc
} else {
	log.Warn("VAPID não configurado: os avisos só aparecerão no log")
}
go reminder.NewWorker(reminderRepo, notifier, log).Run(ctx)

// página de teste do PWA (pasta web/)
mux.Handle("GET /", http.FileServer(http.Dir("web")))
```

Acrescente `"casal/internal/push"` aos imports.

**Página mínima de teste** (pasta `web/`)

`web/index.html`

```html
<!doctype html>
<html lang="pt-BR">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <link rel="manifest" href="/manifest.webmanifest">
  <link rel="apple-touch-icon" href="/icon-192.png">
  <title>Casal: teste de push</title>
</head>
<body>
  <h1>Teste de push</h1>
  <form id="login">
    <input id="email" type="email" placeholder="e-mail" required>
    <input id="password" type="password" placeholder="senha" required>
    <button>Entrar</button>
  </form>
  <p><button id="enable">Ativar avisos</button></p>
  <p><button id="test">Enviar teste para mim</button></p>
  <pre id="log"></pre>
  <script src="/app.js"></script>
</body>
</html>
```

`web/app.js`

```js
const out = (m) => { document.getElementById('log').textContent += m + '\n'; };
const api = (path, opts = {}) =>
  fetch(path, { credentials: 'same-origin', headers: { 'Content-Type': 'application/json' }, ...opts });

function urlBase64ToUint8Array(base64) {
  const pad = '='.repeat((4 - (base64.length % 4)) % 4);
  const raw = atob((base64 + pad).replace(/-/g, '+').replace(/_/g, '/'));
  return Uint8Array.from([...raw].map((c) => c.charCodeAt(0)));
}

document.getElementById('login').addEventListener('submit', async (e) => {
  e.preventDefault();
  const res = await api('/v1/auth/login', {
    method: 'POST',
    body: JSON.stringify({
      email: document.getElementById('email').value,
      password: document.getElementById('password').value,
    }),
  });
  out('login: ' + res.status);
});

document.getElementById('enable').addEventListener('click', async () => {
  if (!('serviceWorker' in navigator) || !('PushManager' in window)) {
    out('Sem suporte a push. No iPhone, instale o app na tela inicial.');
    return;
  }
  const reg = await navigator.serviceWorker.register('/sw.js');
  await navigator.serviceWorker.ready;
  if ((await Notification.requestPermission()) !== 'granted') {
    out('permissão negada');
    return;
  }
  const { public_key } = await (await api('/v1/push/vapid-key')).json();
  const sub = await reg.pushManager.subscribe({
    userVisibleOnly: true,
    applicationServerKey: urlBase64ToUint8Array(public_key),
  });
  const res = await api('/v1/push/subscriptions', { method: 'POST', body: JSON.stringify(sub.toJSON()) });
  out('assinatura: ' + res.status);
});

document.getElementById('test').addEventListener('click', async () => {
  const res = await api('/v1/push/test', { method: 'POST' });
  out('teste: ' + res.status);
});
```

`web/sw.js`

```js
self.addEventListener('push', (event) => {
  let data = { title: 'Casal', body: '', url: '/' };
  try { data = { ...data, ...event.data.json() }; } catch (_) {}
  // no iOS toda mensagem precisa mostrar uma notificação
  event.waitUntil(
    self.registration.showNotification(data.title, { body: data.body, data: { url: data.url } })
  );
});

self.addEventListener('notificationclick', (event) => {
  event.notification.close();
  event.waitUntil(clients.openWindow(event.notification.data.url || '/'));
});
```

`web/manifest.webmanifest`

```json
{
  "name": "Casal",
  "short_name": "Casal",
  "start_url": "/",
  "display": "standalone",
  "background_color": "#ffffff",
  "theme_color": "#ffffff",
  "icons": [
    { "src": "/icon-192.png", "sizes": "192x192", "type": "image/png" },
    { "src": "/icon-512.png", "sizes": "512x512", "type": "image/png" }
  ]
}
```

Crie dois ícones PNG quadrados (`web/icon-192.png` e `web/icon-512.png`); qualquer imagem serve por enquanto.

**Testar no computador**

1. `make run`, abra `http://localhost:8080`, entre com a conta da Ana, clique em **Ativar avisos** e permita.
2. Clique em **Enviar teste para mim**: a notificação deve aparecer no sistema.
3. Crie um lembrete de um minuto à frente (passo 4) e aguarde.

**Testar no celular** (o ponto que mais importa)

1. Exponha o servidor com HTTPS. O jeito mais rápido é um túnel: `cloudflared tunnel --url http://localhost:8080` (ou `ngrok http 8080`), que mostra um endereço `https://...`.
2. Ponha `COOKIE_SECURE=true` no `.env` e reinicie o servidor.
3. No celular, abra o endereço do túnel. **No iPhone:** Safari > Compartilhar > Adicionar à Tela de Início, e abra o app pelo ícone novo.
4. Entre, toque em **Ativar avisos**, aceite a permissão e toque em **Enviar teste para mim**.
5. Bloqueie a tela e repita o teste, depois com o app totalmente fechado.

Se isso funcionar nos dois celulares (o seu e o da sua esposa) por alguns dias, a Fase 0 está cumprida.

## Isolamento entre casais

Um casal nunca pode ver, alterar ou apagar dados de outro, e isso precisa ser garantido em duas camadas: no código e no banco.

**Camada 1: o código (já feita nos passos anteriores)**

1. O `household_id` **vem da sessão**, via `RequireHousehold`, e nunca do corpo ou da URL da requisição.
2. Toda consulta de uma feature do casal tem `household_id = $x` no `WHERE`, inclusive `UPDATE` e `DELETE` (veja o `Resolve` dos lembretes).
3. Ao usar o identificador de outra pessoa (como `recipient_id`), o service confere se ela pertence ao casal (`AreMembers`).
4. Quando o registro não é do casal, a resposta é `404`, igual a "não existe", para não revelar que ele existe.

Regra de revisão para toda feature nova: **se uma consulta não tem `household_id`, ela precisa de justificativa.**

**Camada 2: o banco (RLS), recomendada antes de convidar outros casais**

A *Row Level Security* do Postgres faz o banco recusar linhas de outro casal mesmo que alguém esqueça o `WHERE`. Funciona assim: cada requisição abre uma transação e informa o casal com `set_config('app.household_id', ...)`; as políticas só deixam passar linhas daquele casal.

Há três pontos de atenção:

- O dono das tabelas ignora o RLS. O app precisa conectar com um **papel separado** (`casal_app`), e as migrações rodam com o dono.
- O agendador de lembretes atravessa todos os casais, então ele usa um modo "sistema" (`app.system = 'on'`) que as políticas aceitam.
- Isso exige que os repositórios usem uma transação com o casal definido, o que muda um pouco o código deles.

**Criar o papel do app** (uma vez, com o dono do banco; troque a senha)

```bash
make db-psql
# dentro do psql:
CREATE ROLE casal_app LOGIN PASSWORD 'outra-senha-forte';
GRANT CONNECT ON DATABASE casal TO casal_app;
GRANT USAGE ON SCHEMA public TO casal_app;
```

**`migrations/00002_rls.sql`**

```sql
-- +goose Up
-- +goose StatementBegin

GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO casal_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO casal_app;

ALTER TABLE reminders ENABLE ROW LEVEL SECURITY;

CREATE POLICY reminders_tenant ON reminders
    USING (
        household_id = nullif(current_setting('app.household_id', true), '')::uuid
        OR current_setting('app.system', true) = 'on'
    )
    WITH CHECK (
        household_id = nullif(current_setting('app.household_id', true), '')::uuid
        OR current_setting('app.system', true) = 'on'
    );

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP POLICY IF EXISTS reminders_tenant ON reminders;
ALTER TABLE reminders DISABLE ROW LEVEL SECURITY;
-- +goose StatementEnd
```

Cada tabela nova que tiver `household_id` (cardápio, freezer, gastos) ganha sua própria política igual a esta, na mesma migração em que for criada.

**`internal/db/tx.go`** (a transação com o casal definido)

```go
package db

import (
	"context"
	"database/sql"
)

type Scope struct {
	HouseholdID string // casal da requisição
	System      bool   // só para processos internos, como o agendador
}

// InTx abre uma transação, informa o escopo ao Postgres (vale só dentro dela) e roda fn.
func InTx(ctx context.Context, db *sql.DB, scope Scope, fn func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.household_id', $1, true)`, scope.HouseholdID); err != nil {
		return err
	}
	system := "off"
	if scope.System {
		system = "on"
	}
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.system', $1, true)`, system); err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
```

**Como um repositório passa a usar** (exemplo com `Create` dos lembretes)

```go
func (p *PostgresRepo) Create(ctx context.Context, in Reminder) (Reminder, error) {
	var out Reminder
	err := db.InTx(ctx, p.db, db.Scope{HouseholdID: in.HouseholdID}, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `
			WITH ins AS (
				INSERT INTO reminders (household_id, created_by, recipient_id, body, remind_at)
				VALUES ($1, $2, $3, $4, $5)
				RETURNING *
			)
			SELECT `+cols+` FROM ins r JOIN users u ON u.id = r.created_by`,
			in.HouseholdID, in.CreatedBy, in.RecipientID, in.Body, in.RemindAt)
		var err error
		out, err = scan(row)
		return err
	})
	return out, err
}
```

Faça igual com `List`, `Resolve`, `Snooze` e `Cancel`. No `ProcessDue`, use `db.Scope{System: true}`. Os métodos de leitura que hoje usam `p.db.QueryContext` passam a abrir a transação do mesmo jeito. Importe o pacote como `"casal/internal/db"` nos repositórios. No `main.go` não importe esse pacote: lá existe uma variável local chamada `db` (a conexão), que esconderia o nome do pacote.

**Duas variáveis de conexão no `.env`**

```bash
# o app usa o papel limitado
DATABASE_URL=postgres://casal_app:outra-senha-forte@localhost:5432/casal?sslmode=disable
# as migrações usam o dono
MIGRATE_DATABASE_URL=postgres://casal:troque-esta-senha@localhost:5432/casal?sslmode=disable
```

No `Makefile`, troque `$(DATABASE_URL)` por `$(MIGRATE_DATABASE_URL)` nos comandos do `goose`.

**Provar que funciona:** o teste de isolamento da próxima seção deve passar **com** o RLS ligado e também **sem** o `WHERE household_id` em uma consulta de propósito, para você ver o banco barrar.

## Testes

Há dois tipos de teste: os **unitários**, que rodam sem banco usando repositórios falsos, e os de **integração**, que rodam no Postgres e provam as garantias que só o banco dá (isolamento e limite de duas pessoas).

**Rodar**

```bash
make test                # unitários; os de integração são pulados
make test-integration    # todos, com o banco de teste
```

Acrescente ao `Makefile`:

```makefile
TEST_DB=casal_test
TEST_URL=postgres://$(DB_USER):$(DB_PASSWORD)@localhost:5432/$(TEST_DB)?sslmode=disable

test-db:
	-docker compose exec db createdb -U $(DB_USER) $(TEST_DB)
	goose -dir $(MIGRATIONS) postgres "$(TEST_URL)" up

test-integration: test-db
	TEST_DATABASE_URL="$(TEST_URL)" go test ./... -count=1
```

O teste de integração usa o dono do banco (`$(DB_USER)`), que ignora o RLS. Para testar com o RLS ligado, aponte `TEST_DATABASE_URL` para o papel `casal_app`.

**1. Senhas** (`internal/auth/password_test.go`)

```go
package auth

import "testing"

func TestPasswordHashAndVerify(t *testing.T) {
	hash, err := HashPassword("senha-forte-123")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword("senha-forte-123", hash) {
		t.Error("a senha correta deveria passar")
	}
	if VerifyPassword("outra-senha", hash) {
		t.Error("a senha errada não deveria passar")
	}
	if VerifyPassword("senha-forte-123", "lixo") {
		t.Error("um hash malformado não deveria passar")
	}

	otherHash, _ := HashPassword("senha-forte-123")
	if hash == otherHash {
		t.Error("o mesmo texto deve gerar hashes diferentes (salt aleatório)")
	}
}
```

**2. Contas e sessões** (`internal/auth/service_test.go`)

```go
package auth

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

type fakeRepo struct {
	users    map[string]User // por e-mail
	hashes   map[string]string
	sessions map[string]fakeSession // por hash do token
	nextID   int
}

type fakeSession struct {
	userID  string
	expires time.Time
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{users: map[string]User{}, hashes: map[string]string{}, sessions: map[string]fakeSession{}}
}

func (f *fakeRepo) CreateUser(_ context.Context, email, name, hash string) (User, error) {
	if _, ok := f.users[email]; ok {
		return User{}, ErrEmailTaken
	}
	f.nextID++
	u := User{ID: fmt.Sprintf("u%d", f.nextID), Email: email, Name: name, Timezone: "America/Sao_Paulo"}
	f.users[email] = u
	f.hashes[email] = hash
	return u, nil
}

func (f *fakeRepo) UserByEmail(_ context.Context, email string) (User, string, error) {
	u, ok := f.users[email]
	if !ok {
		return User{}, "", ErrNotFound
	}
	return u, f.hashes[email], nil
}

func (f *fakeRepo) CreateSession(_ context.Context, userID, tokenHash string, expiresAt time.Time) error {
	f.sessions[tokenHash] = fakeSession{userID: userID, expires: expiresAt}
	return nil
}

func (f *fakeRepo) UserBySession(_ context.Context, tokenHash string, now time.Time) (User, error) {
	s, ok := f.sessions[tokenHash]
	if !ok || !s.expires.After(now) {
		return User{}, ErrNotFound
	}
	for _, u := range f.users {
		if u.ID == s.userID {
			return u, nil
		}
	}
	return User{}, ErrNotFound
}

func (f *fakeRepo) DeleteSession(_ context.Context, tokenHash string) error {
	delete(f.sessions, tokenHash)
	return nil
}

func newTestService(t *testing.T) *Service {
	t.Helper()
	svc, err := NewService(newFakeRepo(), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func TestRegisterAndLogin(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	u, token, err := svc.Register(ctx, "  Ana@Exemplo.com ", "Ana", "senha-forte-123")
	if err != nil {
		t.Fatal(err)
	}
	if u.Email != "ana@exemplo.com" {
		t.Errorf("o e-mail deveria ser normalizado, veio %q", u.Email)
	}
	got, err := svc.UserFromToken(ctx, token)
	if err != nil || got.ID != u.ID {
		t.Fatalf("a sessão do cadastro deveria valer: %v", err)
	}

	if _, _, err := svc.Login(ctx, "ana@exemplo.com", "senha-forte-123"); err != nil {
		t.Errorf("login correto falhou: %v", err)
	}
	if _, _, err := svc.Login(ctx, "ana@exemplo.com", "errada"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("senha errada deveria dar ErrInvalidCredentials, deu %v", err)
	}
	if _, _, err := svc.Login(ctx, "ninguem@exemplo.com", "qualquer"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("e-mail inexistente deveria dar o mesmo erro, deu %v", err)
	}
}

func TestRegisterValidation(t *testing.T) {
	svc := newTestService(t)
	cases := []struct{ name, email, user, pass string }{
		{"e-mail inválido", "nao-e-email", "Ana", "senha-forte-123"},
		{"e-mail com nome", "Ana <ana@exemplo.com>", "Ana", "senha-forte-123"},
		{"nome vazio", "ana@exemplo.com", "  ", "senha-forte-123"},
		{"senha curta", "ana@exemplo.com", "Ana", "1234567"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, err := svc.Register(context.Background(), c.email, c.user, c.pass)
			if !errors.Is(err, ErrInvalidInput) {
				t.Errorf("esperava ErrInvalidInput, veio %v", err)
			}
		})
	}
}

func TestRegisterDuplicateEmail(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)
	if _, _, err := svc.Register(ctx, "ana@exemplo.com", "Ana", "senha-forte-123"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Register(ctx, "ANA@exemplo.com", "Outra", "senha-forte-123"); !errors.Is(err, ErrEmailTaken) {
		t.Errorf("esperava ErrEmailTaken, veio %v", err)
	}
}

func TestLogoutEndsSession(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)
	_, token, err := svc.Register(ctx, "ana@exemplo.com", "Ana", "senha-forte-123")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Logout(ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UserFromToken(ctx, token); err == nil {
		t.Error("a sessão deveria ter sido encerrada")
	}
}

func TestExpiredSession(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)
	_, token, err := svc.Register(ctx, "ana@exemplo.com", "Ana", "senha-forte-123")
	if err != nil {
		t.Fatal(err)
	}
	svc.now = func() time.Time { return time.Now().Add(2 * time.Hour) } // além do TTL de 1h
	if _, err := svc.UserFromToken(ctx, token); err == nil {
		t.Error("uma sessão expirada não deveria valer")
	}
}
```

**3. Convite** (`internal/household/service_test.go`)

```go
package household

import (
	"context"
	"errors"
	"testing"
	"time"

	"casal/internal/auth"
)

// Embute a interface: só implementamos o que o teste usa. Chamar outro método causa panic.
type fakeRepo struct {
	Repository
	members  []Member
	invites  map[string]time.Time // hash do token -> validade
	inMember map[string]bool
}

func (f *fakeRepo) Members(context.Context, string) ([]Member, error) { return f.members, nil }

func (f *fakeRepo) CreateInvite(_ context.Context, _, _, tokenHash string, expiresAt time.Time) error {
	f.invites[tokenHash] = expiresAt
	return nil
}

func (f *fakeRepo) IsMember(_ context.Context, _, userID string) (bool, error) {
	return f.inMember[userID], nil
}

func TestInviteStoresOnlyTheHash(t *testing.T) {
	repo := &fakeRepo{members: []Member{{ID: "u1"}}, invites: map[string]time.Time{}}
	svc := NewService(repo)

	raw, expires, err := svc.Invite(context.Background(), "h1", "u1")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := repo.invites[raw]; ok {
		t.Error("o token bruto não pode ser guardado")
	}
	if _, ok := repo.invites[auth.HashToken(raw)]; !ok {
		t.Error("o hash do token deveria ter sido guardado")
	}
	if !expires.After(time.Now().Add(6 * 24 * time.Hour)) {
		t.Error("o convite deveria valer cerca de 7 dias")
	}
}

func TestInviteRefusedWhenHouseholdIsFull(t *testing.T) {
	repo := &fakeRepo{members: []Member{{ID: "u1"}, {ID: "u2"}}, invites: map[string]time.Time{}}
	_, _, err := NewService(repo).Invite(context.Background(), "h1", "u1")
	if !errors.Is(err, ErrHouseholdFull) {
		t.Errorf("esperava ErrHouseholdFull, veio %v", err)
	}
}

func TestAreMembers(t *testing.T) {
	repo := &fakeRepo{inMember: map[string]bool{"u1": true, "u2": true}}
	svc := NewService(repo)

	if ok, _ := svc.AreMembers(context.Background(), "h1", "u1", "u2"); !ok {
		t.Error("os dois são membros")
	}
	if ok, _ := svc.AreMembers(context.Background(), "h1", "u1", "u9"); ok {
		t.Error("u9 não é membro")
	}
}
```

**4. Lembretes** (`internal/reminder/service_test.go`)

```go
package reminder

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeRepo struct {
	Repository
	created []Reminder
}

func (f *fakeRepo) Create(_ context.Context, r Reminder) (Reminder, error) {
	r.ID = "r1"
	r.Status = "scheduled"
	f.created = append(f.created, r)
	return r, nil
}

type fakeMembers struct{ ok bool }

func (f fakeMembers) AreMembers(context.Context, string, ...string) (bool, error) { return f.ok, nil }

var fixedNow = time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)

func newTestService(membersOK bool) (*Service, *fakeRepo) {
	repo := &fakeRepo{}
	svc := NewService(repo, fakeMembers{ok: membersOK})
	svc.now = func() time.Time { return fixedNow }
	return svc, repo
}

func TestCreateDefaultsRecipientToCreator(t *testing.T) {
	svc, repo := newTestService(true)
	r, err := svc.Create(context.Background(), "h1", "u1", "", "  comprar pão ", fixedNow.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if r.RecipientID != "u1" || r.Body != "comprar pão" || len(repo.created) != 1 {
		t.Errorf("resultado inesperado: %+v", r)
	}
}

func TestCreateValidation(t *testing.T) {
	cases := []struct {
		name      string
		membersOK bool
		body      string
		at        time.Time
	}{
		{"texto vazio", true, "   ", fixedNow.Add(time.Hour)},
		{"texto longo demais", true, strings.Repeat("a", 501), fixedNow.Add(time.Hour)},
		{"sem horário", true, "ok", time.Time{}},
		{"horário no passado", true, "ok", fixedNow.Add(-time.Hour)},
		{"destinatário de outro casal", false, "ok", fixedNow.Add(time.Hour)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, repo := newTestService(c.membersOK)
			_, err := svc.Create(context.Background(), "h1", "u1", "u2", c.body, c.at)
			if !errors.Is(err, ErrInvalidInput) {
				t.Errorf("esperava ErrInvalidInput, veio %v", err)
			}
			if len(repo.created) != 0 {
				t.Error("nada deveria ter sido gravado")
			}
		})
	}
}

func TestCreateAllowsAFewSecondsInThePast(t *testing.T) {
	svc, _ := newTestService(true)
	if _, err := svc.Create(context.Background(), "h1", "u1", "u1", "agora", fixedNow.Add(-30*time.Second)); err != nil {
		t.Errorf("um lembrete para \"agora\" deveria ser aceito: %v", err)
	}
}

func TestSnoozeLimits(t *testing.T) {
	svc, _ := newTestService(true)
	for _, minutes := range []int{0, -5, 1441} {
		if _, err := svc.Snooze(context.Background(), "h1", "r1", "u1", minutes); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%d minutos deveria ser inválido, veio %v", minutes, err)
		}
	}
}
```

**5. Integração: base de testes** (`internal/testutil/db.go`)

```go
package testutil

import (
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// OpenDB conecta no banco de teste ou pula o teste se TEST_DATABASE_URL não estiver definida.
func OpenDB(t *testing.T) *sql.DB {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("defina TEST_DATABASE_URL para rodar os testes de integração")
	}
	db, err := sql.Open("pgx", url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Ping(); err != nil {
		t.Fatalf("banco de teste indisponível: %v", err)
	}
	return db
}

// NewUser cria uma pessoa com e-mail único e a apaga no fim do teste.
func NewUser(t *testing.T, db *sql.DB, tag string) string {
	t.Helper()
	var id string
	email := fmt.Sprintf("%s-%d@teste.local", tag, time.Now().UnixNano())
	if err := db.QueryRow(
		`INSERT INTO users (email, name, password_hash) VALUES ($1, $2, 'x') RETURNING id`,
		email, "Teste "+tag).Scan(&id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Exec(`DELETE FROM users WHERE id = $1`, id) })
	return id
}

// NewHousehold cria um casal com os membros informados e o apaga no fim do teste.
func NewHousehold(t *testing.T, db *sql.DB, memberIDs ...string) string {
	t.Helper()
	var id string
	if err := db.QueryRow(`INSERT INTO households DEFAULT VALUES RETURNING id`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Exec(`DELETE FROM households WHERE id = $1`, id) })
	for _, m := range memberIDs {
		if _, err := db.Exec(
			`INSERT INTO household_members (household_id, user_id) VALUES ($1, $2)`, id, m); err != nil {
			t.Fatal(err)
		}
	}
	return id
}
```

**6. Integração: um casal não vê o outro** (`internal/reminder/isolation_test.go`)

```go
package reminder

import (
	"context"
	"errors"
	"testing"
	"time"

	"casal/internal/testutil"
)

func TestHouseholdsAreIsolated(t *testing.T) {
	db := testutil.OpenDB(t)
	ctx := context.Background()

	userA := testutil.NewUser(t, db, "a")
	userB := testutil.NewUser(t, db, "b")
	hhA := testutil.NewHousehold(t, db, userA)
	hhB := testutil.NewHousehold(t, db, userB)
	repo := NewPostgresRepo(db)

	created, err := repo.Create(ctx, Reminder{
		HouseholdID: hhA, CreatedBy: userA, RecipientID: userA,
		Body: "segredo do casal A", RemindAt: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}

	list, err := repo.List(ctx, hhB, userB)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Errorf("o casal B não deveria ver lembretes do A, viu %d", len(list))
	}

	if _, err := repo.Resolve(ctx, hhB, created.ID, userB, time.Now()); !errors.Is(err, ErrNotFound) {
		t.Errorf("B resolveu um lembrete de A: %v", err)
	}
	if _, err := repo.Resolve(ctx, hhA, created.ID, userB, time.Now()); !errors.Is(err, ErrNotFound) {
		t.Errorf("quem não é o destinatário resolveu: %v", err)
	}
	if err := repo.Cancel(ctx, hhB, created.ID, userB); !errors.Is(err, ErrNotFound) {
		t.Errorf("B cancelou um lembrete de A: %v", err)
	}

	if _, err := repo.Resolve(ctx, hhA, created.ID, userA, time.Now()); err != nil {
		t.Errorf("A deveria conseguir resolver o próprio lembrete: %v", err)
	}
}
```

**7. Integração: nunca mais de duas pessoas** (`internal/household/concurrency_test.go`)

```go
package household

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"casal/internal/auth"
	"casal/internal/testutil"
)

func TestOnlyOneInviteTakesTheLastSlot(t *testing.T) {
	db := testutil.OpenDB(t)
	ctx := context.Background()

	owner := testutil.NewUser(t, db, "dono")
	b := testutil.NewUser(t, db, "b")
	c := testutil.NewUser(t, db, "c")
	hh := testutil.NewHousehold(t, db, owner)
	repo := NewPostgresRepo(db)

	// dois convites válidos para a mesma vaga
	tokens := []string{"token-um", "token-dois"}
	for _, tk := range tokens {
		if err := repo.CreateInvite(ctx, hh, owner, auth.HashToken(tk), time.Now().Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
	}

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, who := range []string{b, c} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = repo.AcceptInvite(ctx, auth.HashToken(tokens[i]), who, time.Now())
		}()
	}
	wg.Wait()

	ok, full := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, ErrHouseholdFull):
			full++
		default:
			t.Errorf("erro inesperado: %v", err)
		}
	}
	if ok != 1 || full != 1 {
		t.Fatalf("esperava 1 aceito e 1 recusado, veio %d e %d", ok, full)
	}

	members, err := repo.Members(ctx, hh)
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != MaxMembers {
		t.Errorf("o casal deveria ter %d pessoas, tem %d", MaxMembers, len(members))
	}
}
```

Os testes 6 e 7 são os mais importantes do projeto: se algum dia falharem, **não faça deploy**.

**Ordem recomendada:** escreva cada teste unitário antes da regra que ele cobre, veja-o falhar (`go test ./internal/auth/...`) e só então implemente. Para os de integração, rode primeiro com o código correto e depois quebre de propósito (tire o `household_id` de um `WHERE`) para confirmar que o teste realmente detecta o erro.

## Checklist da Fase 0 e próximos passos

A Fase 0 termina quando um lembrete criado por uma pessoa chega, no horário certo, ao celular da outra, com o app fechado, e quando nenhum casal enxerga o outro.

**`cmd/api/main.go` completo**

As ligações ficaram espalhadas pelos passos; esta é a versão final, com tudo no lugar.

```go
package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // registra o driver "pgx" no database/sql

	"casal/internal/auth"
	"casal/internal/config"
	"casal/internal/household"
	"casal/internal/httpx"
	"casal/internal/push"
	"casal/internal/reminder"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("falha fatal", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := db.PingContext(r.Context()); err != nil {
			httpx.Error(w, http.StatusServiceUnavailable, "banco indisponível")
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// contas e sessões
	authSvc, err := auth.NewService(auth.NewPostgresRepo(db), cfg.SessionTTL)
	if err != nil {
		return err
	}
	auth.NewHandler(authSvc, log, cfg.CookieSecure).Routes(mux)
	requireUser := auth.Require(authSvc)

	// casal e convite
	householdSvc := household.NewService(household.NewPostgresRepo(db))
	household.NewHandler(householdSvc, log).Routes(mux, requireUser)
	requireHousehold := household.RequireHousehold(householdSvc)

	// lembretes
	reminderRepo := reminder.NewPostgresRepo(db)
	reminder.NewHandler(reminder.NewService(reminderRepo, householdSvc), log).
		Routes(mux, requireUser, requireHousehold)

	// push e agendador
	var notifier reminder.Notifier = logNotifier{log}
	if cfg.VAPIDPublicKey != "" && cfg.VAPIDPrivateKey != "" {
		pushSvc := push.NewService(
			push.NewPostgresRepo(db),
			push.NewWebPushSender(cfg.VAPIDSubject, cfg.VAPIDPublicKey, cfg.VAPIDPrivateKey),
			log,
		)
		push.NewHandler(pushSvc, cfg.VAPIDPublicKey, log).Routes(mux, requireUser)
		notifier = pushSvc
	} else {
		log.Warn("VAPID não configurado: os avisos só aparecerão no log")
	}
	go reminder.NewWorker(reminderRepo, notifier, log).Run(ctx)

	// página de teste do PWA
	mux.Handle("GET /", http.FileServer(http.Dir("web")))

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpx.Chain(mux, httpx.Recover(log), httpx.Logging(log)),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	log.Info("servidor iniciado", "addr", cfg.HTTPAddr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// logNotifier é o aviso de mentira usado enquanto o VAPID não está configurado.
type logNotifier struct{ log *slog.Logger }

func (n logNotifier) Notify(_ context.Context, userID, title, body string) error {
	n.log.Info("notificação (simulada)", "user", userID, "title", title, "body", body)
	return nil
}
```

**Checklist da Fase 0**

- [ ] Ferramentas instaladas (Go 1.24+, Docker, goose, extensões do VS Code)
- [ ] Repositório criado no Git, com `.env` fora do controle de versão
- [ ] Postgres sobe com `make db-up` e a migração `00001` aplica sem erro
- [ ] `/healthz` responde `200` e, com o banco parado, `503`
- [ ] Cadastro, login, logout e `/v1/me` funcionando
- [ ] Casal criado, convite gerado e aceito pela segunda pessoa
- [ ] Terceiro convite recusado com `409`; convite repetido recusado com `404`
- [ ] Lembrete criado, disparado pelo agendador (log) e resolvido pelo destinatário
- [ ] Testes unitários passando (`make test`)
- [ ] Testes de integração passando (`make test-integration`), incluindo o isolamento e a vaga única
- [ ] Chaves VAPID geradas e guardadas no `.env`
- [ ] Push funcionando no computador (botão "Enviar teste para mim")
- [ ] Push funcionando no **Android** com o app fechado
- [ ] Push funcionando no **iPhone**, com o app instalado na tela inicial e a tela bloqueada
- [ ] Lembrete real de uma pessoa para a outra chegando no horário, por pelo menos três dias seguidos
- [ ] RLS ligado (migração `00002`) e o teste de isolamento ainda passando

**Pendências conhecidas (anote como tarefas)**

| Pendência | Por que importa |
| --- | --- |
| Limitar tentativas de login por e-mail e por IP | Impede adivinhar senhas |
| Apagar sessões e convites expirados periodicamente | Evita o banco crescer sem necessidade |
| Reenviar avisos que falharam, com número de tentativas | Hoje uma falha de push não é repetida |
| Recuperação de senha por e-mail | Quem esquece a senha fica sem acesso |
| Sair do casal e apagar a conta | Previsto nas regras do produto |
| Configuração de avisos por pessoa (silêncio, tipos) | Previsto no documento de produto |
| Dockerfile e hospedagem com HTTPS | Necessário para usar fora do computador |
| Cópias de segurança do banco | Os dados do casal não podem se perder |

**Depois da Fase 0**

1. **Rotina semanal** (`internal/routine`): blocos recorrentes, exceções e a visão de horários livres em comum.
2. **Lista de compras e tarefas da casa** (`internal/shopping`, `internal/chores`).
3. **Frontend de verdade:** o app em React e Vite, como PWA, no lugar da página de teste.
4. **Módulo Marmita** (`internal/kitchen`): receitas, cardápio, dia de cozinhar e freezer. Ele chama `shopping` e `reminder` por funções públicas, nunca pelas tabelas.
5. **Módulo Finanças** (`internal/finance`).
6. **Convite para outros casais de teste**, com o RLS ligado e a lista de pendências acima resolvida.

**Como trabalhar no dia a dia**

- Uma feature por vez, em um ramo do Git, com um commit por passo que funciona.
- Para cada feature nova: migração, repositório, service com teste, handler, ligação no `main.go` e um bloco no `requests.http`.
- Antes de cada commit: `make vet` e `make test`.
- Se um trecho deste guia não compilar, o erro está no guia, não em você: corrija, e me diga qual foi para eu ajustar o documento.
