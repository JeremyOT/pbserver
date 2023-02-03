package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/JeremyOT/httpserver"
)

type Service struct {
	*httpserver.Server
}

func writePB(r io.Reader) error {
	pbcopy := exec.Command("pbcopy")
	pbcopy.Stdin = r
	return pbcopy.Run()
}

func readPB(w io.Writer) error {
	pbpaste := exec.Command("pbpaste")
	pbpaste.Stdout = w
	return pbpaste.Run()
}

func (s *Service) handleRequest(writer http.ResponseWriter, request *http.Request) {
	defer request.Body.Close()
	log.Printf("Method: %v Path: %v\n", request.Method, request.URL.Path)
	if request.URL.Path != "/pb" {
		io.WriteString(writer, "PUT or GET to /pb\n")
		return
	}
	switch request.Method {
	case http.MethodGet:
		if err := readPB(writer); err != nil {
			log.Printf("Error reading PB: %v", err)
		}
	case http.MethodPut, http.MethodPost:
		writePB(request.Body)
	default:
		http.Error(writer, fmt.Sprintf("Method not allowed: %s", request.Method), http.StatusMethodNotAllowed)
	}
}

var (
	address = flag.String("address", "127.0.0.1:8042", "The address to bind to")
	logFile = flag.String("log", "", "Write logs to this file")
)

func monitorSignal(s *Service, sigChan <-chan os.Signal) {
	sig := <-sigChan
	log.Printf("Exiting (%s)...", sig)
	select {
	case <-s.Stop():
		return
	case <-sigChan:
		log.Printf("Force quitting (%s)...", sig)
		os.Exit(-1)
	}
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	flag.Parse()

	if *logFile != "" {
		f, err := os.Create(*logFile)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		log.SetOutput(f)
	}

	service := &Service{}
	service.Server = httpserver.New(service.handleRequest)
	service.Start(*address)
	tcpAddr, ok := service.Address().(*net.TCPAddr)
	if !ok {
		log.Fatal("Failed to bind")
	}
	log.Printf("Listening on http://%v", tcpAddr)
	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGQUIT)
	go monitorSignal(service, sigChan)
	<-service.Wait()
}
