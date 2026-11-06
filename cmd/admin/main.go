// Comando de administração do beta, rodado na máquina de quem opera (não é uma rota da API).
//
//	go run ./cmd/admin signup-code create [--note "amiga da Carla"] [--uses 1] [--days 14]
//	go run ./cmd/admin signup-code list
//	go run ./cmd/admin signup-code revoke <id>
//	go run ./cmd/admin admin create <email>      (conta do painel /admin: mostra senha e segredo do 2FA uma única vez)
//
// Usa a DATABASE_URL do ambiente (o mesmo papel do app).
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"time"

	"casal/internal/admin"
	"casal/internal/config"
	"casal/internal/db"
	"casal/internal/signup"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(1)
	}
}

const usage = `uso:
  admin signup-code create [--note "rótulo"] [--uses 1] [--days 14]
  admin signup-code list
  admin signup-code revoke <id>
  admin admin create <email>`

func run(ctx context.Context, args []string) error {
	if len(args) < 2 || (args[0] != "signup-code" && args[0] != "admin") {
		return fmt.Errorf("%s", usage)
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	conn, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer conn.Close()
	if args[0] == "admin" {
		return adminAccount(ctx, conn, args[1:])
	}
	svc := signup.NewService(conn)

	switch args[1] {
	case "create":
		fs := flag.NewFlagSet("create", flag.ContinueOnError)
		note := fs.String("note", "", "rótulo para você lembrar de quem é")
		uses := fs.Int("uses", 1, "quantas contas podem usar o código")
		days := fs.Int("days", 14, "validade em dias")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		c, raw, err := svc.Create(ctx, *note, *uses, time.Duration(*days)*24*time.Hour, "")
		if err != nil {
			return err
		}
		fmt.Printf("Código (aparece só agora, guarde): %s\nválido até %s, %d uso(s), id %s\n",
			raw, c.ExpiresAt.Local().Format("02/01/2006 15:04"), c.MaxUses, c.ID)
		return nil
	case "list":
		codes, err := svc.List(ctx)
		if err != nil {
			return err
		}
		for _, c := range codes {
			state := "ativo"
			switch {
			case c.RevokedAt != nil:
				state = "revogado"
			case time.Now().After(c.ExpiresAt):
				state = "vencido"
			case c.Uses >= c.MaxUses:
				state = "esgotado"
			}
			fmt.Printf("%s  %-9s %d/%d usos  até %s  %s\n", c.ID, state, c.Uses, c.MaxUses, c.ExpiresAt.Local().Format("02/01/2006"), c.Note)
		}
		return nil
	case "revoke":
		if len(args) != 3 {
			return fmt.Errorf("%s", usage)
		}
		if err := svc.Revoke(ctx, args[2]); err != nil {
			return err
		}
		fmt.Println("revogado")
		return nil
	}
	return fmt.Errorf("%s", usage)
}

// adminAccount cria a conta do painel de suporte. Não há cadastro pela API: só quem tem acesso ao servidor cria.
func adminAccount(ctx context.Context, conn *sql.DB, args []string) error {
	if len(args) != 2 || args[0] != "create" {
		return fmt.Errorf("%s", usage)
	}
	svc, err := admin.NewService(admin.NewPostgresRepo(conn), admin.Deps{})
	if err != nil {
		return err
	}
	c, err := svc.CreateAdmin(ctx, args[1])
	if err != nil {
		return err
	}
	fmt.Printf("Conta criada: %s\n\nSenha (aparece só agora): %s\nSegredo do 2FA (aparece só agora): %s\nQR code (cole no aplicativo autenticador): %s\n",
		c.Admin.Email, c.Password, c.Secret, c.URI)
	return nil
}
