package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/matbits/counter/pkg/fhandler"
	"github.com/matbits/counter/pkg/lockfile"
)

var (
	fileName   string
	listenAddr string
	number     float64
	lock       sync.RWMutex
)

func init() {
	flag.StringVar(&fileName, "file", "counter.txt", "path to counter storage file")
	flag.StringVar(&listenAddr, "listen", ":8080", "[ip]:port to listen")
}

func main() {
	flag.Parse()

	lockFile := filepath.Join(os.TempDir(), "counter.lock")
	flock := lockfile.NewFcntlLockfile(lockFile)

	err := flock.LockWrite()
	if err != nil {
		log.Printf("unable to get lock '%s': %s", lockFile, err)
		os.Exit(1)
	}

	defer flock.Unlock()

	if listenAddr == "" || fileName == "" {
		log.Println("invalid address or file")
		os.Exit(1)
	}

	err = createFile(fileName)
	if err != nil {
		log.Printf("unable to create file '%s': %s", fileName, err)
		os.Exit(1)
	}

	counterContent, err := os.ReadFile(fileName)
	if err != nil {
		log.Printf("unable to read file '%s': %s", fileName, err)
		os.Exit(1)
	}

	err = json.Unmarshal(counterContent, &number)
	if err != nil {
		log.Printf("unable to unmarshal counter: %s", err)
		os.Exit(1)
	}

	http.HandleFunc("/hostname", hostname)
	http.HandleFunc("/latest", latestCounter)

	server := &http.Server{Addr: listenAddr}

	addr := server.Addr
	if addr == "" {
		addr = ":http"
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Println(err.Error())

		return
	}

	defer ln.Close()

	interChan := make(chan os.Signal, 2)
	signal.Notify(interChan, os.Interrupt, syscall.SIGTERM) // subscribe to system signals

	go shutdown(server, interChan)

	log.Println("server running")

	err = server.Serve(ln)
	if err != nil {
		if !errors.Is(err, http.ErrServerClosed) {
			log.Printf("unable to handle: %s", err)
			os.Exit(1)
		}
	}
}

func latestCounter(w http.ResponseWriter, r *http.Request) {
	lock.RLock()
	defer lock.RUnlock()

	_, err := w.Write([]byte(fmt.Sprintf("%d", int(number))))
	if err != nil {
		log.Printf("unable to number: %s", err)
	}
}

func hostname(w http.ResponseWriter, r *http.Request) {
	lock.Lock()
	defer lock.Unlock()

	number++

	out, err := json.Marshal(number)
	if err != nil {
		number--

		log.Printf("unable to marshal counter: %s", err)
		w.WriteHeader(http.StatusServiceUnavailable)

		return
	}

	err = fhandler.WriteAtomicTmpDir("counter", fileName, out, 0644)
	if err != nil {
		number--

		log.Printf("unable to write file '%s': %s", fileName, err)
		w.WriteHeader(http.StatusServiceUnavailable)

		return
	}

	_, err = w.Write([]byte(fmt.Sprintf("%d", int(number))))
	if err != nil {
		log.Printf("unable to number: %s", err)
	}
}

func createFile(fileName string) error {
	_, err := os.Stat(fileName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return initFile(fileName)
		}

		return err
	}

	return nil
}

func initFile(fileName string) error {
	out, err := json.Marshal(number)
	if err != nil {
		return err
	}

	return fhandler.WriteAtomicTmpDir("counter", fileName, out, 0644)
}

func shutdown(server *http.Server, c chan os.Signal) {
	<-c

	ctx, cancal := context.WithTimeout(context.Background(), time.Minute)
	defer cancal()

	err := server.Shutdown(ctx)
	if err != nil {
		log.Printf("unable to shutdown server: %s", err)
	}
}
