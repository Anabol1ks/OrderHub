package cleanup

import (
	"context"
	"time"

	"go.uber.org/zap"
)

type Scheduler struct {
	cleanup *CleanupService
	log     *zap.Logger
	stopCh  chan struct{}
}

func NewScheduler(cleanup *CleanupService, log *zap.Logger) *Scheduler {
	return &Scheduler{
		cleanup: cleanup,
		log:     log,
		stopCh:  make(chan struct{}),
	}
}

// Start запускает планировщик задач
func (s *Scheduler) Start(ctx context.Context) {
	s.log.Info("starting cleanup scheduler")

	// Запускаем горутины для разных задач
	go s.runExpiredTokensCleanup(ctx)
	go s.runSessionsCleanup(ctx)
	go s.runConsumedTokensCleanup(ctx)
}

// Stop останавливает планировщик
func (s *Scheduler) Stop() {
	s.log.Info("stopping cleanup scheduler")
	close(s.stopCh)
}

// runExpiredTokensCleanup очищает истёкшие токены каждые 30 минут
func (s *Scheduler) runExpiredTokensCleanup(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	// Выполняем сразу при старте
	if err := s.cleanup.CleanupExpiredTokens(ctx); err != nil {
		s.log.Error("initial expired tokens cleanup failed", zap.Error(err))
	}

	for {
		select {
		case <-ticker.C:
			if err := s.cleanup.CleanupExpiredTokens(ctx); err != nil {
				s.log.Error("expired tokens cleanup failed", zap.Error(err))
			}
		case <-s.stopCh:
			s.log.Info("expired tokens cleanup stopped")
			return
		case <-ctx.Done():
			s.log.Info("expired tokens cleanup cancelled")
			return
		}
	}
}

// runSessionsCleanup очищает сессии каждый час
func (s *Scheduler) runSessionsCleanup(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := s.cleanup.CleanupOrphanedSessions(ctx); err != nil {
				s.log.Error("orphaned sessions cleanup failed", zap.Error(err))
			}
			if err := s.cleanup.CleanupOldSessions(ctx); err != nil {
				s.log.Error("old sessions cleanup failed", zap.Error(err))
			}
		case <-s.stopCh:
			s.log.Info("sessions cleanup stopped")
			return
		case <-ctx.Done():
			s.log.Info("sessions cleanup cancelled")
			return
		}
	}
}

// runConsumedTokensCleanup очищает использованные токены каждые 6 часов
func (s *Scheduler) runConsumedTokensCleanup(ctx context.Context) {
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := s.cleanup.CleanupConsumedTokens(ctx); err != nil {
				s.log.Error("consumed tokens cleanup failed", zap.Error(err))
			}
		case <-s.stopCh:
			s.log.Info("consumed tokens cleanup stopped")
			return
		case <-ctx.Done():
			s.log.Info("consumed tokens cleanup cancelled")
			return
		}
	}
}

// RunOnceNow выполняет полную очистку немедленно (для тестирования)
func (s *Scheduler) RunOnceNow(ctx context.Context) error {
	return s.cleanup.RunFullCleanup(ctx)
}
