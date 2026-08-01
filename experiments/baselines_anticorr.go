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

const chainID = "baselines-anticorr"

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

func makeAntiCorr(n int, rng *rand.Rand) []shared.Obs {
	out := make([]shared.Obs, n)
	for i := 0; i < n; i++ {
		if i%2 == 0 {

			out[i] = shared.Obs{
				ID:       fmt.Sprintf("V%04d", i),
				Rep:      clamp(780+rng.Intn(80), 0, 1000),
				Missed:   clamp(40+rng.Intn(40), 0, 1000),
				Lat:      clamp(160+rng.Intn(50), 0, 1000),
				Load:     clamp(250+rng.Intn(150), 0, 1000),
				Aff:      0,
				LastProp: -1,
			}
		} else {

			out[i] = shared.Obs{
				ID:       fmt.Sprintf("V%04d", i),
				Rep:      clamp(480+rng.Intn(80), 0, 1000),
				Missed:   clamp(5+rng.Intn(20), 0, 1000),
				Lat:      clamp(10+rng.Intn(25), 0, 1000),
				Load:     clamp(100+rng.Intn(80), 0, 1000),
				Aff:      0,
				LastProp: -1,
			}
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

func runSeed(n, R int, pol policy, seed int64) (pq, bad float64, nak int, latUs float64) {
	rng := rand.New(rand.NewSource(seed))
	obs := makeAntiCorr(n, rng)
	counts := make(map[string]int, n)
	merits := make(map[string]float64, n)
	for i := range obs {
		merits[obs[i].ID] = merit(obs[i])
		counts[obs[i].ID] = 0
	}
	var latSum time.Duration
	for r := 1; r <= R; r++ {
		applyMildNoise(obs, rng)
		t0 := time.Now()
		id, _ := pol.elect(obs, r)
		latSum += time.Since(t0)
		counts[id]++
		for i := range obs {
			if obs[i].ID == id {
				obs[i].LastProp = r
				obs[i].Rep = clamp(obs[i].Rep+2, 0, 1000)
			}
		}
	}
	return poolQuality(counts, merits, R),
		belowMedianShare(counts, merits, R),
		nakamoto(counts, R),
		float64(latSum.Nanoseconds()) / float64(R) / 1e3
}

func main() {
	fmt.Println("=== Anti-correlated population baseline campaign ===")
	fmt.Println("High-Rep co-occurs with high-Lat; mid-Rep with low-Lat.")
	fmt.Println("R=2000, seeds=9 (42..50)")
	fmt.Println()
	fmt.Printf("%-6s %-10s %10s %10s %10s %10s %10s\n",
		"n", "policy", "PQ_iqm", "PQ_std", "Nak_mean", "BadShare", "lat_us")

	seeds := make([]int64, 9)
	for i := range seeds {
		seeds[i] = 42 + int64(i)
	}
	for _, n := range []int{16, 40} {
		for _, pol := range allPolicies() {
			var pqs, naks, bads, lats []float64
			for _, s := range seeds {
				pq, bad, nak, lat := runSeed(n, 2000, pol, s+int64(n)*1000)
				pqs = append(pqs, pq)
				naks = append(naks, float64(nak))
				bads = append(bads, bad)
				lats = append(lats, lat)
			}
			_, pqS := meanStd(pqs)
			nakM, _ := meanStd(naks)
			badM, _ := meanStd(bads)
			latM, _ := meanStd(lats)
			fmt.Printf("%-6d %-10s %10.4f %10.4f %10.2f %10.4f %10.2f\n",
				n, pol.name, iqm(pqs), pqS, nakM, badM, latM)
		}
	}
	fmt.Println("DONE")
}
