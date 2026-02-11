package mcp

import (
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/atakanatali/contextify/internal/memory"
)

type Server struct {
	mcpServer *mcp.Server
	svc       *memory.Service
}

func NewServer(svc *memory.Service) *Server {
	s := &Server{svc: svc}

	s.mcpServer = mcp.NewServer(&mcp.Implementation{
		Name:    "contextify",
		Version: "0.1.0",
	}, nil)

	s.registerTools()

	return s
}

func (s *Server) Handler() http.Handler {
	return mcp.NewStreamableHTTPHandler(func(req *http.Request) *mcp.Server {
		return s.mcpServer
	}, nil)
}

func (s *Server) MCPServer() *mcp.Server {
	return s.mcpServer
}
