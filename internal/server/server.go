package server

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/stockyard-dev/stockyard-brand-standalone/internal/store"
)

type Server struct {
	db       *store.DB
	mux      *http.ServeMux
	port     int
	adminKey string
	client   *http.Client
	limits   Limits
}

func New(db *store.DB, port int, adminKey string, limits Limits) *Server {
	s := &Server{
		db:       db,
		mux:      http.NewServeMux(),
		port:     port,
		adminKey: adminKey,
		client:   &http.Client{Timeout: 10 * time.Second},
		limits:   limits,
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	// Event ingestion — no auth required (use network controls or API key)
	s.mux.HandleFunc("POST /api/events", s.handleAppend)

	// Read + verify — admin key
	s.mux.HandleFunc("GET /api/events", s.admin(s.handleList))
	s.mux.HandleFunc("GET /api/events/{id}", s.admin(s.handleGetEvent))
	s.mux.HandleFunc("GET /api/verify", s.admin(s.handleVerify))

	// Evidence export
	s.mux.HandleFunc("GET /api/evidence/export", s.admin(s.handleExport))

	// Policies
	s.mux.HandleFunc("GET /api/policies", s.admin(s.handleListPolicies))
	s.mux.HandleFunc("POST /api/policies", s.admin(s.handleSavePolicy))
	s.mux.HandleFunc("GET /api/policies/templates", s.admin(s.handlePolicyTemplates))
	s.mux.HandleFunc("POST /api/policies/templates/{framework}", s.admin(s.handleApplyTemplate))

	// Webhooks
	s.mux.HandleFunc("GET /api/webhooks", s.admin(s.handleListWebhooks))
	s.mux.HandleFunc("POST /api/webhooks", s.admin(s.handleCreateWebhook))
	s.mux.HandleFunc("DELETE /api/webhooks/{id}", s.admin(s.handleDeleteWebhook))

	// Stats
	s.mux.HandleFunc("GET /api/stats", s.admin(s.handleStats))
	s.mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"status": "ok"})
	})
}

func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("[brand] listening on %s", addr)
	srv := &http.Server{
		Addr:         addr,
		Handler:      s.mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}
	return srv.ListenAndServe()
}

// ── Auth ──────────────────────────────────────────────────────────────

func (s *Server) admin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.adminKey == "" {
			next(w, r)
			return
		}
		key := r.Header.Get("Authorization")
		key = strings.TrimPrefix(key, "Bearer ")
		if key == "" {
			key = r.URL.Query().Get("key")
		}
		if key != s.adminKey {
			writeJSON(w, 401, map[string]string{"error": "admin key required"})
			return
		}
		next(w, r)
	}
}

func sourceIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return strings.Split(fwd, ",")[0]
	}
	return r.RemoteAddr
}

// ── Event Handlers ────────────────────────────────────────────────────

func (s *Server) handleAppend(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EventType string         `json:"type"`
		Actor     string         `json:"actor"`
		Resource  string         `json:"resource"`
		Detail    map[string]any `json:"detail"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.EventType == "" {
		writeJSON(w, 400, map[string]string{"error": "type is required"})
		return
	}

	// On free tier, warn when approaching monthly event limit (soft limit — self-hosted never hard-blocks)
	if s.limits.MaxEventsPerMonth > 0 {
		monthTotal := s.db.MonthlyEventCount()
		if LimitReached(s.limits.MaxEventsPerMonth, monthTotal) {
			w.Header().Set("X-License-Warning", "free tier event limit reached — upgrade to Pro at https://stockyard.dev/brand/")
		}
	}
	entry, err := s.db.AppendEvent(req.EventType, req.Actor, req.Resource, sourceIP(r), req.Detail)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}

	log.Printf("[brand] event seq=%d type=%s actor=%s", entry.Seq, entry.EventType, entry.Actor)

	// Fire webhooks async
	go s.fireWebhooks(entry)

	writeJSON(w, 201, map[string]any{"entry": entry})
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := 100
	if l := q.Get("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}
	offset := 0
	if o := q.Get("offset"); o != "" {
		fmt.Sscanf(o, "%d", &offset)
	}

	entries, total, err := s.db.ListEvents(store.ListFilter{
		EventType: q.Get("type"),
		Actor:     q.Get("actor"),
		From:      q.Get("from"),
		To:        q.Get("to"),
		Limit:     limit,
		Offset:    offset,
	})
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if entries == nil {
		entries = []store.Entry{}
	}
	writeJSON(w, 200, map[string]any{"entries": entries, "total": total, "count": len(entries)})
}

func (s *Server) handleGetEvent(w http.ResponseWriter, r *http.Request) {
	entry, err := s.db.GetEvent(r.PathValue("id"))
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "event not found"})
		return
	}
	writeJSON(w, 200, map[string]any{"entry": entry})
}

func (s *Server) handleVerify(w http.ResponseWriter, r *http.Request) {
	result := s.db.VerifyChain()
	code := 200
	if !result.Valid {
		code = 409
	}
	writeJSON(w, code, result)
}

// ── Evidence Export ───────────────────────────────────────────────────

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")

	pack, err := s.db.ExportEvidence(from, to)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="evidence-%s.json"`, time.Now().Format("20060102")))
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(pack)
}

// ── Policy Handlers ───────────────────────────────────────────────────

func (s *Server) handleListPolicies(w http.ResponseWriter, r *http.Request) {
	policies, err := s.db.ListPolicies()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if policies == nil {
		policies = []store.Policy{}
	}
	writeJSON(w, 200, map[string]any{"policies": policies})
}

func (s *Server) handleSavePolicy(w http.ResponseWriter, r *http.Request) {
	var p store.Policy
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil || p.Name == "" {
		writeJSON(w, 400, map[string]string{"error": "name required"})
		return
	}
	p.Enabled = true
	if err := s.db.SavePolicy(&p); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 201, map[string]any{"policy": p})
}

func (s *Server) handlePolicyTemplates(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"templates": store.ListPolicyTemplates()})
}

func (s *Server) handleApplyTemplate(w http.ResponseWriter, r *http.Request) {
	framework := r.PathValue("framework")
	tmpl, ok := store.GetPolicyTemplate(framework)
	if !ok {
		writeJSON(w, 404, map[string]string{"error": "unknown framework — use: soc2, hipaa, gdpr, eu_ai_act"})
		return
	}
	tmpl.Enabled = true
	if err := s.db.SavePolicy(tmpl); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 201, map[string]any{"policy": tmpl})
}

// ── Webhook Handlers ──────────────────────────────────────────────────

func (s *Server) handleListWebhooks(w http.ResponseWriter, r *http.Request) {
	whs, err := s.db.ListWebhooks()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if whs == nil {
		whs = []store.Webhook{}
	}
	writeJSON(w, 200, map[string]any{"webhooks": whs})
}

func (s *Server) handleCreateWebhook(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL        string `json:"url"`
		Secret     string `json:"secret"`
		EventTypes string `json:"event_types"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.URL == "" {
		writeJSON(w, 400, map[string]string{"error": "url required"})
		return
	}
	wh, err := s.db.CreateWebhook(req.URL, req.Secret, req.EventTypes)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 201, map[string]any{"webhook": wh})
}

func (s *Server) handleDeleteWebhook(w http.ResponseWriter, r *http.Request) {
	s.db.DeleteWebhook(r.PathValue("id"))
	writeJSON(w, 200, map[string]string{"status": "deleted"})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.db.Stats())
}

// ── Webhook Dispatch ──────────────────────────────────────────────────

func (s *Server) fireWebhooks(entry *store.Entry) {
	hooks, err := s.db.ActiveWebhooks()
	if err != nil || len(hooks) == 0 {
		return
	}

	payload, _ := json.Marshal(map[string]any{
		"event":      "ledger.append",
		"entry":      entry,
		"fired_at":   time.Now().UTC().Format(time.RFC3339),
	})

	for _, wh := range hooks {
		// Check event type filter
		if wh.EventTypes != "*" && wh.EventTypes != "" {
			matched := false
			for _, t := range strings.Split(wh.EventTypes, ",") {
				if strings.TrimSpace(t) == entry.EventType {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		req, err := http.NewRequest("POST", wh.URL, bytes.NewReader(payload))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Brand-Event", entry.EventType)
		req.Header.Set("X-Brand-Seq", fmt.Sprintf("%d", entry.Seq))

		if wh.Secret != "" {
			mac := hmac.New(sha256.New, []byte(wh.Secret))
			mac.Write(payload)
			req.Header.Set("X-Brand-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
		}

		resp, err := s.client.Do(req)
		if err != nil {
			log.Printf("[brand] webhook %s failed: %v", wh.URL, err)
			continue
		}
		resp.Body.Close()
		log.Printf("[brand] webhook %s → %d", wh.URL, resp.StatusCode)
	}
}

// ── Helpers ───────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
