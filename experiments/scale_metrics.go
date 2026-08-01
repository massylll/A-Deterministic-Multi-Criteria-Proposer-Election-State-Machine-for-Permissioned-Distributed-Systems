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

const chainID = "scale-metrics"

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

func runPolicy(n, R int, clique bool, seed int64) (counts map[string]int, ties, rounds int, merits map[string]float64) {
	rng := rand.New(rand.NewSource(seed))
	obs := makeHeterogeneous(n, rng)
	counts = make(map[string]int, n)
	merits = make(map[string]float64, n)
	for i := range obs {
		merits[obs[i].ID] = merit(obs[i])
		counts[obs[i].ID] = 0
	}
	for r := 1; r <= R; r++ {
		applyMildNoise(obs, rng)
		var id string
		var tieSet int
		if clique {
			id = shared.ElectClique(obs, r)
			tieSet = 1
		} else {
			id, _, tieSet = shared.ElectUniformBordaDetail(obs, r, chainID)
		}
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
	return counts, ties, R, merits
}

func poolQuality(counts map[string]int, merits map[string]float64, R int) float64 {
	var pq float64
	for id, c := range counts {
		pq += (float64(c) / float64(R)) * merits[id]
	}
	return pq
}

func nakamoto(counts map[string]int, R int) int {
	type kv struct {
		id string
		s  float64
	}
	arr := make([]kv, 0, len(counts))
	for id, c := range counts {
		arr = append(arr, kv{id, float64(c) / float64(R)})
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

func badConcentrators(counts map[string]int, merits map[string]float64, R, n int) float64 {
	return belowMedianShare(counts, merits, R) * float64(n)
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
	if len(xs) < 4 {
		sort.Float64s(xs)
		var s float64
		for _, x := range xs {
			s += x
		}
		return s / float64(len(xs))
	}
	cp := append([]float64(nil), xs...)
	sort.Float64s(cp)
	lo := len(cp) / 4
	hi := len(cp) - lo
	var s float64
	for i := lo; i < hi; i++ {
		s += cp[i]
	}
	return s / float64(hi-lo)
}

func runClusterLift(n, K, R int, seed int64, dmpe bool) float64 {
	rng := rand.New(rand.NewSource(seed))
	obs := makeHeterogeneous(n, rng)

	sort.Slice(obs, func(i, j int) bool { return obs[i].ID < obs[j].ID })
	cluster := make(map[string]bool, K)
	for i := 0; i < K; i++ {
		cluster[obs[i].ID] = true
		obs[i].Rep = clamp(obs[i].Rep+120, 0, 1000)
		obs[i].Missed = clamp(obs[i].Missed-20, 0, 1000)
		obs[i].Lat = clamp(obs[i].Lat-20, 0, 1000)
	}
	counts := make(map[string]int, n)
	for i := range obs {
		counts[obs[i].ID] = 0
	}
	for r := 1; r <= R; r++ {
		applyMildNoise(obs, rng)

		for i := range obs {
			if cluster[obs[i].ID] {
				obs[i].Rep = clamp(obs[i].Rep+1, 0, 1000)
			}
		}
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
	fair := float64(K) / float64(n)
	return cShare / fair
}

func main() {
	fmt.Println("=== DMPE scale_metrics (uniform Borda default) ===")
	fmt.Println()


	ns := []int{16, 40, 100}
	R := 4000
	seeds := []int64{42, 43, 44, 45, 46}
	fmt.Println("--- Multi-seed policy metrics (R=4000, 5 seeds) ---")
	fmt.Printf("%-6s %-8s %10s %10s %10s %10s %10s\n", "n", "policy", "PQ_iqm", "PQ_std", "Nak_mean", "BadC_iqm", "tie_frac")
	for _, n := range ns {
		for _, clique := range []bool{false, true} {
			label := "DMPE"
			if clique {
				label = "Clique"
			}
			var pqs, bads, naks, tieFracs []float64
			for _, s := range seeds {
				counts, ties, rounds, merits := runPolicy(n, R, clique, s+int64(n)*100)
				pqs = append(pqs, poolQuality(counts, merits, rounds))
				bads = append(bads, badConcentrators(counts, merits, rounds, n))
				naks = append(naks, float64(nakamoto(counts, rounds)))
				tieFracs = append(tieFracs, float64(ties)/float64(rounds))
			}
			pqM, pqS := meanStd(pqs)
			badM, _ := meanStd(bads)
			nakM, _ := meanStd(naks)
			tieM, _ := meanStd(tieFracs)
			_ = pqM
			fmt.Printf("%-6d %-8s %10.4f %10.4f %10.2f %10.3f %10.4f\n",
				n, label, iqm(pqs), pqS, nakM, iqm(bads), tieM)
			_ = badM
		}
	}


	fmt.Println()
	fmt.Println("--- Cluster lift (R=2000, seed=42; lift = cluster_share / fair_share) ---")
	fmt.Printf("%-6s %-4s %-8s %10s\n", "N", "K", "policy", "lift")
	for _, N := range []int{16, 32} {
		for _, frac := range []float64{0.125, 0.25, 0.375, 0.5} {
			K := int(math.Round(frac * float64(N)))
			if K < 1 {
				K = 1
			}
			for _, dmpe := range []bool{true, false} {
				label := "DMPE"
				if !dmpe {
					label = "Clique"
				}
				lift := runClusterLift(N, K, 2000, 42+int64(N*10+K), dmpe)
				fmt.Printf("%-6d %-4d %-8s %10.3f\n", N, K, label, lift)
			}
		}
	}


	fmt.Println()
	fmt.Println("--- Tie-set size histogram (n=16, R=4000, seed=42, uniform Borda) ---")
	rng := rand.New(rand.NewSource(42))
	obs := makeHeterogeneous(16, rng)
	hist := map[int]int{}
	for r := 1; r <= 4000; r++ {
		applyMildNoise(obs, rng)
		id, _, ts := shared.ElectUniformBordaDetail(obs, r, chainID)
		hist[ts]++
		for i := range obs {
			if obs[i].ID == id {
				obs[i].LastProp = r
				obs[i].Rep = clamp(obs[i].Rep+2, 0, 1000)
			}
		}
	}
	keys := make([]int, 0, len(hist))
	for k := range hist {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	for _, k := range keys {
		fmt.Printf("  tieSet=%d  rounds=%d  frac=%.4f\n", k, hist[k], float64(hist[k])/4000.0)
	}
	fmt.Println("DONE")
}
