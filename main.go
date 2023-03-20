package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

func getExecutePath() string {
	ex, err := os.Executable()
	if err == nil {
		return filepath.Dir(ex)
	}

	exReal, err := filepath.EvalSymlinks(ex)
	if err != nil {
		log.Panic(err)
	}
	return filepath.Dir(exReal)
}

func main() {
	var (
		modelPath  string
		listenAddr string
		threads    int
		seed       int
		nctx       int
		mode       string
		sockFile   string
		workers    int
	)
	flags := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flags.StringVar(&modelPath, "m", "", "path to q4_0.bin model file to load")
	flags.StringVar(&listenAddr, "l", "127.0.0.1:4000", "Listen address")
	flags.StringVar(&mode, "M", "master", "process mode (master|worker)")
	flags.StringVar(&sockFile, "S", "", "worker listen socket file")
	flags.IntVar(&threads, "t", 4, "Number of threads to use during computation")
	flags.IntVar(&seed, "s", -1, "seed")
	flags.IntVar(&nctx, "c", 512, "context size")
	flags.IntVar(&workers, "w", 2, "Number workers")
	err := flags.Parse(os.Args[1:])
	if err != nil {
		panic(err)
	}

	execFile, err := os.Executable()
	if err != nil {
		panic(err)
	}

	staticPath := getExecutePath() + "/static"

	if modelPath == "" {
		fmt.Println("Require model path")
		return
	}

	switch mode {
	case "worker":
		runWorkerMode(sockFile, modelPath, threads, seed, nctx)
	case "master":
		runMasterMode(execFile, listenAddr, staticPath, workers, modelPath, threads, seed, nctx)
	}
}

func runWorkerMode(sockFile string, modelPath string, threads int, seed int, nctx int) {
	model := NewGGMLModel(modelPath, nctx, threads)
	err := model.Load()
	if err != nil {
		log.Println("Cannot Load Model:", err)
		os.Exit(1)
	}
	worker := NewWorker(model, sockFile)
	err = worker.Run()
	if err != nil {
		log.Println("Cannot Run worker:", err)
		time.Sleep(2)
		os.Exit(1)
	}
}

func runMasterMode(execFile string, listenAddr string, staticPath string, workers int, modelPath string, threads int, seed int, nctx int) {
	wm := NewWorkerManager(execFile, modelPath, workers, nctx, threads)
	wm.StartWorkers()

	info := SystemInfo()
	fmt.Println(info)

	srv := APIServer{
		Seed:       seed,
		WorkerMgr:  wm,
		Listen:     listenAddr,
		StaticPath: staticPath,
	}
	srv.Run()
}
