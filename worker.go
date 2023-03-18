package main

type Job struct {
	Prompt   string
	Params   PredictParams
	Response chan string
	Reason   FinishReason
	Err      error
}

func NewJob(prompt string, params PredictParams) *Job {
	return &Job{
		Prompt:   prompt,
		Params:   params,
		Response: make(chan string),
	}
}

type Worker struct {
	Model *GGMLModel
	jobCh chan *Job
}

func NewWorker(model *GGMLModel) *Worker {
	return &Worker{
		Model: model,
		jobCh: make(chan *Job),
	}
}

func (w *Worker) DispatchJob(job *Job) {
	w.jobCh <- job
}

func (w *Worker) Run() {
	for job := range w.jobCh {
		w.runJob(job)
	}
}

func (w *Worker) runJob(job *Job) {
	reason, err := w.Model.Predict(job.Params, job.Prompt, func(word string) {
		job.Response <- word
	})
	job.Err = err
	job.Reason = reason
	close(job.Response)
}
