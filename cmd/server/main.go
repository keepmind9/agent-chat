package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/keepmind9/agent-chat/internal/server"
	"github.com/keepmind9/agent-chat/internal/store"
)

func main() {
	port := flag.String("port", "8080", "server port")
	dbPath := flag.String("db", "agent-chat.db", "SQLite database path")
	flag.Parse()

	s, err := store.Open(*dbPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer s.Close()

	hub := server.NewHub()
	go hub.Run()

	h := server.NewHandler(s, hub)

	r := gin.Default()

	// API endpoints
	r.POST("/api/register", gin.WrapF(h.HandleRegister))
	r.POST("/api/send", gin.WrapF(h.HandleSend))
	r.POST("/api/send-group", gin.WrapF(h.HandleSend))
	r.GET("/api/messages", gin.WrapF(h.HandleGetMessages))
	r.GET("/api/messages/recent", gin.WrapF(h.HandleRecentMessages))
	r.POST("/api/messages/read", gin.WrapF(h.HandleMarkRead))
	r.GET("/api/agents", gin.WrapF(h.HandleListAgents))
	r.GET("/api/groups", gin.WrapF(h.HandleListGroups))

	// WebSocket
	r.GET("/ws", gin.WrapF(h.HandleWebSocket))

	// Web dashboard static files
	r.StaticFS("/web", http.Dir("./web"))
	r.GET("/", func(c *gin.Context) { c.Redirect(http.StatusFound, "/web/") })

	log.Printf("agent-chat server starting on :%s", *port)
	if err := r.Run(":" + *port); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
