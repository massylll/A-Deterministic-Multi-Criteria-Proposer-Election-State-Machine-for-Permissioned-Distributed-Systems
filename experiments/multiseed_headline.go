// Massyl Benarab (ORCID: 0009-0006-9405-3374)
// Younes Aoures (ORCID: 0009-0009-6551-1294)

package main

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"time"

	shared "dmpe/shared"
)

const chainID = "multiseed-headline"

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
	type kv struct {
		s float64
	}
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

func pct(xs []float64, p float64) float64 {
	cp := append([]float64(nil), xs...)
	sort.Float64s(cp)
	if len(cp) == 0 {
		return 0
	}
	idx := int(p * float64(len(cp)-1))
	return cp[idx]
}

type seedResult struct {
	pq, badShare, tieFrac, latUsMean float64
	nak                              int
}

func runSeed(n, R int, clique bool, seed int64) seedResult {
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
	latN := 0
	for r := 1; r <= R; r++ {
		applyMildNoise(obs, rng)
		var id string
		var tieSet int
		t0 := time.Now()
		if clique {
			id = shared.ElectClique(obs, r)
			tieSet = 1
		} else {
			id, _, tieSet = shared.ElectUniformBordaDetail(obs, r, chainID)
		}
		latSum += time.Since(t0)
		latN++
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
		pq:        poolQuality(counts, merits, R),
		badShare:  belowMedianShare(counts, merits, R),
		nak:       nakamoto(counts, R),
		tieFrac:   float64(ties) / float64(R),
		latUsMean: float64(latSum.Nanoseconds()) / float64(latN) / 1e3,
	}
}

func main() {
	fmt.Println("=== DMPE multi-seed headline metrics (A6) ===")
	fmt.Println("R=2000, seeds=9 (42..50), uniform Borda vs Clique")
	fmt.Println()

	seeds := make([]int64, 9)
	for i := range seeds {
		seeds[i] = 42 + int64(i)
	}
	ns := []int{16, 40, 100}
	R := 2000

	fmt.Printf("%-6s %-8s %10s %10s %10s %10s %10s %10s %10s\n",
		"n", "policy", "PQ_iqm", "PQ_std", "Nak_mean", "Nak_std", "BadShare", "tie_frac", "lat_us")
	for _, n := range ns {
		for _, clique := range []bool{false, true} {
			label := "DMPE"
			if clique {
				label = "Clique"
			}
			var pqs, naks, bads, ties, lats []float64
			for _, s := range seeds {
				res := runSeed(n, R, clique, s+int64(n)*1000)
				pqs = append(pqs, res.pq)
				naks = append(naks, float64(res.nak))
				bads = append(bads, res.badShare)
				ties = append(ties, res.tieFrac)
				lats = append(lats, res.latUsMean)
			}
			pqM, pqS := meanStd(pqs)
			nakM, nakS := meanStd(naks)
			badM, _ := meanStd(bads)
			tieM, _ := meanStd(ties)
			latM, _ := meanStd(lats)
			_ = pqM
			fmt.Printf("%-6d %-8s %10.4f %10.4f %10.2f %10.2f %10.4f %10.4f %10.2f\n",
				n, label, iqm(pqs), pqS, nakM, nakS, badM, tieM, latM)
		}
	}


	fmt.Println()
	fmt.Println("--- Cluster lift multi-seed (R=1500, 5 seeds) ---")
	fmt.Printf("%-6s %-4s %-8s %10s %10s\n", "N", "K", "policy", "lift_mean", "lift_std")
	for _, cfg := range []struct{ N, K int }{{16, 4}, {32, 8}} {
		for _, dmpe := range []bool{true, false} {
			label := "DMPE"
			if !dmpe {
				label = "Clique"
			}
			var lifts []float64
			for si, s := range []int64{42, 43, 44, 45, 46} {
				lifts = append(lifts, clusterLift(cfg.N, cfg.K, 1500, s+int64(si*17+cfg.N), dmpe))
			}
			m, sd := meanStd(lifts)
			fmt.Printf("%-6d %-4d %-8s %10.3f %10.3f\n", cfg.N, cfg.K, label, m, sd)
		}
	}
	fmt.Println("DONE")
}

func clusterLift(n, K, R int, seed int64, dmpe bool) float64 {
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
		var id string
		if dmpe {
			id, _ = shared.ElectUniformBorda(obs, r, chainID)
		} else {
			id = shared.ElectClique(obs, r)
		}
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
