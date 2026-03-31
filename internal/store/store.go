package store

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct{ conn *sql.DB }

func Open(dataDir string) (*DB, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	conn, err := sql.Open("sqlite", filepath.Join(dataDir, "brand.db"))
	if err != nil {
		return nil, err
	}
	conn.Exec("PRAGMA journal_mode=WAL")
	conn.Exec("PRAGMA busy_timeout=5000")
	conn.SetMaxOpenConns(1)
	db := &DB{conn: conn}
	return db, db.migrate()
}

func (db *DB) Close() error { return db.conn.Close() }

func (db *DB) migrate() error {
	_, err := db.conn.Exec(`
CREATE TABLE IF NOT EXISTS ledger (
    id          TEXT PRIMARY KEY,
    seq         INTEGER UNIQUE NOT NULL,
    event_type  TEXT NOT NULL,
    actor       TEXT DEFAULT '',
    resource    TEXT DEFAULT '',
    detail_json TEXT DEFAULT '{}',
    source_ip   TEXT DEFAULT '',
    prev_hash   TEXT NOT NULL,
    entry_hash  TEXT UNIQUE NOT NULL,
    created_at  TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_ledger_seq   ON ledger(seq);
CREATE INDEX IF NOT EXISTS idx_ledger_type  ON ledger(event_type);
CREATE INDEX IF NOT EXISTS idx_ledger_actor ON ledger(actor);
CREATE INDEX IF NOT EXISTS idx_ledger_time  ON ledger(created_at);

CREATE TABLE IF NOT EXISTS policies (
    id          TEXT PRIMARY KEY,
    name        TEXT UNIQUE NOT NULL,
    framework   TEXT DEFAULT '',
    rules_json  TEXT DEFAULT '[]',
    enabled     INTEGER DEFAULT 1,
    created_at  TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS webhooks (
    id          TEXT PRIMARY KEY,
    url         TEXT NOT NULL,
    secret      TEXT DEFAULT '',
    event_types TEXT DEFAULT '*',
    enabled     INTEGER DEFAULT 1,
    created_at  TEXT DEFAULT (datetime('now'))
);
`)
	return err
}

// ── Ledger ────────────────────────────────────────────────────────────

type Entry struct {
	ID         string         `json:"id"`
	Seq        int64          `json:"seq"`
	EventType  string         `json:"event_type"`
	Actor      string         `json:"actor"`
	Resource   string         `json:"resource"`
	Detail     map[string]any `json:"detail"`
	SourceIP   string         `json:"source_ip"`
	PrevHash   string         `json:"prev_hash"`
	EntryHash  string         `json:"entry_hash"`
	CreatedAt  string         `json:"created_at"`
}

func (db *DB) AppendEvent(eventType, actor, resource, sourceIP string, detail map[string]any) (*Entry, error) {
	if detail == nil {
		detail = map[string]any{}
	}
	detailJSON, _ := json.Marshal(detail)

	// Get latest hash for chaining — single writer, no race
	var prevHash string
	var prevSeq int64
	err := db.conn.QueryRow("SELECT seq, entry_hash FROM ledger ORDER BY seq DESC LIMIT 1").
		Scan(&prevSeq, &prevHash)
	if errors.Is(err, sql.ErrNoRows) {
		prevHash = "genesis"
		prevSeq = 0
	} else if err != nil {
		return nil, err
	}

	seq := prevSeq + 1
	id := "evt_" + genID(10)
	now := time.Now().UTC().Format(time.RFC3339)

	// Compute entry hash: SHA-256(seq + type + actor + resource + detail + prevHash + timestamp)
	raw := fmt.Sprintf("%d|%s|%s|%s|%s|%s|%s", seq, eventType, actor, resource, string(detailJSON), prevHash, now)
	h := sha256.Sum256([]byte(raw))
	entryHash := hex.EncodeToString(h[:])

	_, err = db.conn.Exec(
		`INSERT INTO ledger (id,seq,event_type,actor,resource,detail_json,source_ip,prev_hash,entry_hash,created_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?)`,
		id, seq, eventType, actor, resource, string(detailJSON), sourceIP, prevHash, entryHash, now)
	if err != nil {
		return nil, err
	}

	return &Entry{
		ID: id, Seq: seq, EventType: eventType, Actor: actor,
		Resource: resource, Detail: detail, SourceIP: sourceIP,
		PrevHash: prevHash, EntryHash: entryHash, CreatedAt: now,
	}, nil
}

type ListFilter struct {
	EventType string
	Actor     string
	From      string
	To        string
	Limit     int
	Offset    int
}

func (db *DB) ListEvents(f ListFilter) ([]Entry, int, error) {
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 100
	}

	where := "1=1"
	args := []any{}
	if f.EventType != "" {
		where += " AND event_type=?"
		args = append(args, f.EventType)
	}
	if f.Actor != "" {
		where += " AND actor=?"
		args = append(args, f.Actor)
	}
	if f.From != "" {
		where += " AND created_at >= ?"
		args = append(args, f.From)
	}
	if f.To != "" {
		where += " AND created_at <= ?"
		args = append(args, f.To)
	}

	var total int
	db.conn.QueryRow("SELECT COUNT(*) FROM ledger WHERE "+where, args...).Scan(&total)

	args = append(args, f.Limit, f.Offset)
	rows, err := db.conn.Query(
		`SELECT id,seq,event_type,actor,resource,detail_json,source_ip,prev_hash,entry_hash,created_at
		 FROM ledger WHERE `+where+` ORDER BY seq DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []Entry
	for rows.Next() {
		var e Entry
		var detailRaw string
		rows.Scan(&e.ID, &e.Seq, &e.EventType, &e.Actor, &e.Resource,
			&detailRaw, &e.SourceIP, &e.PrevHash, &e.EntryHash, &e.CreatedAt)
		json.Unmarshal([]byte(detailRaw), &e.Detail)
		out = append(out, e)
	}
	return out, total, rows.Err()
}

func (db *DB) GetEvent(id string) (*Entry, error) {
	var e Entry
	var detailRaw string
	err := db.conn.QueryRow(
		`SELECT id,seq,event_type,actor,resource,detail_json,source_ip,prev_hash,entry_hash,created_at
		 FROM ledger WHERE id=?`, id).
		Scan(&e.ID, &e.Seq, &e.EventType, &e.Actor, &e.Resource,
			&detailRaw, &e.SourceIP, &e.PrevHash, &e.EntryHash, &e.CreatedAt)
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(detailRaw), &e.Detail)
	return &e, nil
}

// ── Chain Verification ────────────────────────────────────────────────

type VerifyResult struct {
	Valid        bool   `json:"valid"`
	TotalEntries int64  `json:"total_entries"`
	Checked      int64  `json:"checked"`
	BrokenAt     *int64 `json:"broken_at,omitempty"`
	Message      string `json:"message"`
}

func (db *DB) VerifyChain() VerifyResult {
	rows, err := db.conn.Query(
		`SELECT seq,event_type,actor,resource,detail_json,prev_hash,entry_hash,created_at
		 FROM ledger ORDER BY seq ASC`)
	if err != nil {
		return VerifyResult{Message: err.Error()}
	}
	defer rows.Close()

	var total int64
	db.conn.QueryRow("SELECT COUNT(*) FROM ledger").Scan(&total)

	var checked int64
	prevHash := "genesis"

	for rows.Next() {
		var seq int64
		var eventType, actor, resource, detailJSON, storedPrevHash, storedHash, createdAt string
		rows.Scan(&seq, &eventType, &actor, &resource, &detailJSON, &storedPrevHash, &storedHash, &createdAt)

		// Verify prev_hash linkage
		if storedPrevHash != prevHash {
			return VerifyResult{Valid: false, TotalEntries: total, Checked: checked, BrokenAt: &seq,
				Message: fmt.Sprintf("chain broken at seq %d: prev_hash mismatch", seq)}
		}

		// Recompute entry hash
		raw := fmt.Sprintf("%d|%s|%s|%s|%s|%s|%s", seq, eventType, actor, resource, detailJSON, prevHash, createdAt)
		h := sha256.Sum256([]byte(raw))
		expected := hex.EncodeToString(h[:])
		if expected != storedHash {
			return VerifyResult{Valid: false, TotalEntries: total, Checked: checked, BrokenAt: &seq,
				Message: fmt.Sprintf("chain broken at seq %d: entry_hash mismatch", seq)}
		}

		prevHash = storedHash
		checked++
	}

	return VerifyResult{Valid: true, TotalEntries: total, Checked: checked,
		Message: fmt.Sprintf("chain intact — %d entries verified", checked)}
}

// ── Evidence Export ───────────────────────────────────────────────────

type EvidencePack struct {
	ExportedAt string  `json:"exported_at"`
	From       string  `json:"from"`
	To         string  `json:"to"`
	EntryCount int     `json:"entry_count"`
	HeadHash   string  `json:"head_hash"`
	ChainValid bool    `json:"chain_valid"`
	Entries    []Entry `json:"entries"`
}

func (db *DB) ExportEvidence(from, to string) (*EvidencePack, error) {
	entries, _, err := db.ListEvents(ListFilter{From: from, To: to, Limit: 10000})
	if err != nil {
		return nil, err
	}

	headHash := "genesis"
	if len(entries) > 0 {
		// entries are DESC — head is entries[0]
		headHash = entries[0].EntryHash
	}

	v := db.VerifyChain()
	return &EvidencePack{
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		From:       from, To: to,
		EntryCount: len(entries),
		HeadHash:   headHash,
		ChainValid: v.Valid,
		Entries:    entries,
	}, nil
}

// ── Policies ──────────────────────────────────────────────────────────

type Policy struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Framework string         `json:"framework"`
	Rules     []PolicyRule   `json:"rules"`
	Enabled   bool           `json:"enabled"`
	CreatedAt string         `json:"created_at"`
}

type PolicyRule struct {
	EventType   string `json:"event_type"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

var policyTemplates = map[string]*Policy{
	"soc2": {
		Name: "SOC2 Type II", Framework: "soc2",
		Rules: []PolicyRule{
			{EventType: "user_login", Description: "Log all authentication events", Required: true},
			{EventType: "user_logout", Description: "Log all session terminations", Required: true},
			{EventType: "permission_change", Description: "Log all access control changes", Required: true},
			{EventType: "data_export", Description: "Log all data exports", Required: true},
			{EventType: "config_change", Description: "Log all configuration changes", Required: true},
		},
	},
	"hipaa": {
		Name: "HIPAA Audit Controls", Framework: "hipaa",
		Rules: []PolicyRule{
			{EventType: "phi_access", Description: "Log all PHI access", Required: true},
			{EventType: "phi_modify", Description: "Log all PHI modifications", Required: true},
			{EventType: "phi_export", Description: "Log all PHI exports", Required: true},
			{EventType: "user_login", Description: "Log all authentication events", Required: true},
			{EventType: "permission_change", Description: "Log all access control changes", Required: true},
		},
	},
	"gdpr": {
		Name: "GDPR Article 30", Framework: "gdpr",
		Rules: []PolicyRule{
			{EventType: "data_access", Description: "Log all personal data access", Required: true},
			{EventType: "data_export", Description: "Log all data exports", Required: true},
			{EventType: "data_delete", Description: "Log all erasure requests", Required: true},
			{EventType: "consent_change", Description: "Log all consent modifications", Required: true},
		},
	},
	"eu_ai_act": {
		Name: "EU AI Act Logging", Framework: "eu_ai_act",
		Rules: []PolicyRule{
			{EventType: "ai_inference", Description: "Log all AI model inference calls", Required: true},
			{EventType: "ai_decision", Description: "Log all automated decisions", Required: true},
			{EventType: "model_deploy", Description: "Log all model deployments", Required: true},
			{EventType: "human_override", Description: "Log all human intervention events", Required: true},
		},
	},
}

func GetPolicyTemplate(framework string) (*Policy, bool) {
	p, ok := policyTemplates[framework]
	return p, ok
}

func ListPolicyTemplates() []map[string]string {
	out := []map[string]string{}
	for k, v := range policyTemplates {
		out = append(out, map[string]string{"id": k, "name": v.Name, "framework": v.Framework})
	}
	return out
}

func (db *DB) SavePolicy(p *Policy) error {
	rulesJSON, _ := json.Marshal(p.Rules)
	if p.ID == "" {
		p.ID = "pol_" + genID(8)
	}
	en := 0
	if p.Enabled {
		en = 1
	}
	_, err := db.conn.Exec(
		`INSERT OR REPLACE INTO policies (id,name,framework,rules_json,enabled) VALUES (?,?,?,?,?)`,
		p.ID, p.Name, p.Framework, string(rulesJSON), en)
	return err
}

func (db *DB) ListPolicies() ([]Policy, error) {
	rows, err := db.conn.Query("SELECT id,name,framework,rules_json,enabled,created_at FROM policies WHERE enabled=1")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Policy
	for rows.Next() {
		var p Policy
		var rulesJSON string
		var en int
		rows.Scan(&p.ID, &p.Name, &p.Framework, &rulesJSON, &en, &p.CreatedAt)
		json.Unmarshal([]byte(rulesJSON), &p.Rules)
		p.Enabled = en == 1
		out = append(out, p)
	}
	return out, rows.Err()
}

// ── Webhooks ──────────────────────────────────────────────────────────

type Webhook struct {
	ID         string `json:"id"`
	URL        string `json:"url"`
	Secret     string `json:"secret,omitempty"`
	EventTypes string `json:"event_types"`
	Enabled    bool   `json:"enabled"`
	CreatedAt  string `json:"created_at"`
}

func (db *DB) CreateWebhook(url, secret, eventTypes string) (*Webhook, error) {
	id := "wh_" + genID(8)
	if eventTypes == "" {
		eventTypes = "*"
	}
	_, err := db.conn.Exec(
		"INSERT INTO webhooks (id,url,secret,event_types) VALUES (?,?,?,?)",
		id, url, secret, eventTypes)
	if err != nil {
		return nil, err
	}
	return &Webhook{ID: id, URL: url, EventTypes: eventTypes, Enabled: true,
		CreatedAt: time.Now().UTC().Format(time.RFC3339)}, nil
}

func (db *DB) ListWebhooks() ([]Webhook, error) {
	rows, err := db.conn.Query("SELECT id,url,event_types,enabled,created_at FROM webhooks")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Webhook
	for rows.Next() {
		var w Webhook
		var en int
		rows.Scan(&w.ID, &w.URL, &w.EventTypes, &en, &w.CreatedAt)
		w.Enabled = en == 1
		out = append(out, w)
	}
	return out, rows.Err()
}

func (db *DB) DeleteWebhook(id string) error {
	_, err := db.conn.Exec("DELETE FROM webhooks WHERE id=?", id)
	return err
}

func (db *DB) ActiveWebhooks() ([]Webhook, error) {
	rows, err := db.conn.Query("SELECT id,url,secret,event_types FROM webhooks WHERE enabled=1")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Webhook
	for rows.Next() {
		var w Webhook
		rows.Scan(&w.ID, &w.URL, &w.Secret, &w.EventTypes)
		w.Enabled = true
		out = append(out, w)
	}
	return out, rows.Err()
}

// ── Stats ─────────────────────────────────────────────────────────────

func (db *DB) Stats() map[string]any {
	var total, today int
	var headHash string
	db.conn.QueryRow("SELECT COUNT(*) FROM ledger").Scan(&total)
	db.conn.QueryRow("SELECT COUNT(*) FROM ledger WHERE created_at >= date('now')").Scan(&today)
	db.conn.QueryRow("SELECT COALESCE(entry_hash,'genesis') FROM ledger ORDER BY seq DESC LIMIT 1").Scan(&headHash)
	return map[string]any{
		"total_events": total,
		"events_today": today,
		"head_hash":    headHash,
	}
}

// ── Helpers ───────────────────────────────────────────────────────────

func genID(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// MonthlyEventCount returns the number of events appended in the current calendar month.
func (db *DB) MonthlyEventCount() int {
	var n int
	db.conn.QueryRow("SELECT COUNT(*) FROM ledger WHERE created_at >= date('now','start of month')").Scan(&n)
	return n
}
