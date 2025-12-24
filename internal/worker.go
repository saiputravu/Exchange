package server

import (
	"github.com/rs/zerolog/log"
	tomb "gopkg.in/tomb.v2"
)

const (
	TASK_CHAN_SIZE = 100
)

type WorkerFunction = func(t *tomb.Tomb, task any) error
type WorkerPool struct {
	n     int            // number of workers
	tasks chan any       // task connection pool
	work  WorkerFunction // do work method
}

func NewWorkerPool(size int) WorkerPool {
	return WorkerPool{
		tasks: make(chan any, TASK_CHAN_SIZE),
		n:     size,
	}
}

func (pool *WorkerPool) Setup(t *tomb.Tomb, work WorkerFunction) {
	// Maintain a full pool of workers.
	log.Info().Int("activeWorkers", pool.n).Msg("adding workers")
	activeWorkers := 0
	for {
		select {
		case <-t.Dying():
			return
		default:
			if activeWorkers < pool.n {
				t.Go(func() error {
					err := pool.worker(t, work)
					activeWorkers--
					return err
				})
				activeWorkers++
			}
		}
	}
}

// Workers wait on tasks in the task connection pool and action them.
func (pool *WorkerPool) worker(t *tomb.Tomb, work WorkerFunction) error {
	log.Info().Msg("worker starting")
	select {
	case <-t.Dying():
		return nil
	case task := <-pool.tasks:
		if err := work(t, task); err != nil {
			log.Error().Err(err).Msg("worker exiting")
			return err
		}
	}
	return nil
}
