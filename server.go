package main

import (
	"io"
	"log"

	"github.com/gin-gonic/gin"
)

type APIServer struct {
	Seed   int
	Model  *GGMLModel
	Listen string
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
}

func (s *APIServer) Help(c *gin.Context) {
	respJson(c, 200, gin.H{
		"/api/":           "Help",
		"/api/completion": "Completion",
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
	if reqParams.Stream {
		wordChan := make(chan string)
		go func() {
			s.Model.Predict(pp, reqParams.Prompt, func(word string) {
				wordChan <- word
			})
			close(wordChan)
		}()
		c.Stream(func(w io.Writer) bool {
			output, ok := <-wordChan
			if !ok {
				return false
			}
			w.Write([]byte(output))
			return true
		})
	} else {
		resp := ""
		tokens := 0
		reason, err := s.Model.Predict(pp, reqParams.Prompt, func(word string) {
			resp += word
			tokens += 1
		})
		if err != nil {
			respJsonErr(c, err)
			return
		}
		respJson(c, 200, gin.H{
			"Prompt":         reqParams.Prompt,
			"Text":           resp,
			"Tokens":         tokens,
			"CompleteReason": reason.String(),
		})
	}
}
