package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type service struct{ client *realtimeClient }

func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func failure(w http.ResponseWriter, err error) {
	var rejected rejection
	if errors.As(err, &rejected) && rejected.Status >= 400 && rejected.Status < 500 {
		respond(w, rejected.Status, map[string]string{"error": rejected.Detail.Message, "code": rejected.Detail.Code})
		return
	}
	log.Printf("request failed: %v", err)
	respond(w, http.StatusBadGateway, map[string]string{"error": "upstream request failed"})
}

func decode(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20)).Decode(v)
}

func (s service) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /announcements", func(w http.ResponseWriter, r *http.Request) {
		var a announcement
		if err := decode(r, &a); err != nil {
			respond(w, 400, map[string]string{"error": "invalid JSON"})
			return
		}
		if err := a.validate(); err != nil {
			respond(w, 400, map[string]string{"error": err.Error()})
			return
		}
		if err := s.client.publish(r.Context(), "members.announcement", a, "announcement:"+a.ID); err != nil {
			failure(w, err)
			return
		}
		respond(w, 202, map[string]string{"id": a.ID, "event": "members.announcement"})
	})
	mux.HandleFunc("POST /orders/updates", func(w http.ResponseWriter, r *http.Request) {
		var u orderUpdate
		if err := decode(r, &u); err != nil {
			respond(w, 400, map[string]string{"error": "invalid JSON"})
			return
		}
		event, err := u.event()
		if err != nil {
			respond(w, 400, map[string]string{"error": err.Error()})
			return
		}
		if err := s.client.publish(r.Context(), event, u, "order:"+u.ID); err != nil {
			failure(w, err)
			return
		}
		respond(w, 202, map[string]string{"id": u.ID, "event": event})
	})
	mux.HandleFunc("POST /members/token", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			MemberID string `json:"member_id"`
		}
		if err := decode(r, &input); err != nil || input.MemberID == "" {
			respond(w, 400, map[string]string{"error": "member_id is required"})
			return
		}
		data, err := s.client.memberToken(r.Context(), input.MemberID)
		if err != nil {
			failure(w, err)
			return
		}
		respond(w, 200, data)
	})
	return mux
}

func main() {
	client, err := newRealtimeClient()
	if err != nil {
		log.Fatal(err)
	}
	if err := client.createChannel(context.Background()); err != nil {
		log.Fatal(err)
	}
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	server := &http.Server{Addr: addr, Handler: service{client: client}.routes()}
	stopContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- server.ListenAndServe()
	}()
	log.Printf("member service listening on %s", addr)
	select {
	case <-stopContext.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := server.Shutdown(shutdownContext); err != nil {
			log.Printf("server shutdown failed: %v", err)
		}
		cancel()
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("server failed: %v", err)
		}
	}
	cleanupContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := client.deleteChannel(cleanupContext); err != nil {
		log.Fatalf("delete channel: %v", err)
	}
}
