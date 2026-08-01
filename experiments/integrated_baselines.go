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

const chainID = "integrated-baselines"

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

func evolveFromOutcome(obs []shared.Obs, winner string, round int, rng *rand.Rand) {
	for i := range obs {
		if obs[i].ID == winner {
			obs[i].LastProp = round
			obs[i].Rep = clamp(obs[i].Rep+3+rng.Intn(3), 0, 1000)
			obs[i].Lat = clamp(obs[i].Lat+rng.Intn(9)-4, 0, 1000)
			obs[i].Missed = clamp(obs[i].Missed-rng.Intn(3), 0, 1000)
			obs[i].Load = clamp(obs[i].Load+rng.Intn(11)-5, 0, 1000)
		} else {

			if rng.Float64() < 0.15 {
				obs[i].Missed = clamp(obs[i].Missed+1+rng.Intn(2), 0, 1000)
			}
			obs[i].Load = clamp(obs[i].Load+rng.Intn(11)-5, 0, 1000)
			obs[i].Rep = clamp(obs[i].Rep+rng.Intn(3)-1, 0, 1000)
		}
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

func iqm(xs []float64) float64 {
	s := append([]float64{}, xs...)
	sort.Float64s(s)
	lo := len(s) / 4
	hi := len(s) - lo
	if hi <= lo {
		hi = lo + 1
	}
	var sum float64
	for i := lo; i < hi; i++ {
		sum += s[i]
	}
	return sum / float64(hi-lo)
}

func meanStd(xs []float64) (float64, float64) {
	var m float64
	for _, x := range xs {
		m += x
	}
	m /= float64(len(xs))
	var v float64
	for _, x := range xs {
		d := x - m
		v += d * d
	}
	return m, math.Sqrt(v / float64(len(xs)))
}

type pol struct {
	name  string
	elect func([]shared.Obs, int) string
}

func policies() []pol {
	return []pol{
		{"DMPE", func(obs []shared.Obs, r int) string {
			id, _ := shared.ElectUniformBorda(obs, r, chainID)
			return id
		}},
		{"ScalarU", func(obs []shared.Obs, r int) string {
			id, _ := shared.ElectScalarUniform(obs, r, chainID)
			return id
		}},
		{"Scalar", func(obs []shared.Obs, r int) string {
			id, _ := shared.ElectScalar(obs, r, chainID)
			return id
		}},
		{"RepOnly", func(obs []shared.Obs, r int) string {
			id, _ := shared.ElectRepOnly(obs, r, chainID)
			return id
		}},
		{"EligibleRR", func(obs []shared.Obs, r int) string {
			return shared.ElectEligibleRR(obs, r, chainID)
		}},
		{"Clique", func(obs []shared.Obs, r int) string {
			return shared.ElectClique(obs, r)
		}},
	}
}

func runSeed(n, R int, p pol, seed int64) (pq, bad float64, nak int) {
	rng := rand.New(rand.NewSource(seed))
	obs := makeHeterogeneous(n, rng)
	counts := make(map[string]int, n)
	for i := range obs {
		counts[obs[i].ID] = 0
	}


	merits := make(map[string]float64, n)
	for i := range obs {
		merits[obs[i].ID] = merit(obs[i])
	}
	for r := 1; r <= R; r++ {
		id := p.elect(obs, r)
		counts[id]++
		evolveFromOutcome(obs, id, r, rng)
	}
	return poolQuality(counts, merits, R), belowMedianShare(counts, merits, R), nakamoto(counts, R)
}

func main() {
	fmt.Println("=== Integrated outcome-coupled baselines (co-varying) ===")
	fmt.Println("Observation axes evolve from election outcomes each round.")
	fmt.Println("R=4000, seeds=9 (42..50); n=16,40")
	fmt.Println()
	fmt.Printf("%-6s %-10s %10s %10s %10s %10s\n",
		"n", "policy", "PQ_iqm", "PQ_std", "Nak_mean", "BadShare")

	seeds := make([]int64, 9)
	for i := range seeds {
		seeds[i] = 42 + int64(i)
	}
	t0 := time.Now()
	for _, n := range []int{16, 40} {
		for _, p := range policies() {
			var pqs, naks, bads []float64
			for _, s := range seeds {
				pq, bad, nak := runSeed(n, 4000, p, s+int64(n)*1000)
				pqs = append(pqs, pq)
				naks = append(naks, float64(nak))
				bads = append(bads, bad)
			}
			_, pqS := meanStd(pqs)
			nakM, _ := meanStd(naks)
			badM, _ := meanStd(bads)
			fmt.Printf("%-6d %-10s %10.4f %10.4f %10.2f %10.4f\n",
				n, p.name, iqm(pqs), pqS, nakM, badM)
		}
	}
	fmt.Printf("DONE in %s\n", time.Since(t0).Round(time.Millisecond))
}
