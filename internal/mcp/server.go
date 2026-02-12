package mcp

import (
	"context"
	"net/http"

	"github.com/google/uuid"
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
	}, &mcp.ServerOptions{
		Instructions: `Contextify is a shared memory system for AI agents. Follow these rules:
1. At session start, call get_context with project_id set to the current working directory path
2. When you fix a bug, discover a pattern, or make a decision, call store_memory
3. When you encounter an error or start a new task, call recall_memories first
4. Always set agent_source to identify yourself (e.g. "claude-code", "cursor", "gemini")
5. Set project_id to the current project/workspace path for scoped memories (automatically normalized by server to canonical project name)
6. Use importance 0.8+ for critical/permanent knowledge, 0.5-0.7 for standard`,
	})

	s.registerTools()

	return s
}

func (s *Server) Handler() http.Handler {
	base := mcp.NewStreamableHTTPHandler(func(req *http.Request) *mcp.Server {
		return s.mcpServer
	}, nil)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" {
			reqID = uuid.New().String()[:8]
		}
		w.Header().Set("X-Request-ID", reqID)

		ctx := context.WithValue(r.Context(), "request_id", reqID)
		if sessionID := r.Header.Get("X-Session-ID"); sessionID != "" {
			w.Header().Set("X-Session-ID", sessionID)
			ctx = context.WithValue(ctx, "session_id", sessionID)
		}

		base.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) MCPServer() *mcp.Server {
	return s.mcpServer
}
