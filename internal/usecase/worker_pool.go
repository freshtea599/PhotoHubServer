// backend/internal/usecase/worker_pool.go
package usecase

import (
	"context"
	"sync"
	"time"

	"github.com/freshtea599/PhotoHubServer.git/internal/domain"
)

// WorkerPool управляет фиксированным пулом горутин для обработки изображений.
type WorkerPool struct {
	highJobs chan domain.Job
	midJobs  chan domain.Job
	lowJobs  chan domain.Job

	handler func(domain.Job) domain.JobResult

	numWorkers int
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewWorkerPool создаёт новый пул с заданным числом воркеров и функцией обработки.
func NewWorkerPool(numWorkers int, handler func(domain.Job) domain.JobResult) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	pool := &WorkerPool{
		highJobs:   make(chan domain.Job, 50),
		midJobs:    make(chan domain.Job, 50),
		lowJobs:    make(chan domain.Job, 100),
		handler:    handler,
		numWorkers: numWorkers,
		ctx:        ctx,
		cancel:     cancel,
	}
	return pool
}

// Start запускает всех воркеров.
func (wp *WorkerPool) Start() {
	for i := 0; i < wp.numWorkers; i++ {
		wp.wg.Add(1)
		go wp.worker()
	}
}

// Shutdown завершает работу пула (закрывает каналы и ждёт завершения воркеров).
func (wp *WorkerPool) Shutdown() {
	wp.cancel()
	// Закрываем каналы, чтобы воркеры вышли из select.
	close(wp.highJobs)
	close(wp.midJobs)
	close(wp.lowJobs)
	wp.wg.Wait()
}

// Submit отправляет задачу в пул и ожидает результат.
// Возвращает результат или ошибку, если истекло время ожидания.
func (wp *WorkerPool) Submit(ctx context.Context, job domain.Job) (domain.JobResult, error) {
	// Создаём канал для получения результата (каждая задача несёт свой канал).
	resultChan := make(chan domain.JobResult, 1)
	job.ResultChan = resultChan

	// Отправляем задачу в соответствующий приоритетный канал.
	switch job.Priority {
	case domain.PriorityHigh:
		select {
		case wp.highJobs <- job:
		case <-ctx.Done():
			return domain.JobResult{}, ctx.Err()
		}
	case domain.PriorityMedium:
		select {
		case wp.midJobs <- job:
		case <-ctx.Done():
			return domain.JobResult{}, ctx.Err()
		}
	default:
		select {
		case wp.lowJobs <- job:
		case <-ctx.Done():
			return domain.JobResult{}, ctx.Err()
		}
	}

	// Ожидаем результат с таймаутом.
	select {
	case res := <-resultChan:
		return res, nil
	case <-ctx.Done():
		return domain.JobResult{}, ctx.Err()
	case <-time.After(30 * time.Second): // защита от зависания
		return domain.JobResult{}, context.DeadlineExceeded
	}
}

// worker — бесконечный цикл обработки задач из трёх очередей.
func (wp *WorkerPool) worker() {
	defer wp.wg.Done()
	for {
		var job domain.Job
		var ok bool
		select {
		case job, ok = <-wp.highJobs:
		case job, ok = <-wp.midJobs:
			if !ok {
				// если mid закрыт, пробуем low (или выходим по ctx)
				select {
				case job, ok = <-wp.lowJobs:
				case <-wp.ctx.Done():
					return
				}
			}
		case job, ok = <-wp.lowJobs:
		case <-wp.ctx.Done():
			return
		}
		if !ok {
			// канал закрыт – завершаемся
			return
		}
		// Выполняем задачу
		result := wp.handler(job)
		// Отправляем результат обратно в канал, переданный в задаче
		select {
		case job.ResultChan <- result:
		default:
			// если получатель уже не ждёт (например, отвалился по таймауту)
		}
	}
}
