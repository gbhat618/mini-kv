package console

import (
	"fmt"
	"net/http"
)

type Server struct {
	hub  *Hub
	mux  *http.ServeMux
	port string
}

func NewServer(port string) *Server {
	s := &Server{
		mux:  http.NewServeMux(),
		port: port,
	}

	s.mux.HandleFunc("/ws", s.handleWebSocket)
	s.mux.Handle("/", ServeUI(0))

	return s
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	if s.hub != nil {
		s.hub.HandleWebSocket(w, r)
	}
}

func (s *Server) SetHub(hub *Hub) {
	s.hub = hub
}

func (s *Server) Start() error {
	if s.hub != nil {
		go s.hub.Run()
	}
	addr := fmt.Sprintf(":%s", s.port)
	return http.ListenAndServe(addr, s.mux)
}

func (s *Server) GetHub() *Hub {
	return s.hub
}
