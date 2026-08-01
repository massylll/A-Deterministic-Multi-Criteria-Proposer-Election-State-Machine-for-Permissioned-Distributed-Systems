// Massyl Benarab (ORCID: 0009-0006-9405-3374)
// Younes Aoures (ORCID: 0009-0009-6551-1294)

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	shared "dmpe/shared"
)

type historyResp struct {
	ChainID   string `json:"chainID"`
	UpToRound int    `json:"upToRound"`
	Outcomes  []struct {
		Round    int    `json:"round"`
		Proposer string `json:"proposer"`
		Rep      int    `json:"rep"`
		Missed   int    `json:"missed"`
		Lat      int    `json:"lat"`
		Load     int    `json:"load"`
		Aff      int    `json:"aff"`
	} `json:"outcomes"`
}

type committeeResp struct {
	ChainID    string   `json:"chainID"`
	Validators []string `json:"validators"`
}

type roundResult struct {
	Node     string  `json:"node"`
	Round    int     `json:"round"`
	Proposer string  `json:"proposer"`
	Score    int     `json:"score"`
	ElectUs  float64 `json:"elect_us"`
	PullUs   float64 `json:"pull_us"`
	TotalUs  float64 `json:"total_us"`
}

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

func pullJSON(url string, dest interface{}) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GET %s → %d: %s", url, resp.StatusCode, string(body))
	}
	return json.NewDecoder(resp.Body).Decode(dest)
}

func reconstructObs(validators []string, hist historyResp) []shared.Obs {
	last := make(map[string]int, len(validators))

	latest := make(map[string]shared.Obs, len(validators))
	for _, id := range validators {
		last[id] = -1
		latest[id] = shared.Obs{ID: id, Rep: 500, Missed: 5, Lat: 20, Load: 100, Aff: 0, LastProp: -1}
	}
	for _, o := range hist.Outcomes {
		last[o.Proposer] = o.Round
		latest[o.Proposer] = shared.Obs{
			ID:       o.Proposer,
			Rep:      o.Rep,
			Missed:   o.Missed,
			Lat:      o.Lat,
			Load:     o.Load,
			Aff:      o.Aff,
			LastProp: o.Round,
		}
	}
	obs := make([]shared.Obs, 0, len(validators))
	for _, id := range validators {
		o := latest[id]
		o.LastProp = last[id]
		obs = append(obs, o)
	}
	return obs
}

func main() {
	nodeID := envOr("NODE_ID", "node-0")
	histURL := envOr("HISTORY_URL", "http://history:8080")
	chainID := envOr("CHAIN_ID", "dmpe-demo")
	n := envInt("N", 16)
	rounds := envInt("ROUNDS", 50)
	startDelay := envInt("START_DELAY_MS", 0)

	log.Printf("[%s] starting: history=%s chain=%s n=%d rounds=%d delay=%dms",
		nodeID, histURL, chainID, n, rounds, startDelay)

	if startDelay > 0 {
		time.Sleep(time.Duration(startDelay) * time.Millisecond)
	}


	for i := 0; i < 60; i++ {
		resp, err := http.Get(histURL + "/health")
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(500 * time.Millisecond)
		if i == 59 {
			log.Fatalf("[%s] history-server not ready after 30s", nodeID)
		}
	}

	var comm committeeResp
	if err := pullJSON(histURL+"/committee", &comm); err != nil {
		log.Fatalf("[%s] committee pull: %v", nodeID, err)
	}
	if len(comm.Validators) != n {
		log.Fatalf("[%s] committee size mismatch: got %d want %d", nodeID, len(comm.Validators), n)
	}
	log.Printf("[%s] committee loaded (%d validators)", nodeID, len(comm.Validators))

	enc := json.NewEncoder(os.Stdout)

	for r := 1; r <= rounds; r++ {
		t0 := time.Now()

		var hist historyResp
		if err := pullJSON(fmt.Sprintf("%s/history?round=%d", histURL, r), &hist); err != nil {
			log.Printf("[%s] round %d pull error: %v", nodeID, r, err)
			continue
		}
		pullUs := float64(time.Since(t0).Microseconds())

		obs := reconstructObs(comm.Validators, hist)

		t1 := time.Now()
		proposer, score := shared.ElectBorda(obs, r, chainID)
		electUs := float64(time.Since(t1).Microseconds())
		totalUs := float64(time.Since(t0).Microseconds())

		_ = enc.Encode(roundResult{
			Node:     nodeID,
			Round:    r,
			Proposer: proposer,
			Score:    score,
			ElectUs:  electUs,
			PullUs:   pullUs,
			TotalUs:  totalUs,
		})
	}

	log.Printf("[%s] finished %d rounds", nodeID, rounds)
}
