// Massyl Benarab (ORCID: 0009-0006-9405-3374)
// Younes Aoures (ORCID: 0009-0009-6551-1294)

package main

import (
	"fmt"
	"math/rand"
	"sort"
	"time"

	"dmpe/consensus"
	shared "dmpe/shared"
)

const chainID = "multi-engine-demo"

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
		out[i] = shared.Obs{
			ID:       fmt.Sprintf("V%04d", i),
			Rep:      clamp(350+band*160+rng.Intn(50), 0, 1000),
			Missed:   clamp(5+(3-band)*40+rng.Intn(15), 0, 1000),
			Lat:      clamp(10+(3-band)*55+rng.Intn(20), 0, 1000),
			Load:     clamp(120+rng.Intn(250), 0, 1000),
			Aff:      0,
			LastProp: -1,
		}
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
		cur[i].LastProp = base[i].LastProp
	}
	return cur
}

func recordOutcome(base []shared.Obs, winner string, round int, finalized bool) {
	for i := range base {
		if base[i].ID != winner {
			continue
		}
		base[i].LastProp = round
		if finalized {
			base[i].Rep = clamp(base[i].Rep+3, 0, 1000)
		} else {
			base[i].Missed = clamp(base[i].Missed+8, 0, 1000)
			base[i].Rep = clamp(base[i].Rep-5, 0, 1000)
		}
		return
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
	m := make(map[string]float64, len(base))
	for _, o := range base {
		m[o.ID] = consensus.MeritRanking(o)
	}
	var pq float64
	for id, w := range wins {
		pq += (float64(w) / float64(total)) * m[id]
	}
	return pq
}

func runCampaign(
	label string,
	base []shared.Obs,
	rounds int,
	seed int64,
	electName string,
	elect func(obs []shared.Obs, round int) string,
	engines []consensus.Engine,
) {
	rng := rand.New(rand.NewSource(seed))
	state := make([]shared.Obs, len(base))
	copy(state, base)
	for i := range state {
		state[i].LastProp = -1
	}

	half := rounds / 2
	winsAll := make(map[string]int)
	winsHold := make(map[string]int)

	finHold := make(map[string]int)
	totHold := make(map[string]int)
	finAll := make(map[string]int)
	totAll := make(map[string]int)

	agreeRounds := 0

	for r := 1; r <= rounds; r++ {
		cur := applyNoise(state, rng)
		prop := elect(cur, r)
		winsAll[prop]++
		if r > half {
			winsHold[prop]++
		}

		propsSeen := map[string]bool{}
		var refFinal bool
		for i, eng := range engines {
			erng := rand.New(rand.NewSource(seed + int64(r)*1009 + int64(i)*17 + int64(len(eng.Name()))))
			res := eng.RunRound(r, prop, cur, erng)
			propsSeen[res.Proposer] = true
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
		if len(propsSeen) == 1 {
			agreeRounds++
		}
		recordOutcome(state, prop, r, refFinal)
	}

	fmt.Printf("\n=== %s | elect=%s | n=%d rounds=%d seed=%d ===\n",
		label, electName, len(base), rounds, seed)
	fmt.Printf("cross-engine proposer agreement: %d/%d (%.1f%%)\n",
		agreeRounds, rounds, 100*float64(agreeRounds)/float64(rounds))
	fmt.Printf("PoolQuality (all):      %.4f\n", poolQuality(base, winsAll))
	fmt.Printf("PoolQuality (held-out): %.4f\n", poolQuality(base, winsHold))
	fmt.Printf("%-10s  FSR_all   FSR_heldout\n", "engine")
	names := []string{"Clique", "IBFT", "HotStuff"}
	for _, name := range names {
		if totAll[name] == 0 {
			continue
		}
		fsrA := float64(finAll[name]) / float64(totAll[name])
		fsrH := float64(finHold[name]) / float64(totHold[name])
		fmt.Printf("%-10s  %.4f    %.4f\n", name, fsrA, fsrH)
	}
}

func madExperiment(n, rounds int, seed int64) {
	rng := rand.New(rand.NewSource(seed))
	base := makeHeterogeneous(n, rng)
	state := make([]shared.Obs, n)
	copy(state, base)

	agreeFinalized := 0
	divergeStale := 0
	for r := 1; r <= rounds; r++ {
		cur := applyNoise(state, rng)
		pA, _ := shared.ElectUniformBorda(cur, r, chainID)
		pB, _ := shared.ElectUniformBorda(cur, r, chainID)
		if pA == pB {
			agreeFinalized++
		}
		stale := make([]shared.Obs, n)
		copy(stale, cur)
		for i := range stale {
			if stale[i].LastProp > 0 && r-stale[i].LastProp < 3 {
				stale[i].LastProp = -1
			}
		}
		pStale, _ := shared.ElectUniformBorda(stale, r, chainID)
		if pStale != pA {
			divergeStale++
		}
		recordOutcome(state, pA, r, true)
	}
	fmt.Printf("\n=== MAD / finality-boundary  n=%d  rounds=%d ===\n", n, rounds)
	fmt.Printf("agreement on identical finalized snapshot: %d/%d\n", agreeFinalized, rounds)
	fmt.Printf("divergence when Domain B uses stale prefix:  %d/%d\n", divergeStale, rounds)
	fmt.Println("Interpretation: DMPE requires a finalized prefix (Theorem 1);")
	fmt.Println("stale non-finalized views can diverge — that is consensus's responsibility.")
}

func scaleLatency(sizes []int, trials int, seed int64) {
	fmt.Printf("\n=== Pure-path election latency (repeated trials, uniform Borda) ===\n")
	fmt.Printf("%-6s  mean_us  p95_us\n", "N")
	for _, n := range sizes {
		rng := rand.New(rand.NewSource(seed + int64(n)*13))
		base := makeHeterogeneous(n, rng)
		samples := make([]float64, 0, trials)

		for w := 0; w < 20; w++ {
			_, _ = shared.ElectUniformBorda(base, w+1, chainID)
		}
		for t := 0; t < trials; t++ {
			cur := applyNoise(base, rng)
			t0 := time.Now()
			_, _ = shared.ElectUniformBorda(cur, t+1, chainID)
			samples = append(samples, float64(time.Since(t0).Microseconds()))
		}
		sort.Float64s(samples)
		mean := 0.0
		for _, s := range samples {
			mean += s
		}
		mean /= float64(len(samples))
		p95 := samples[int(0.95*float64(len(samples)-1))]
		fmt.Printf("%-6d  %7.1f  %6.1f\n", n, mean, p95)
	}
}

func main() {
	engines := []consensus.Engine{
		consensus.CliqueEngine{},
		consensus.IBFTEngine{},
		consensus.HotStuffEngine{},
	}

	rng := rand.New(rand.NewSource(42))
	het16 := makeHeterogeneous(16, rng)
	het32 := makeHeterogeneous(32, rand.New(rand.NewSource(43)))

	dmpe := func(obs []shared.Obs, round int) string {
		id, _ := shared.ElectUniformBorda(obs, round, chainID)
		return id
	}
	cliqueElect := func(obs []shared.Obs, round int) string {
		return shared.ElectClique(obs, round)
	}

	const rounds = 4000

	runCampaign("DMPE→{Clique,IBFT,HotStuff}", het16, rounds, 1001, "uniform-Borda", dmpe, engines)
	runCampaign("Clique-elect→{Clique,IBFT,HotStuff}", het16, rounds, 1002, "round-robin", cliqueElect, engines)
	runCampaign("DMPE n=32", het32, rounds, 1003, "uniform-Borda", dmpe, engines)

	madExperiment(16, 2000, 2001)
	scaleLatency([]int{16, 32, 40, 70, 100, 200, 300, 500}, 200, 3001)
}
