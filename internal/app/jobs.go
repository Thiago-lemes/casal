package app

import (
	"context"
	"log/slog"
	"time"
)

// purger apaga dados que já não servem para nada e devolve quantos apagou.
type purger func(ctx context.Context) (int64, error)

// maintenance roda as limpezas ao iniciar e depois a cada hora. Quando todas as da rodada dão certo, avisa
// onSuccess (as métricas usam isso para provar que o job está vivo).
func maintenance(log *slog.Logger, name string, onSuccess func(job string), purgers ...purger) Job {
	return func(ctx context.Context) {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			failed := false
			for _, purge := range purgers {
				if n, err := purge(ctx); err != nil {
					failed = true
					log.Error("falha na limpeza", "job", name, "err", err)
				} else if n > 0 {
					log.Info("limpeza concluída", "job", name, "removidos", n)
				}
			}
			if !failed && onSuccess != nil {
				onSuccess(name)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}
}
