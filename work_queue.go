package main

import (
	"log"

	"github.com/google/uuid"
)

type DetailJob struct {
	Event OfflineEvent `json:"event"`
}

type DetailWorker struct {
	ID               uuid.UUID           `json:"id"`
	WorkerPool       chan chan DetailJob `json:"pool"`
	DetailJobChannel chan DetailJob      `json:"channel"`
	Quit             chan chan bool      `json:"quit"`
}

type Dispatcher struct {
	WorkerPool    chan chan DetailJob `json:"pool"`
	MaxWorkers    int                 `json:"maxWorkers"`
	Quit          chan chan bool      `json:"quit"`
	DetailWorkers []DetailWorker      `json:"workers"`
}

var (
	MaxWorker      = getEnvInt("MAX_WORKERS", 10)
	MaxQueue       = getEnvInt("MAX_QUEUE", 100)
	DetailJobQueue = make(chan DetailJob)
)

func NewDetailWorker(workerPool chan chan DetailJob) DetailWorker {
	return DetailWorker{
		ID:               uuid.New(),
		WorkerPool:       workerPool,
		DetailJobChannel: make(chan DetailJob),
		Quit:             make(chan chan bool),
	}
}

func (detailWorker DetailWorker) Start() {
	log.Printf("DetailWorker[%s]: Started", detailWorker.ID.String())
	go func() {
		for {
			detailWorker.WorkerPool <- detailWorker.DetailJobChannel

			select {
			case detailJob := <-detailWorker.DetailJobChannel:
				/* process offline event here */
				log.Printf("DetailWorker[%s]: Processing detail job %+v\n", detailWorker.ID.String(), detailJob)
			case done := <-detailWorker.Quit:
				log.Printf("DetailWorker[%s]: Quitting\n", detailWorker.ID.String())
				done <- true
				return
			}
		}
	}()
}

func (detailWorker DetailWorker) Stop() {
	done := make(chan bool)
	go func() {
		detailWorker.Quit <- done
	}()

	<-done
}

func NewDispatcher(maxWorkers int) *Dispatcher {
	return &Dispatcher{
		WorkerPool:    make(chan chan DetailJob, maxWorkers),
		MaxWorkers:    maxWorkers,
		Quit:          make(chan chan bool),
		DetailWorkers: make([]DetailWorker, maxWorkers),
	}
}

func (dispatcher *Dispatcher) Run() {
	log.Printf("Starting %v DetailWorkers\n", dispatcher.MaxWorkers)
	for i := 0; i < dispatcher.MaxWorkers; i++ {
		worker := NewDetailWorker(dispatcher.WorkerPool)
		worker.Start()
		dispatcher.DetailWorkers[i] = worker
	}

	go dispatcher.Dispatch()
}

func (dispatcher *Dispatcher) Stop() {
	log.Println("[Dispatcher]: Received request to stop")
	done := make(chan bool)
	go func() {
		dispatcher.Quit <- done
	}()

	<-done
}

func (dispatcher *Dispatcher) Dispatch() {
	for {
		select {
		case detailJob := <-DetailJobQueue:
			go func(detailJob DetailJob) {
				detailJobChannel := <-dispatcher.WorkerPool

				log.Printf("[Dispatcher]: Dispatching DetailJob: %+v\n", detailJob)
				detailJobChannel <- detailJob
			}(detailJob)
		case done := <-dispatcher.Quit:
			go func() {
				log.Println("[Dispatcher]: Closing all DetailWorkers")
				for _, worker := range dispatcher.DetailWorkers {
					worker.Stop()
				}
				log.Println("[Dispatcher]: Quitting")
				done <- true
			}()
		}
	}
}
