package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
)

type Job struct {
	Prompt   string
	Params   PredictParams
	Response chan string
	Reason   string
	Err      error
}

func NewJob(prompt string, params PredictParams) *Job {
	return &Job{
		Prompt:   prompt,
		Params:   params,
		Response: make(chan string),
	}
}

func (j *Job) Finish(reason string, err error) {
	j.Reason = reason
	j.Err = err
	close(j.Response)
}

type workerJob struct {
	params *workerRequest
	respCh chan string
	err    error
	reason FinishReason
}

type workerRequest struct {
	Prompt string
	PP     PredictParams
}

func (r workerRequest) Encode() []byte {
	ret, _ := json.Marshal(r)
	ret = append(ret, '\n')
	return ret
}

type workerResponse struct {
	Text   string
	Finish bool
	Reason string
	Err    string
}

func (r workerResponse) Encode() []byte {
	ret, _ := json.Marshal(r)
	ret = append(ret, '\n')
	return ret
}

type Worker struct {
	Model    *GGMLModel
	sockFile string
	jobCh    chan *workerJob
}

func NewWorker(model *GGMLModel, fname string) *Worker {
	return &Worker{
		Model:    model,
		sockFile: fname,
		jobCh:    make(chan *workerJob),
	}
}

func (w *Worker) Run() error {
	if _, err := os.Stat(w.sockFile); err == nil {
		err = os.Remove(w.sockFile)
		if err != nil {
			return err
		}
	}
	log.Println("Listen unix:", w.sockFile)
	sock, err := net.Listen("unix", w.sockFile)
	if err != nil {
		return err
	}
	// Start model worker
	go w.startModelWorker()
	for {
		conn, err := sock.Accept()
		if err != nil {
			return err
		}
		go w.handleConn(conn)
	}
	return nil
}

func (w *Worker) startModelWorker() {
	for job := range w.jobCh {
		w.runJob(job)
	}
}

func (w *Worker) handleConn(conn net.Conn) {
	defer conn.Close()
	for {
		reader := bufio.NewReader(conn)
		line, err := reader.ReadBytes('\n')
		if err != nil {
			log.Println("Cannot read connection:", err)
			return
		}
		params := new(workerRequest)
		err = json.Unmarshal(line, params)
		if err != nil {
			log.Println("Cannot unmarshal parameter:", err)
			return
		}
		w.handleRequest(conn, params)
	}
}

func (w *Worker) handleRequest(conn net.Conn, p *workerRequest) {
	job := &workerJob{
		params: p,
		respCh: make(chan string),
	}
	w.jobCh <- job
	for text := range job.respCh {
		item := workerResponse{
			Text:   text,
			Finish: false,
			Reason: "",
			Err:    "",
		}
		conn.Write(item.Encode())
	}
	errMsg := ""
	if job.err != nil {
		errMsg = job.err.Error()
	}
	item := workerResponse{
		Text:   "",
		Finish: true,
		Err:    errMsg,
		Reason: job.reason.String(),
	}
	conn.Write(item.Encode())
}

func (w *Worker) runJob(job *workerJob) {
	reason, err := w.Model.Predict(job.params.PP, job.params.Prompt, func(word string) {
		job.respCh <- word
	})
	job.err = err
	job.reason = reason
	close(job.respCh)
}

type workerClient struct {
	id       int
	sockFile string
	start    bool
	conn     net.Conn
	jobCh    chan *Job
}

func (c *workerClient) ensureConn() (net.Conn, error) {
	var err error
	if c.conn == nil {
		c.conn, err = net.Dial("unix", c.sockFile)
		if err != nil {
			c.conn = nil
			return nil, err
		}
	}
	return c.conn, nil
}

func (c *workerClient) run() {
	go func() {
		for job := range c.jobCh {
			c.processJob(job)
		}
	}()
}

func (c *workerClient) processJob(job *Job) {
	conn, err := c.ensureConn()
	if err != nil {
		job.Finish("Error", err)
		return
	}
	req := workerRequest{
		Prompt: job.Prompt,
		PP:     job.Params,
	}
	reqData := req.Encode()
	_, err = conn.Write(reqData)
	if err != nil {
		job.Finish("Error", err)
		conn.Close()
		c.conn = nil
		return
	}
	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				// EOF just finish it
				job.Finish("Error", io.EOF)
				conn.Close()
				c.conn = nil
				return
			}
		}
		resp := new(workerResponse)
		err = json.Unmarshal(line, resp)
		if err != nil {
			job.Finish("Error", err)
			conn.Close()
			c.conn = nil
			return
		}
		if resp.Finish {
			if resp.Err == "" {
				job.Finish(resp.Reason, nil)
			} else {
				job.Finish(resp.Reason, errors.New(resp.Err))
			}
			return
		} else {
			job.Response <- resp.Text
		}
	}
	// Actrually should not got there.
	job.Finish("Error", errors.New("Buggy"))
}

func (c *workerClient) Close() error {
	close(c.jobCh)
	return c.conn.Close()
}

type WorkerManager struct {
	execFile   string
	modelPath  string
	numWorkers int
	ctxSize    int
	threads    int
	workers    []*workerClient
	jobCh      chan *Job
}

func NewWorkerManager(execFile string, modelPath string, numWorkers int, ctxSize int, threads int) *WorkerManager {
	return &WorkerManager{
		execFile:   execFile,
		numWorkers: numWorkers,
		modelPath:  modelPath,
		ctxSize:    ctxSize,
		threads:    threads,
		workers:    make([]*workerClient, numWorkers),
		jobCh:      make(chan *Job),
	}
}

func (m *WorkerManager) StartWorkers() error {
	for i := 0; i < m.numWorkers; i++ {
		sockFile := fmt.Sprintf("/tmp/ggml-worker.%d.sock", i)
		go m.startWorkerProcess(i, sockFile)
		client := &workerClient{
			id:       i,
			sockFile: sockFile,
			start:    true,
			jobCh:    m.jobCh,
		}
		client.run()
		m.workers[i] = client
	}
	return nil
}

func (m *WorkerManager) startWorkerProcess(id int, sockFile string) {
	execFile := m.execFile
	for {
		m.workers[id].start = false
		log.Printf("Start Worker Process %d", id)
		cmd := exec.Command(execFile,
			"-M", "worker",
			"-t", fmt.Sprintf("%d", m.threads),
			"-m", m.modelPath,
			"-S", sockFile,
			"-c", fmt.Sprintf("%d", m.ctxSize),
		)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			log.Println("Cannot get stdout:", err)
			m.workers[id].start = false
			return
		}
		go m.handleStdout(id, stdout)
		m.workers[id].start = true
		err = cmd.Run()
		if err != nil {
			log.Println("Start worker got error", err)
			m.workers[id].start = false
			return
		}
	}
}

func (m *WorkerManager) handleStdout(id int, out io.ReadCloser) {
	for {
		reader := bufio.NewReader(out)
		line, _, err := reader.ReadLine()
		if err != nil {
			log.Printf("[Worker %d] Read Stdout got error: %v", id, err)
			break
		}
		log.Printf("[Worker %d] %s", id, string(line))
	}
	out.Close()
}

func (m *WorkerManager) DispatchJob(job *Job) {
	for i := 0; i < m.numWorkers; i++ {
		client := m.workers[i]
		if !client.start {
			continue
		}
		m.jobCh <- job
		return
	}
	// Means no worker available
	job.Finish("Error", errors.New("No available worker"))
}
