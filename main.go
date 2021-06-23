package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"text/template"
	"time"

	"cloud.google.com/go/compute/metadata"
)

//Index holds fields displayed on the index.html template
type Index struct {
	Hostname string
	Tags     []string
}

type key int

const (
	requestIDKey key = 0
)

var (
	listenAddr string
	healthy    int32
)

//go:embed static
var staticFiles embed.FS

//go:embed templates/index.html
var indexFile string

//go:embed templates/err.html
var errFile string

// This example demonstrates how to use your own transport when using this package.
func main() {
	flag.StringVar(&listenAddr, "listen-addr", ":80", "server listen address")
	flag.Parse()

	logger := log.New(os.Stdout, "gcp-meta: ", log.LstdFlags)
	logger.Println("GCP Metadata reader is starting...")

	fsys, err := fs.Sub(staticFiles, "static")
	if err != nil {
		fmt.Print(err.Error())
	}

	router := http.NewServeMux()
	router.Handle("/", index())
	router.Handle("/healthz", healthz())
	router.Handle("/static/",
		http.StripPrefix("/static/",
			http.FileServer(http.FS(fsys))))

	nextRequestID := func() string {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}

	server := &http.Server{
		Addr:         listenAddr,
		Handler:      tracing(nextRequestID)(logging(logger)(router)),
		ErrorLog:     logger,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	done := make(chan bool)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)

	go func() {
		<-quit
		logger.Println("GCP Metadata reader is shutting down...")
		atomic.StoreInt32(&healthy, 0)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		server.SetKeepAlivesEnabled(false)
		if err := server.Shutdown(ctx); err != nil {
			logger.Fatalf("Could not gracefully shutdown the server: %v\n", err)
		}
		close(done)
	}()

	logger.Println("GCP Metadata reader server is ready to handle requests at", listenAddr)
	atomic.StoreInt32(&healthy, 1)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("Could not listen on %s: %v\n", listenAddr, err)
	}

	<-done
	logger.Panicln("GCP Metadata reader stopped")
	// fmt.Println(http.ListenAndServe(":80", nil))
}

func index() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if metadata.OnGCE() {
			h, err := metadata.Hostname()
			if err != nil {
				fmt.Print(err.Error())
			}
			t, err := metadata.InstanceTags()
			if err != nil {
				fmt.Print(err.Error())
			}

			index := Index{h, t}

			// template := template.Must(template.ParseFiles("templates/index.html"))
			template := template.Must(template.New("index").Parse(indexFile))

			if err := template.ExecuteTemplate(w, "index", index); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		} else {
			template := template.Must(template.New("err").Parse(errFile))
			// template := template.Must(template.ParseFiles("templates/err.html"))

			if err := template.ExecuteTemplate(w, "err", nil); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}
	})
}

func healthz() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.LoadInt32(&healthy) == 1 {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	})
}

func logging(logger *log.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				requestID, ok := r.Context().Value(requestIDKey).(string)
				if !ok {
					requestID = "unknown"
				}
				logger.Println(requestID, r.Method, r.URL.Path, r.RemoteAddr, r.UserAgent())
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func tracing(nextRequestID func() string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get("X-Request-Id")
			if requestID == "" {
				requestID = nextRequestID()
			}
			ctx := context.WithValue(r.Context(), requestIDKey, requestID)
			w.Header().Set("X-Request-Id", requestID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
