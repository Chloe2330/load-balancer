package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
)

type Backend struct {
	URL          *url.URL
	Alive        bool
	mux          sync.RWMutex
	ReverseProxy *httputil.ReverseProxy
}

type ServerPool struct {
	backends []*Backend
	current  uint64
}

func (s *ServerPool) NextIndex() int {
	nextIndex := atomic.AddUint64(&s.current, uint64(1))
	return int(nextIndex % uint64(len(s.backends)))
}

func (s *ServerPool) GetNextBackend() *Backend {
	next := s.NextIndex()
	length := len(s.backends) + next

	// cyclic traversal of backends slice
	for i := next; i < length; i++ {
		index := i % len(s.backends)
		// mark working backend as current one
		if s.backends[index].IsAlive() {
			if i != next {
				atomic.StoreUint64(&s.current, uint64(index))
			}
			return s.backends[index]
		}
	}
	return nil
}

func (b *Backend) IsAlive() bool { // read operation
	b.mux.RLock()
	alive := b.Alive
	b.mux.RUnlock()
	return alive
}

func (b *Backend) SetAlive(alive bool) { // write operation
	b.mux.Lock()
	b.Alive = alive
	b.mux.Unlock()
}

// load balancer logic (round-robin)
func lb(w http.ResponseWriter, r *http.Request) {
	peer := servers.GetNextBackend()
	if peer != nil {
		peer.ReverseProxy.ServeHTTP(w, r)
		return
	}
	http.Error(w, "Service not available", http.StatusServiceUnavailable)
}

var servers ServerPool

func main() {
	var port int

	// create http server
	server := http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: http.HandlerFunc(lb),
	}

	log.Printf("Load Balancer started at :%d\n", port)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
