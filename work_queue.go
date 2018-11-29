package main

import (
	"fmt"
	"log"

	"github.com/google/uuid"
)

type TaskName int

type LogFormatFn func(string, ...interface{})
type LogFn func(...interface{})

type Job struct {
	Payload   interface{} `json:"payload"`
	TaskName  TaskName    `json:"taskName"`
	LogPrefix string      `json:"logPrefix"`
}

type Worker struct {
	ID         uuid.UUID      `json:"id"`
	WorkerPool chan chan Job  `json:"pool"`
	JobChannel chan Job       `json:"channel"`
	Quit       chan chan bool `json:"quit"`
}

type Dispatcher struct {
	WorkerPool chan chan Job  `json:"pool"`
	MaxWorkers int            `json:"maxWorkers"`
	Quit       chan chan bool `json:"quit"`
	Workers    []Worker       `json:"workers"`
}

const (
	EventDetail TaskName = iota
)

var (
	MaxWorker = getEnvInt("MAX_WORKERS", 10)
	MaxQueue  = getEnvInt("MAX_QUEUE", 100)
	JobQueue  = make(chan Job)
)

func NewWorker(workerPool chan chan Job) Worker {
	return Worker{
		ID:         uuid.New(),
		WorkerPool: workerPool,
		JobChannel: make(chan Job),
		Quit:       make(chan chan bool),
	}
}

func (worker Worker) Start() {
	worker.Log(log.Println, "Started")
	go func() {
		for {
			worker.WorkerPool <- worker.JobChannel

			select {
			case job := <-worker.JobChannel:
				job.LogPrefix = worker.LogPrefix()
				worker.Logf(log.Printf, "Processing job %+v\n", job)
				job.Execute()
			case done := <-worker.Quit:
				worker.Log(log.Println, "Quitting")
				done <- true
				return
			}
		}
	}()
}

func (Worker Worker) Stop() {
	done := make(chan bool)
	go func() {
		Worker.Quit <- done
	}()

	<-done
}

func NewDispatcher(maxWorkers int) *Dispatcher {
	return &Dispatcher{
		WorkerPool: make(chan chan Job, maxWorkers),
		MaxWorkers: maxWorkers,
		Quit:       make(chan chan bool),
		Workers:    make([]Worker, maxWorkers),
	}
}

func (dispatcher *Dispatcher) Run() {
	log.Printf("[Dispatcher]: Starting %v Workers\n", dispatcher.MaxWorkers)
	for i := 0; i < dispatcher.MaxWorkers; i++ {
		worker := NewWorker(dispatcher.WorkerPool)
		worker.Start()
		dispatcher.Workers[i] = worker
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
		case job := <-JobQueue:
			go func(job Job) {
				JobChannel := <-dispatcher.WorkerPool

				log.Printf("[Dispatcher]: Dispatching Job: %+v\n", job)
				JobChannel <- job
			}(job)
		case done := <-dispatcher.Quit:
			go func() {
				log.Println("[Dispatcher]: Closing all Workers")
				for _, worker := range dispatcher.Workers {
					worker.Stop()
				}
				log.Println("[Dispatcher]: Quitting")
				done <- true
			}()
		}
	}
}

func (worker Worker) LogPrefix() string {
	return fmt.Sprintf("Worker [%s]", worker.ID.String())
}

func (worker Worker) Logf(logFn LogFormatFn, value string, args ...interface{}) {
	logStatement := fmt.Sprintf("%s: %s", worker.LogPrefix(), value)
	logFn(logStatement, args)
}

func (worker Worker) Log(logFn LogFn, value string, args ...interface{}) {
	logStatement := fmt.Sprintf("%s: %s", worker.LogPrefix(), value)

	if len(args) > 0 {
		logFn(logStatement, args)
	} else {
		logFn(logStatement)
	}
}

func (job Job) Execute() {
	switch job.TaskName {
	case EventDetail:
		collectDetail(job)
	}
}

func (job Job) Logf(logFn LogFormatFn, value string, args ...interface{}) {
	logStatement := fmt.Sprintf("%s: %s", job.LogPrefix, value)
	logFn(logStatement, args)
}

func (job Job) Log(logFn LogFn, value string, args ...interface{}) {
	logStatement := fmt.Sprintf("%s: %s", job.LogPrefix, value)

	if len(args) > 0 {
		logFn(logStatement, args)
	} else {
		logFn(logStatement)
	}
}
