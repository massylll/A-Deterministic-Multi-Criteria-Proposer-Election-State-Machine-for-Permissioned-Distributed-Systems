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

const chainID = "fallback-stress"

func clamp(x, lo, hi int) int {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}

type regime struct {
	name      string
	latBase   int
	missBase  int
	latSpread int

	forceOver float64
}

func makeRegime(n int, reg regime, rng *rand.Rand) []shared.Obs {
	out := make([]shared.Obs, n)
	nOver := int(float64(n) * reg.forceOver)
	for i := 0; i < n; i++ {
		lat := clamp(reg.latBase+rng.Intn(reg.latSpread+1), 0, 1000)
		miss := clamp(reg.missBase+rng.Intn(40), 0, 1000)
		if i < nOver {
			lat = clamp(shared.LateShield+50+rng.Intn(100), 0, 1000)
			miss = clamp(shared.MissShield+20+rng.Intn(50), 0, 1000)
		}
		out[i] = shared.Obs{
			ID:       fmt.Sprintf("V%04d", i),
			Rep:      clamp(400+rng.Intn(400), 0, 1000),
			Missed:   miss,
			Lat:      lat,
			Load:     clamp(100+rng.Intn(200), 0, 1000),
			Aff:      0,
			LastProp: -1,
		}
	}
	return out
}

func maxShare(counts map[string]int, R int) float64 {
	maxC := 0
	for _, c := range counts {
		if c > maxC {
			maxC = c
		}
	}
	return float64(maxC) / float64(R)
}

func gini(counts map[string]int, R int) float64 {

	arr := make([]float64, 0, len(counts))
	for _, c := range counts {
		arr = append(arr, float64(c)/float64(R))
	}
	sort.Float64s(arr)
	n := float64(len(arr))
	var num float64
	for i, x := range arr {
		num += (2*float64(i+1) - n - 1) * x
	}
	return num / n
}

func run(n, R int, reg regime, seed int64) (fbRate, maxSh, meanElig float64, unique int) {
	rng := rand.New(rand.NewSource(seed))
	obs := makeRegime(n, reg, rng)
	counts := make(map[string]int, n)
	for i := range obs {
		counts[obs[i].ID] = 0
	}
	fb := 0
	var eligSum int
	for r := 1; r <= R; r++ {
		ec := shared.EligibleCount(obs, r)
		eligSum += ec
		if ec == 0 {
			fb++
		}
		id, _ := shared.ElectUniformBorda(obs, r, chainID)
		counts[id]++
		for i := range obs {
			if obs[i].ID == id {
				obs[i].LastProp = r

				if rng.Float64() < 0.05 {
					obs[i].Lat = clamp(obs[i].Lat-30, 0, 1000)
					obs[i].Missed = clamp(obs[i].Missed-10, 0, 1000)
				}
			}
		}
	}
	uniq := 0
	for _, c := range counts {
		if c > 0 {
			uniq++
		}
	}
	return float64(fb) / float64(R), maxShare(counts, R), float64(eligSum) / float64(R), uniq
}

func main() {
	fmt.Println("=== Forced-fallback concentration stress ===")
	fmt.Println("Measures fallback rate and max per-validator share under DMPE.")
	fmt.Println("Theoretical 1/S bound applies only when fallback rate = 0.")
	fmt.Println()

	regs := []regime{
		{"benign", 30, 10, 40, 0.0},
		{"mild_shield", 200, 80, 80, 0.25},
		{"heavy_shield", 350, 150, 100, 0.55},
		{"near_total", 450, 220, 80, 0.85},
		{"all_over", 600, 350, 50, 1.0},
	}

	fmt.Printf("%-12s %-4s %8s %8s %8s %8s %8s %8s\n",
		"regime", "n", "fb_rate", "maxShare", "1/S", "meanElig", "uniq", "ok_bound")

	seeds := []int64{7, 11, 13, 17, 19}
	t0 := time.Now()
	for _, n := range []int{16, 40} {
		S := float64(shared.SL(n))
		bound := 1.0 / S
		for _, reg := range regs {
			var fbs, maxs, eligs []float64
			var uniqs []float64
			for _, s := range seeds {
				fb, ms, me, uq := run(n, 2000, reg, s+int64(n)*100)
				fbs = append(fbs, fb)
				maxs = append(maxs, ms)
				eligs = append(eligs, me)
				uniqs = append(uniqs, float64(uq))
			}
			var fbM, msM, elM, uqM float64
			for i := range fbs {
				fbM += fbs[i]
				msM += maxs[i]
				elM += eligs[i]
				uqM += uniqs[i]
			}
			k := float64(len(seeds))
			fbM /= k
			msM /= k
			elM /= k
			uqM /= k
			ok := "n/a"
			if fbM < 0.01 {
				if msM <= bound+0.02 {
					ok = "yes"
				} else {
					ok = "VIOL"
				}
			} else if fbM > 0.5 {

				ok = "fb"
			} else {
				ok = "mixed"
			}
			fmt.Printf("%-12s %-4d %8.3f %8.4f %8.4f %8.2f %8.1f %8s\n",
				reg.name, n, fbM, msM, bound, elM, uqM, ok)
		}
	}

	fmt.Println()
	fmt.Println("Under all_over, selection should approach full-set RR (proportional).")
	fmt.Printf("Expected max share ≈ 1/n (plus cooldown interaction when recovery fires).\n")
	fmt.Printf("DONE in %s\n", time.Since(t0).Round(time.Millisecond))
	_ = math.Sqrt2
}
