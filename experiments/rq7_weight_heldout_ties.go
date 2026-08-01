// Massyl Benarab (ORCID: 0009-0006-9405-3374)
// Younes Aoures (ORCID: 0009-0009-6551-1294)

package main

import (
	"fmt"
	"math"
	"math/rand"
	"sort"

	shared "dmpe/shared"
)

const chainID = "rq7-experiment"

func merit(o shared.Obs) float64 {
	rep := float64(o.Rep) / 1000.0
	miss := 1.0 - float64(o.Missed)/1000.0
	lat := math.Max(0, 1.0-float64(o.Lat)/200.0)
	load := 1.0 - float64(o.Load)/1000.0
	return rep * miss * lat * load
}

func clamp(x, lo, hi int) int {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}

func makeHeterogeneous(n int, rng *rand.Rand) []shared.Obs {
	out := make([]shared.Obs, n)
	for i := 0; i < n; i++ {

		band := i % 4
		baseRep := 550 + band*100 + rng.Intn(40)
		baseMiss := 10 + (3-band)*15 + rng.Intn(10)
		baseLat := 15 + (3-band)*25 + rng.Intn(15)
		baseLoad := 150 + rng.Intn(200)
		out[i] = shared.Obs{
			ID:       fmt.Sprintf("V%04d", i),
			Rep:      clamp(baseRep, 0, 1000),
			Missed:   clamp(baseMiss, 0, 1000),
			Lat:      clamp(baseLat, 0, 1000),
			Load:     clamp(baseLoad, 0, 1000),
			Aff:      0,
			LastProp: -1,
		}
	}
	return out
}

func makeLatencySkew(n, nSlow int, rng *rand.Rand) []shared.Obs {
	out := makeHeterogeneous(n, rng)

	idx := rng.Perm(n)[:nSlow]
	for _, i := range idx {
		out[i].Lat = clamp(180+rng.Intn(40), 0, 1000)
	}
	return out
}

func applyNoise(base []shared.Obs, rng *rand.Rand) []shared.Obs {
	cur := make([]shared.Obs, len(base))
	copy(cur, base)
	for i := range cur {
		cur[i].Rep = clamp(cur[i].Rep+rng.Intn(21)-10, 0, 1000)
		cur[i].Missed = clamp(cur[i].Missed+rng.Intn(7)-3, 0, 1000)
		cur[i].Lat = clamp(cur[i].Lat+rng.Intn(11)-5, 0, 1000)
		cur[i].Load = clamp(cur[i].Load+rng.Intn(21)-10, 0, 1000)
	}
	return cur
}

func recordOutcome(base []shared.Obs, winner string, round int) {
	for i := range base {
		if base[i].ID == winner {
			base[i].LastProp = round
			base[i].Rep = clamp(base[i].Rep+2, 0, 1000)
			return
		}
	}
}

type runStats struct {
	wins       map[string]int
	tieRounds  int
	totalRounds int

	winsFirst map[string]int
	winsSecond map[string]int
}

func newStats() *runStats {
	return &runStats{
		wins:       make(map[string]int),
		winsFirst:  make(map[string]int),
		winsSecond: make(map[string]int),
	}
}

func (s *runStats) add(id string, round, half int, tieSet int) {
	s.wins[id]++
	s.totalRounds++
	if tieSet > 1 {
		s.tieRounds++
	}
	if round <= half {
		s.winsFirst[id]++
	} else {
		s.winsSecond[id]++
	}
}

func poolQuality(base []shared.Obs, wins map[string]int) float64 {
	total := 0
	for _, w := range wins {
		total += w
	}
	if total == 0 {
		return 0
	}
	meritByID := make(map[string]float64, len(base))
	for _, o := range base {
		meritByID[o.ID] = merit(o)
	}
	var pq float64
	for id, w := range wins {
		pq += (float64(w) / float64(total)) * meritByID[id]
	}
	return pq
}

func kendallTauWins(ref, other map[string]int, ids []string) float64 {
	n := len(ids)
	if n < 2 {
		return 1
	}
	var conc, disc float64
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			a, b := ids[i], ids[j]
			rSign := sgnI(ref[a] - ref[b])
			oSign := sgnI(other[a] - other[b])
			if rSign == 0 || oSign == 0 {
				continue
			}
			if rSign == oSign {
				conc++
			} else {
				disc++
			}
		}
	}
	den := conc + disc
	if den == 0 {
		return 1
	}
	return (conc - disc) / den
}

func sgnI(x int) int {
	if x > 0 {
		return 1
	}
	if x < 0 {
		return -1
	}
	return 0
}

func slowShare(base []shared.Obs, wins map[string]int, slowIDs map[string]bool, rounds int) float64 {
	s := 0
	for id, w := range wins {
		if slowIDs[id] {
			s += w
		}
	}
	return float64(s) / float64(rounds)
}

type electFn func(obs []shared.Obs, round int) (string, int, int)

func runPolicy(base []shared.Obs, rounds int, seed int64, elect electFn) *runStats {
	rng := rand.New(rand.NewSource(seed))

	state := make([]shared.Obs, len(base))
	copy(state, base)
	for i := range state {
		state[i].LastProp = -1
	}
	st := newStats()
	half := rounds / 2
	for r := 1; r <= rounds; r++ {
		cur := applyNoise(state, rng)

		for i := range cur {
			cur[i].LastProp = state[i].LastProp
		}
		id, _, tieSet := elect(cur, r)
		st.add(id, r, half, tieSet)
		recordOutcome(state, id, r)
	}
	return st
}

func idsOf(obs []shared.Obs) []string {
	out := make([]string, len(obs))
	for i, o := range obs {
		out[i] = o.ID
	}
	sort.Strings(out)
	return out
}

func main() {
	const (
		n      = 16
		nSlow  = 4
		rounds = 4000
		seed   = 42
	)

	fmt.Println("=== DMPE RQ7 / construct-validity suite ===")
	fmt.Printf("n=%d  nSlow=%d  rounds=%d  seed=%d  chainID=%s\n\n", n, nSlow, rounds, seed, chainID)

	rng := rand.New(rand.NewSource(seed))
	het := makeHeterogeneous(n, rng)
	skew := makeLatencySkew(n, nSlow, rand.New(rand.NewSource(seed+1)))


	slowIDs := make(map[string]bool)
	for _, o := range skew {
		if o.Lat >= 180 {
			slowIDs[o.ID] = true
		}
	}
	fmt.Printf("skew slow-class size: %d\n", len(slowIDs))


	weighted := func(obs []shared.Obs, round int) (string, int, int) {
		return shared.ElectBordaDetail(obs, round, chainID)
	}
	uniform := func(obs []shared.Obs, round int) (string, int, int) {
		return shared.ElectUniformBordaDetail(obs, round, chainID)
	}

	wRHalf := func(obs []shared.Obs, round int) (string, int, int) {
		return shared.ElectWeightedBorda(obs, round, chainID, 50, 120, 60, 40, 20)
	}
	wR15 := func(obs []shared.Obs, round int) (string, int, int) {
		return shared.ElectWeightedBorda(obs, round, chainID, 150, 120, 60, 40, 20)
	}

	latHeavy := func(obs []shared.Obs, round int) (string, int, int) {
		return shared.ElectWeightedBorda(obs, round, chainID, 40, 40, 200, 20, 20)
	}

	type named struct {
		name string
		fn   electFn
	}
	policies := []named{
		{"weighted (prod)", weighted},
		{"uniform (default)", uniform},
		{"wR×0.5", wRHalf},
		{"wR×1.5", wR15},
		{"lat-heavy", latHeavy},
	}


	fmt.Println("--- Heterogeneous population ---")
	var refWins map[string]int
	idList := idsOf(het)
	for _, p := range policies {
		st := runPolicy(het, rounds, seed+10, p.fn)
		pq := poolQuality(het, st.wins)
		pq1 := poolQuality(het, st.winsFirst)
		pq2 := poolQuality(het, st.winsSecond)
		tieFrac := float64(st.tieRounds) / float64(st.totalRounds)
		tau := 1.0
		if refWins != nil {
			tau = kendallTauWins(refWins, st.wins, idList)
		} else {
			refWins = st.wins
		}
		fmt.Printf("%-18s  PQ=%.4f  PQ_first=%.4f  PQ_second=%.4f  |ΔPQ|=%.4f  tieFrac=%.3f  τ(vs prod)=%.3f\n",
			p.name, pq, pq1, pq2, math.Abs(pq1-pq2), tieFrac, tau)
	}


	fmt.Println("\n--- Latency-skew population (nSlow minority) ---")
	refWins = nil
	idList = idsOf(skew)
	for _, p := range policies {
		st := runPolicy(skew, rounds, seed+20, p.fn)
		pq := poolQuality(skew, st.wins)
		pq1 := poolQuality(skew, st.winsFirst)
		pq2 := poolQuality(skew, st.winsSecond)
		tieFrac := float64(st.tieRounds) / float64(st.totalRounds)
		ss := slowShare(skew, st.wins, slowIDs, rounds)
		tau := 1.0
		if refWins != nil {
			tau = kendallTauWins(refWins, st.wins, idList)
		} else {
			refWins = st.wins
		}
		fmt.Printf("%-18s  PQ=%.4f  PQ_first=%.4f  PQ_second=%.4f  |ΔPQ|=%.4f  slowShare=%.3f  tieFrac=%.3f  τ=%.3f\n",
			p.name, pq, pq1, pq2, math.Abs(pq1-pq2), ss, tieFrac, tau)
	}


	fmt.Println("\n--- Clique baseline (heterogeneous) ---")
	cliqueFn := func(obs []shared.Obs, round int) (string, int, int) {
		return shared.ElectClique(obs, round), 0, 1
	}
	stC := runPolicy(het, rounds, seed+30, cliqueFn)
	fmt.Printf("%-18s  PQ=%.4f  PQ_first=%.4f  PQ_second=%.4f\n",
		"Clique", poolQuality(het, stC.wins), poolQuality(het, stC.winsFirst), poolQuality(het, stC.winsSecond))

	fmt.Println("\nInterpretation:")
	fmt.Println("  • Uniform vs weighted: if τ≈1 and |ΔPQ| small, weights are inert → default to uniform Borda.")
	fmt.Println("  • Held-out: |PQ_first − PQ_second| measures temporal stability of the quality signal.")
	fmt.Println("  • tieFrac: fraction of rounds where SHA-256 tie-break was decisive (tieSet>1).")
	fmt.Println("  • slowShare: under latency skew, lower is better (quality-aware rejection of slow class).")
}
