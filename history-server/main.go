// Massyl Benarab (ORCID: 0009-0006-9405-3374)
// Younes Aoures (ORCID: 0009-0009-6551-1294)

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
)

type BlockOutcome struct {
	Round    int    `json:"round"`
	Proposer string `json:"proposer"`


	Rep    int `json:"rep"`
	Missed int `json:"missed"`
	Lat    int `json:"lat"`
	Load   int `json:"load"`
	Aff    int `json:"aff"`
}

type HistoryResponse struct {
	ChainID   string         `json:"chainID"`
	UpToRound int            `json:"upToRound"`
	Outcomes  []BlockOutcome `json:"outcomes"`
}

type Committee struct {
	ChainID    string   `json:"chainID"`
	Validators []string `json:"validators"`
}

var (
	chainID   = envOr("CHAIN_ID", "dmpe-demo")
	n         = envInt("N", 16)
	maxRounds = envInt("MAX_ROUNDS", 200)
	port      = envOr("HISTORY_PORT", "8080")

	mu       sync.RWMutex
	history  []BlockOutcome
	commIDs  []string
)

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func buildDeterministicHistory(n, maxR int) ([]string, []BlockOutcome) {
	ids := make([]string, n)
	for i := 0; i < n; i++ {
		ids[i] = fmt.Sprintf("V%04d", i)
	}
	out := make([]BlockOutcome, 0, maxR)
	lastProp := make(map[string]int, n)
	for i := range ids {
		lastProp[ids[i]] = -1
	}



	for r := 1; r <= maxR; r++ {
		prop := ids[(r-1)%n]
		idx := (r - 1) % n
		bo := BlockOutcome{
			Round:    r,
			Proposer: prop,
			Rep:      500 + (idx % 50),
			Missed:   5 + (idx % 5),
			Lat:      20 + (idx % 30),
			Load:     100 + (idx % 40),
			Aff:      0,
		}
		out = append(out, bo)
		lastProp[prop] = r
	}
	return ids, out
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

func handleCommittee(w http.ResponseWriter, _ *http.Request) {
	mu.RLock()
	defer mu.RUnlock()
	resp := Committee{ChainID: chainID, Validators: append([]string(nil), commIDs...)}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func handleHistory(w http.ResponseWriter, r *http.Request) {
	roundStr := r.URL.Query().Get("round")
	round, err := strconv.Atoi(roundStr)
	if err != nil || round < 0 {
		http.Error(w, "round query param required (non-negative int)", http.StatusBadRequest)
		return
	}
	mu.RLock()
	defer mu.RUnlock()
	if round > len(history) {
		round = len(history)
	}
	resp := HistoryResponse{
		ChainID:   chainID,
		UpToRound: round,
		Outcomes:  history[:round],
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func main() {
	commIDs, history = buildDeterministicHistory(n, maxRounds)
	log.Printf("history-server: chainID=%s n=%d maxRounds=%d port=%s", chainID, n, maxRounds, port)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/committee", handleCommittee)
	mux.HandleFunc("/history", handleHistory)

	addr := ":" + port
	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
