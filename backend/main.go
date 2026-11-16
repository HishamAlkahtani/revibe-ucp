package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

func main() {
	// Keep application logs on stdout so ordinary shell redirection works.
	log.SetOutput(os.Stdout)
	gin.DefaultWriter = os.Stdout
	gin.DefaultErrorWriter = os.Stdout
	if err := loadEnv("../.env"); err != nil {
		log.Fatal(err)
	}
	api := &API{client: &http.Client{Timeout: 45 * time.Second}}
	commerce := &Commerce{api: api, token: os.Getenv("UCP_ACCESS_TOKEN"), buyerIP: os.Getenv("UCP_BUYER_IP"), profile: env("UCP_AGENT_PROFILE_URL", "https://shopify.dev/ucp/agent-profiles/examples/2026-08-25/valid-with-capabilities.json")}
	agent := &Agent{api: api, commerce: commerce, key: os.Getenv("OPENAI_API_KEY"), model: env("OPENAI_MODEL", "gpt-4.1-mini"), endpoint: "https://api.openai.com/v1/responses"}
	server := &Server{agent: agent, sessions: newSessionStore()}
	gin.SetMode(gin.ReleaseMode)
	log.Print("Go backend: http://127.0.0.1:8080")
	log.Printf("OpenAI model=%s key_configured=%t; press Ctrl+C to stop", agent.model, agent.key != "")
	log.Printf("UCP resource=cart token_configured=%t tool_payloads=%t", commerce.token != "", os.Getenv("LOG_TOOL_PAYLOADS") != "false")
	log.Fatal((&http.Server{Addr: "127.0.0.1:8080", Handler: server.routes(), ReadHeaderTimeout: 5 * time.Second}).ListenAndServe())
}
