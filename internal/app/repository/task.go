package repository

import (
	"errors"
	"io"
	"net/http"
	"sync"
	"test_task_golang/internal/model"

	"github.com/google/uuid"
)

const (
	defaultMaxWorkers   = 5
	taskStatusInProcess = "in_process"
	taskStatusError     = "error"
	taskStatusDone      = "done"
)

var (
	ErrTaskNotFound = errors.New("task not found")
)

type repository struct {
	tasks      map[string]*model.Task
	m          sync.Mutex
	taskQueue  chan *model.Task
	Wg         sync.WaitGroup
	maxWorkers int
}

func New() *repository {
	return &repository{
		tasks:      make(map[string]*model.Task),
		taskQueue:  make(chan *model.Task, defaultMaxWorkers),
		maxWorkers: defaultMaxWorkers,
	}
}

// Create adds a new task to the repository and starts processing it
func (r *repository) Create(t *model.Task) (string, error) {
	r.m.Lock()
	defer r.m.Unlock()

	id := uuid.NewString()
	t.Status = &model.TaskStatus{
		ID:     id,
		Status: taskStatusInProcess,
	}
	r.tasks[id] = t

	r.taskQueue <- t
	r.Wg.Add(1)
	go r.executeTask()

	return id, nil
}

// Get retrieves the status of a task by its ID
func (r *repository) Get(taskID string) (*model.TaskStatus, error) {
	r.m.Lock()
	defer r.m.Unlock()

	task, ok := r.tasks[taskID]
	if !ok {
		return nil, ErrTaskNotFound
	}
	return task.Status, nil
}

// executeTask processes a task from the queue
func (r *repository) executeTask() {
	defer r.Wg.Done()

	for task := range r.taskQueue {
		client := &http.Client{}

		request, err := http.NewRequest(task.Method, task.URL, nil)
		if err != nil {
			task.Status.Status = taskStatusError
			return
		}

		for key, value := range task.Headers {
			request.Header.Set(key, value)
		}

		response, err := client.Do(request)
		if err != nil {
			task.Status.Status = taskStatusError
			return
		}
		defer response.Body.Close()

		task.Status.HTTPStatusCode = response.StatusCode
		headers := make(map[string]string)
		for key, value := range response.Header {
			headers[key] = value[0]
		}
		task.Status.Headers = headers

		body, err := io.ReadAll(response.Body)
		if err != nil {
			task.Status.Status = taskStatusError
			return
		}

		task.Status.Length = int(len(body))
		task.Status.Status = taskStatusDone
	}
}
