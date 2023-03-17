package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	var (
		modelPath  string
		listenAddr string
		threads    int
		seed       int
		nctx       int
	)
	flags := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flags.StringVar(&modelPath, "m", "", "path to q4_0.bin model file to load")
	flags.StringVar(&listenAddr, "l", "127.0.0.1:4000", "Listen address")
	flags.IntVar(&threads, "t", 4, "Number of threads to use during computation")
	flags.IntVar(&seed, "s", -1, "seed")
	flags.IntVar(&nctx, "c", 512, "context size")
	err := flags.Parse(os.Args[1:])
	if err != nil {
		panic(err)
	}

	model := NewGGMLModel(modelPath, nctx, threads)
	err = model.Load()
	if err != nil {
		panic(err)
	}
	info := model.SystemInfo()
	fmt.Println(info)

	srv := APIServer{
		Seed:   seed,
		Model:  model,
		Listen: listenAddr,
	}
	srv.Run()
}
