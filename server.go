package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type APIServer struct {
	Seed      int
	WorkerMgr *WorkerManager
	Listen    string
}

func respJson(c *gin.Context, code int, data any) {
	c.IndentedJSON(code, data)
}

func respJsonErr(c *gin.Context, err error) {
	respJsonErrStr(c, err.Error())
}

func respJsonErrStr(c *gin.Context, msg string) {
	respJson(c, 400, gin.H{
		"Error": msg,
	})
}

func (s *APIServer) Run() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	s.setupRouter(r)

	log.Println("[API Server] Starting at", s.Listen)
	r.Run(s.Listen)
}

func (s *APIServer) setupRouter(r *gin.Engine) {
	ar := r.Group("/api")
	ar.GET("/", s.Help)
	ar.POST("/completion", s.Completion)
	ar.GET("/ws/completion", s.StreamCompletion)
}

func (s *APIServer) Help(c *gin.Context) {
	respJson(c, 200, gin.H{
		"/api/":              "Help",
		"/api/completion":    "Completion",
		"/api/ws/completion": "Completion web socket",
	})
}

type CompletionParams struct {
	Prompt        string  `json:"prompt"`
	Tokens        int     `json:"tokens"`
	TopK          int     `json:"top_k,omitempty"`
	RepeatLastN   int     `json:"repeat_lastn,omitempty"`
	TopP          float32 `json:"top_p,omitempty"`
	Temp          float32 `json:"temp,omitempty"`
	RepeatPenalty float32 `json:"repeat_penalty,omitempty"`
	Stream        bool    `json:"stream,omitempty"`
}

func (p *CompletionParams) ToPredictParams(seed int) PredictParams {
	return PredictParams{
		Seed:          seed,
		Tokens:        p.Tokens,
		RepeatLastN:   p.RepeatLastN,
		TopK:          p.TopK,
		TopP:          p.TopP,
		Temp:          p.Temp,
		RepeatPenalty: p.RepeatPenalty,
		NBatch:        8,
	}
}

type StreamResponse struct {
	Text   string `json:"text"`
	Finish bool   `json:"finish"`
	Reason string `json:"reason"`
}

func (r StreamResponse) Encode() []byte {
	ret, _ := json.Marshal(r)
	ret = append(ret, '\n')
	return ret
}

func (s *APIServer) Completion(c *gin.Context) {
	reqParams := &CompletionParams{
		TopK:          40,
		TopP:          0.9,
		Temp:          0.8,
		RepeatPenalty: 1.3,
		RepeatLastN:   64,
	}
	err := c.BindJSON(reqParams)
	if err != nil {
		respJsonErr(c, err)
		return
	}
	if reqParams.Prompt == "" {
		respJsonErrStr(c, "Empty prompt")
		return
	}
	if reqParams.Tokens == 0 {
		respJsonErrStr(c, "Tokens is zero")
		return
	}
	pp := reqParams.ToPredictParams(s.Seed)
	job := NewJob(reqParams.Prompt, pp)
	s.WorkerMgr.DispatchJob(job)
	if reqParams.Stream {
		c.Stream(func(w io.Writer) bool {
			output, ok := <-job.Response
			if !ok {
				resp := StreamResponse{
					Text:   "",
					Finish: true,
					Reason: job.Reason,
				}
				w.Write(resp.Encode())
				return false
			}
			resp := StreamResponse{
				Text:   output,
				Finish: false,
				Reason: "",
			}
			w.Write(resp.Encode())
			return true
		})
	} else {
		resp := ""
		tokens := 0
		for word := range job.Response {
			resp += word
			tokens += 1
		}
		if job.Err != nil {
			respJsonErr(c, err)
			return
		}
		respJson(c, 200, gin.H{
			"Prompt":         reqParams.Prompt,
			"Text":           resp,
			"Tokens":         tokens,
			"CompleteReason": job.Reason,
		})
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  8192,
	WriteBufferSize: 8192,
	Subprotocols:    []string{"binary"},
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (s *APIServer) StreamCompletion(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("Upgrade conn got error:", err)
		respJsonErrStr(c, "Bad Request")
		return
	}
	defer conn.Close()
	for {
		tp, payload, err := conn.ReadMessage()
		if err != nil {
			if err != io.EOF {
				log.Println("Read got error:", err)
			}
			return
		}
		switch tp {
		case websocket.BinaryMessage:
			// Skip Binary Message
			continue
		case websocket.TextMessage:
		default:
			log.Printf("Invalid message type %d\n", tp)
			return
		}
		// Here is TextMessage
		reqParams := &CompletionParams{
			TopK:          40,
			TopP:          0.9,
			Temp:          0.8,
			RepeatPenalty: 1.3,
			RepeatLastN:   64,
		}
		err = json.Unmarshal(payload, reqParams)
		if err != nil {
			log.Println("Bad Request:", err)
			return
		}
		if reqParams.Prompt == "" {
			err = wsWriteErr(conn, "Empty prompt")
			if err != nil {
				log.Println("Write web socket got error", err)
				return
			}
			continue
		}
		if reqParams.Tokens == 0 {
			err = wsWriteErr(conn, "Tokens is zero")
			if err != nil {
				log.Println("Write web socket got error", err)
				return
			}
			continue
		}
		pp := reqParams.ToPredictParams(s.Seed)
		job := NewJob(reqParams.Prompt, pp)
		s.WorkerMgr.DispatchJob(job)
		for word := range job.Response {
			rmsg := WsResponseMsg{
				Text:   word,
				Error:  "",
				Reason: "",
				Finish: false,
			}
			err = wsWriteResp(conn, rmsg)
			if err != nil {
				log.Println("Write web socket got error", err)
				return
			}
		}
		errMsg := ""
		if job.Err != nil {
			errMsg = job.Err.Error()
		}
		rmsg := WsResponseMsg{
			Text:   "",
			Error:  errMsg,
			Reason: job.Reason,
			Finish: true,
		}
		err = wsWriteResp(conn, rmsg)
		if err != nil {
			log.Println("Write web socket got error", err)
			return
		}
	}
}

type WsResponseMsg struct {
	Text   string `json:"text"`
	Error  string `json:"error"`
	Reason string `json:"reason"`
	Finish bool   `json:"finish"`
}

func (m WsResponseMsg) Encode() []byte {
	ret, _ := json.Marshal(m)
	return ret
}

func wsWriteErr(conn *websocket.Conn, msg string) error {
	rmsg := WsResponseMsg{
		Text:   "",
		Error:  msg,
		Reason: "Error",
		Finish: true,
	}
	return conn.WriteMessage(websocket.TextMessage, rmsg.Encode())
}

func wsWriteResp(conn *websocket.Conn, rmsg WsResponseMsg) error {
	return conn.WriteMessage(websocket.TextMessage, rmsg.Encode())
}
