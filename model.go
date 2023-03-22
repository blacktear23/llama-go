package main

/*
#cgo CFLAGS:   -I. -O3 -DNDEBUG -std=c11 -fPIC -pthread -mavx -mavx2 -mfma -mf16c -msse3
#cgo CXXFLAGS: -O3 -DNDEBUG -std=c++11 -fPIC -pthread -I.
#cgo LDFLAGS:  -L . -l llama

#include <stdint.h>
#include "main.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"runtime/cgo"
	"unsafe"
)

const (
	PROMPT_ERR    FinishReason = 0
	PROMPT_FINISH FinishReason = 1
	PROMPT_STOP   FinishReason = 2
)

type FinishReason int

func (r FinishReason) String() string {
	switch r {
	case PROMPT_ERR:
		return "Error"
	case PROMPT_FINISH:
		return "Finish"
	case PROMPT_STOP:
		return "Stop"
	}
	return "Unknown"
}

//export prompt_callback_bridge
func prompt_callback_bridge(h C.uintptr_t, word *C.char) {
	data := C.GoString(word)
	fn := cgo.Handle(h).Value().(WordCallbackFn)
	fn(data)
}

//export tokenizer_callback_bridge
func tokenizer_callback_bridge(h C.uintptr_t, word *C.char) {
	data := C.GoString(word)
	fn := cgo.Handle(h).Value().(WordCallbackFn)
	fn(data)
}

type WordCallbackFn func(data string)

type PredictParams struct {
	Seed          int
	Tokens        int
	RepeatLastN   int
	TopK          int
	NBatch        int
	TopP          float32
	Temp          float32
	RepeatPenalty float32
}

func DefaultPredictParams(tokens int) PredictParams {
	return PredictParams{
		Seed:          -1,
		Tokens:        tokens,
		TopK:          40,
		NBatch:        8,
		TopP:          0.95,
		Temp:          0.9,
		RepeatPenalty: 1.10,
		RepeatLastN:   64,
	}
}

type GGMLModel struct {
	path    string
	nctx    int
	nParts  int
	threads int
	state   unsafe.Pointer
}

func NewGGMLModel(path string, nctx int, threads int, nParts int) *GGMLModel {
	return &GGMLModel{
		path:    path,
		nctx:    nctx,
		nParts:  nParts,
		threads: threads,
	}
}

func SystemInfo() string {
	info := C.llama_print_system_info()
	return C.GoString(info)
}

func (m *GGMLModel) Load() error {
	modelPath := C.CString(m.path)
	m.state = C.llama_allocate_state()
	fmt.Printf("Loading model %s...\n", m.path)
	result := C.llama_bootstrap(modelPath, m.state, C.int(m.nctx), C.int(m.nParts))
	if result != 0 {
		return errors.New("Bootstrap got error")
	}
	fmt.Printf("Model loaded successfully.\n")
	return nil
}

func (m *GGMLModel) Predict(params PredictParams, text string, cb WordCallbackFn) (FinishReason, error) {
	h := cgo.NewHandle(cb)
	input := C.CString(text)
	pparams := C.llama_allocate_params(input,
		C.int(params.Seed),
		C.int(m.threads),
		C.int(params.Tokens),
		C.int(params.TopK),
		C.float(params.TopP),
		C.float(params.Temp),
		C.float(params.RepeatPenalty),
		C.int(params.RepeatLastN),
		C.int(params.NBatch),
	)
	defer func() {
		C.llama_free_params(pparams)
	}()
	result := C.llama_predict(pparams, m.state, C.uintptr_t(h))
	switch result {
	case 0:
		return PROMPT_STOP, nil
	case 1:
		return PROMPT_ERR, errors.New("Predicting failed")
	case 2:
		return PROMPT_FINISH, nil
	}
	return PROMPT_ERR, errors.New("Unknown result")
}

func (m *GGMLModel) TokenizePrompt(prompt string) []string {
	ret := []string{}
	cb := func(word string) {
		ret = append(ret, word)
	}
	h := cgo.NewHandle(WordCallbackFn(cb))
	input := C.CString(prompt)
	C.llama_tokenize_prompt(m.state, input, C.uintptr_t(h))
	return ret
}
