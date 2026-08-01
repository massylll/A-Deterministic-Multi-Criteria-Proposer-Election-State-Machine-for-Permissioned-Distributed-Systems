// Massyl Benarab (ORCID: 0009-0006-9405-3374)
// Younes Aoures (ORCID: 0009-0009-6551-1294)

package main

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"time"

	"dmpe/consensus"
	shared "dmpe/shared"
)

const chainID = "baselines-campaign"

func clamp(x, lo, hi int) int {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}

func merit(o shared.Obs) float64 {
	rep := float64(o.Rep) / 1000.0
	miss := 1.0 - float64(o.Missed)/1000.0
	lat := math.Max(0, 1.0-float64(o.Lat)/200.0)
	load := 1.0 - float64(o.Load)/1000.0
	return rep * miss * lat * load
}

func makeHeterogeneous(n int, rng *rand.Rand) []shared.Obs {
	out := make([]shared.Obs, n)
	for i := 0; i < n; i++ {
		band := i % 4
		out[i] = shared.Obs{
			ID:       fmt.Sprintf("V%04d", i),
			Rep:      clamp(550+band*100+rng.Intn(40), 0, 1000),
			Missed:   clamp(10+(3-band)*15+rng.Intn(10), 0, 1000),
			Lat:      clamp(15+(3-band)*25+rng.Intn(15), 0, 1000),
			Load:     clamp(150+rng.Intn(200), 0, 1000),
			Aff:      0,
			LastProp: -1,
		}
	}
	return out
}

func applyMildNoise(obs []shared.Obs, rng *rand.Rand) {
	for i := range obs {
		obs[i].Rep = clamp(obs[i].Rep+rng.Intn(11)-5, 0, 1000)
		obs[i].Missed = clamp(obs[i].Missed+rng.Intn(7)-3, 0, 1000)
		obs[i].Lat = clamp(obs[i].Lat+rng.Intn(7)-3, 0, 1000)
	}
}

func nakamoto(counts map[string]int, R int) int {
	type kv struct{ s float64 }
	arr := make([]kv, 0, len(counts))
	for _, c := range counts {
		arr = append(arr, kv{float64(c) / float64(R)})
	}
	sort.Slice(arr, func(i, j int) bool { return arr[i].s > arr[j].s })
	sum := 0.0
	for i, a := range arr {
		sum += a.s
		if sum >= 0.51 {
			return i + 1
		}
	}
	return len(arr)
}

func belowMedianShare(counts map[string]int, merits map[string]float64, R int) float64 {
	ms := make([]float64, 0, len(merits))
	for _, m := range merits {
		ms = append(ms, m)
	}
	sort.Float64s(ms)
	med := ms[len(ms)/2]
	var share float64
	for id, c := range counts {
		if merits[id] < med {
			share += float64(c) / float64(R)
		}
	}
	return share
}

func poolQuality(counts map[string]int, merits map[string]float64, R int) float64 {
	var pq float64
	for id, c := range counts {
		pq += (float64(c) / float64(R)) * merits[id]
	}
	return pq
}

func meanStd(xs []float64) (float64, float64) {
	if len(xs) == 0 {
		return 0, 0
	}
	var s float64
	for _, x := range xs {
		s += x
	}
	m := s / float64(len(xs))
	var v float64
	for _, x := range xs {
		d := x - m
		v += d * d
	}
	return m, math.Sqrt(v / float64(len(xs)))
}

func iqm(xs []float64) float64 {
	cp := append([]float64(nil), xs...)
	sort.Float64s(cp)
	if len(cp) < 4 {
		var s float64
		for _, x := range cp {
			s += x
		}
		return s / float64(len(cp))
	}
	lo := len(cp) / 4
	hi := len(cp) - lo
	var s float64
	for i := lo; i < hi; i++ {
		s += cp[i]
	}
	return s / float64(hi-lo)
}

type policy struct {
	name  string
	elect func(obs []shared.Obs, round int) (id string, tieSet int)
}

func allPolicies() []policy {
	return []policy{
		{"DMPE", func(obs []shared.Obs, round int) (string, int) {
			id, _, t := shared.ElectUniformBordaDetail(obs, round, chainID)
			return id, t
		}},
		{"Scalar", func(obs []shared.Obs, round int) (string, int) {
			id, _, t := shared.ElectScalarDetail(obs, round, chainID)
			return id, t
		}},
		{"RepOnly", func(obs []shared.Obs, round int) (string, int) {
			id, _, t := shared.ElectRepOnlyDetail(obs, round, chainID)
			return id, t
		}},
		{"EligibleRR", func(obs []shared.Obs, round int) (string, int) {
			return shared.ElectEligibleRR(obs, round, chainID), 1
		}},
		{"Clique", func(obs []shared.Obs, round int) (string, int) {
			return shared.ElectClique(obs, round), 1
		}},
	}
}

type seedResult struct {
	pq, badShare, tieFrac, latUs float64
	nak                          int
}

func runSeed(n, R int, pol policy, seed int64) seedResult {
	rng := rand.New(rand.NewSource(seed))
	obs := makeHeterogeneous(n, rng)
	counts := make(map[string]int, n)
	merits := make(map[string]float64, n)
	for i := range obs {
		merits[obs[i].ID] = merit(obs[i])
		counts[obs[i].ID] = 0
	}
	ties := 0
	var latSum time.Duration
	for r := 1; r <= R; r++ {
		applyMildNoise(obs, rng)
		t0 := time.Now()
		id, tieSet := pol.elect(obs, r)
		latSum += time.Since(t0)
		counts[id]++
		if tieSet > 1 {
			ties++
		}
		for i := range obs {
			if obs[i].ID == id {
				obs[i].LastProp = r
				obs[i].Rep = clamp(obs[i].Rep+2, 0, 1000)
			}
		}
	}
	return seedResult{
		pq:       poolQuality(counts, merits, R),
		badShare: belowMedianShare(counts, merits, R),
		nak:      nakamoto(counts, R),
		tieFrac:  float64(ties) / float64(R),
		latUs:    float64(latSum.Nanoseconds()) / float64(R) / 1e3,
	}
}

func headlineCampaign() {
	fmt.Println("=== Baseline multi-seed headline metrics ===")
	fmt.Println("R=2000, seeds=9 (42..50)")
	fmt.Println()
	fmt.Printf("%-6s %-10s %10s %10s %10s %10s %10s %10s %10s\n",
		"n", "policy", "PQ_iqm", "PQ_std", "Nak_mean", "Nak_std", "BadShare", "tie_frac", "lat_us")

	seeds := make([]int64, 9)
	for i := range seeds {
		seeds[i] = 42 + int64(i)
	}
	ns := []int{16, 40, 100}
	R := 2000
	pols := allPolicies()

	for _, n := range ns {
		for _, pol := range pols {
			var pqs, naks, bads, ties, lats []float64
			for _, s := range seeds {
				res := runSeed(n, R, pol, s+int64(n)*1000)
				pqs = append(pqs, res.pq)
				naks = append(naks, float64(res.nak))
				bads = append(bads, res.badShare)
				ties = append(ties, res.tieFrac)
				lats = append(lats, res.latUs)
			}
			_, pqS := meanStd(pqs)
			nakM, nakS := meanStd(naks)
			badM, _ := meanStd(bads)
			tieM, _ := meanStd(ties)
			latM, _ := meanStd(lats)
			fmt.Printf("%-6d %-10s %10.4f %10.4f %10.2f %10.2f %10.4f %10.4f %10.2f\n",
				n, pol.name, iqm(pqs), pqS, nakM, nakS, badM, tieM, latM)
		}
	}
}

func heldoutFSR() {
	fmt.Println()
	fmt.Println("=== Held-out independent FSR (n=16, R=4000, seed=1001) ===")
	fmt.Println("Finalisation model: independentMerit (no Load/Aff; not ranking weights)")

	rng := rand.New(rand.NewSource(42))
	base := makeHeterogeneous(16, rng)
	engines := []consensus.Engine{
		consensus.CliqueEngine{},
		consensus.IBFTEngine{},
		consensus.HotStuffEngine{},
	}
	const rounds = 4000
	half := rounds / 2
	pols := allPolicies()

	fmt.Printf("%-10s %-10s %10s %10s %10s\n", "policy", "engine", "FSR_all", "FSR_held", "PQ_held")

	for _, pol := range pols {
		state := make([]shared.Obs, len(base))
		copy(state, base)
		for i := range state {
			state[i].LastProp = -1
		}
		rngP := rand.New(rand.NewSource(1001))
		winsHold := map[string]int{}
		finHold := map[string]int{}
		totHold := map[string]int{}
		finAll := map[string]int{}
		totAll := map[string]int{}

		for r := 1; r <= rounds; r++ {

			cur := make([]shared.Obs, len(state))
			copy(cur, state)
			for i := range cur {
				cur[i].Rep = clamp(cur[i].Rep+rngP.Intn(21)-10, 0, 1000)
				cur[i].Missed = clamp(cur[i].Missed+rngP.Intn(7)-3, 0, 1000)
				cur[i].Lat = clamp(cur[i].Lat+rngP.Intn(11)-5, 0, 1000)
				cur[i].Load = clamp(cur[i].Load+rngP.Intn(21)-10, 0, 1000)
				cur[i].LastProp = state[i].LastProp
			}
			prop, _ := pol.elect(cur, r)
			if r > half {
				winsHold[prop]++
			}
			var refFinal bool
			for i, eng := range engines {
				erng := rand.New(rand.NewSource(1001 + int64(r)*1009 + int64(i)*17 + int64(len(eng.Name()))))
				res := eng.RunRound(r, prop, cur, erng)
				totAll[eng.Name()]++
				if res.Finalized {
					finAll[eng.Name()]++
				}
				if r > half {
					totHold[eng.Name()]++
					if res.Finalized {
						finHold[eng.Name()]++
					}
				}
				if i == 0 {
					refFinal = res.Finalized
				}
			}

			for i := range state {
				if state[i].ID != prop {
					continue
				}
				state[i].LastProp = r
				if refFinal {
					state[i].Rep = clamp(state[i].Rep+3, 0, 1000)
				} else {
					state[i].Missed = clamp(state[i].Missed+8, 0, 1000)
					state[i].Rep = clamp(state[i].Rep-5, 0, 1000)
				}
				break
			}
		}

		merits := map[string]float64{}
		for _, o := range base {
			merits[o.ID] = merit(o)
		}
		pqH := poolQuality(winsHold, merits, half)
		for _, name := range []string{"Clique", "IBFT", "HotStuff"} {
			fsrA := float64(finAll[name]) / float64(totAll[name])
			fsrH := float64(finHold[name]) / float64(totHold[name])
			fmt.Printf("%-10s %-10s %10.4f %10.4f %10.4f\n", pol.name, name, fsrA, fsrH, pqH)
		}
	}
}

func clusterLift(n, K, R int, seed int64, pol policy) float64 {
	rng := rand.New(rand.NewSource(seed))
	obs := makeHeterogeneous(n, rng)
	sort.Slice(obs, func(i, j int) bool { return obs[i].ID < obs[j].ID })
	cluster := map[string]bool{}
	for i := 0; i < K; i++ {
		cluster[obs[i].ID] = true
		obs[i].Rep = clamp(obs[i].Rep+120, 0, 1000)
		obs[i].Missed = clamp(obs[i].Missed-20, 0, 1000)
		obs[i].Lat = clamp(obs[i].Lat-20, 0, 1000)
	}
	counts := map[string]int{}
	for i := range obs {
		counts[obs[i].ID] = 0
	}
	for r := 1; r <= R; r++ {
		applyMildNoise(obs, rng)
		id, _ := pol.elect(obs, r)
		counts[id]++
		for i := range obs {
			if obs[i].ID == id {
				obs[i].LastProp = r
			}
		}
	}
	var cShare float64
	for id, c := range counts {
		if cluster[id] {
			cShare += float64(c) / float64(R)
		}
	}
	return cShare / (float64(K) / float64(n))
}

func clusterCampaign() {
	fmt.Println()
	fmt.Println("=== Cluster lift multi-seed (R=1500, 5 seeds) ===")
	fmt.Printf("%-6s %-4s %-10s %10s %10s\n", "N", "K", "policy", "lift_mean", "lift_std")
	pols := allPolicies()
	for _, cfg := range []struct{ N, K int }{{16, 4}, {32, 8}} {
		for _, pol := range pols {
			var lifts []float64
			for si, s := range []int64{42, 43, 44, 45, 46} {
				lifts = append(lifts, clusterLift(cfg.N, cfg.K, 1500, s+int64(si*17+cfg.N), pol))
			}
			m, sd := meanStd(lifts)
			fmt.Printf("%-6d %-4d %-10s %10.3f %10.3f\n", cfg.N, cfg.K, pol.name, m, sd)
		}
	}
}

func main() {
	fmt.Println("DMPE baselines campaign — Scalar / RepOnly / EligibleRR vs DMPE / Clique")
	fmt.Println()
	headlineCampaign()
	heldoutFSR()
	clusterCampaign()
	fmt.Println("DONE")
}
