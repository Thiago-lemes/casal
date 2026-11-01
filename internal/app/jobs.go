package app

import (
	"context"
	"log/slog"
	"time"
)

// purger apaga dados que já não servem para nada e devolve quantos apagou.
type purger func(ctx context.Context) (int64, error)

// maintenance roda as limpezas ao iniciar e depois a cada hora.
func maintenance(log *slog.Logger, purgers ...purger) Job {
	return func(ctx context.Context) {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			for _, purge := range purgers {
				if n, err := purge(ctx); err != nil {
					log.Error("falha na limpeza", "err", err)
				} else if n > 0 {
					log.Info("limpeza concluída", "removidos", n)
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}
}
