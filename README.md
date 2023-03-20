# llama-go

Inference of [Facebook's LLaMA](https://github.com/facebookresearch/llama) model in Golang with embedded C/C++. And provide a RESTful API for prompt completion. It also provide a simple web UI for interact with.

## Description

This project embeds the work of [llama.cpp](https://github.com/ggerganov/llama.cpp) in a Golang binary.
The main goal is to run the model using 4-bit quantization using CPU on Consumer-Grade hardware.

At startup, the model is loaded and a prompt is offered to enter a prompt,
after the results have been printed another prompt can be entered.
The program can be quit using ctrl+c.

This project was tested on Linux but should be able to get to work on macOS as well.

## Requirements

The memory requirements for the models are approximately:

```
7B  -> 4 GB (1 file)
13B -> 8 GB (2 files)
30B -> 16 GB (4 files)
65B -> 32 GB (8 files)
```

## Installation

```bash
# build this repo
git clone https://github.com/cornelk/llama-go
cd llama-go
make

# install Python dependencies
python3 -m pip install torch numpy sentencepiece
```

Obtain the original LLaMA model weights and place them in ./models - 
for example by using the https://github.com/shawwn/llama-dl script to download them.

Use the following steps to convert the LLaMA-7B model to a format that is compatible:

```bash
ls ./models
65B 30B 13B 7B tokenizer_checklist.chk tokenizer.model

# convert the 7B model to ggml FP16 format
python3 convert-pth-to-ggml.py models/7B/ 1

# quantize the model to 4-bits
./quantize.sh 7B
```

When running the larger models, make sure you have enough disk space to store all the intermediate files.

## Usage

```bash
./llama-go -h
Usage of ./llama-go:
  -M string
    	process mode (master|worker) (default "master")
  -S string
    	worker listen socket file
  -c int
    	context size (default 512)
  -l string
    	Listen address (default "127.0.0.1:4000")
  -m string
    	path to q4_0.bin model file to load
  -s int
    	seed (default -1)
  -t int
    	Number of threads to use during computation (default 4)
  -w int
    	Number workers (default 2)


./llama-go -m ./models/13B/ggml-model-q4_0.bin
```

As llama.cpp do not support process miltiple requests in one process so we provide a multi-process mode to support parallel request process. `-w` will set the number worker process to be started. And the `-M` and `-S` parameter is handled by multi-process system, so user should not take care about it.

## HTTP API
#### /api/completion
* POST
* Request Parameter: type is json.

	```
	{
		"prompt": string,
		"tokens": int,
		"top_k": int,
		"top_p": float,
		"temp": float,
		"repeat_penalty": float,
		"repeat_lastn": int,
	}
	```

	* prompt: required, prompt text.
	* tokens: required, number tokens generated.
	* top\_k: optional, default 40
	* top\_p: optional, default 0.9
	* temp: optional, default 0.8
	* repeat\_penalty: optional, default 1.3
	* repeat\_lastn: optional, default 64

* Response: type is json.

	```
	{
		"Prompt": string,
		"Text": string,
		"Tokens": int,
		"CompleteReason": string,
	}
	```
