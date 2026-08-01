// Massyl Benarab (ORCID: 0009-0006-9405-3374)
// Younes Aoures (ORCID: 0009-0009-6551-1294)

package main

import (
	"fmt"
	"runtime"
	"time"

	shared "dmpe/shared"
)

const chainID = "footprint-bench"

func makeStaticObs(n int) []shared.Obs {
	out := make([]shared.Obs, n)
	for i := 0; i < n; i++ {
		band := i % 4
		out[i] = shared.Obs{
			ID:       fmt.Sprintf("V%04d", i),
			Rep:      550 + band*100,
			Missed:   10 + (3-band)*15,
			Lat:      15 + (3-band)*25,
			Load:     200,
			Aff:      0,
			LastProp: -1,
		}
	}
	return out
}

func benchOnce(obs []shared.Obs, rounds int) (nsPerOp float64, allocBytesPerOp uint64, allocsPerOp float64) {

	for r := 1; r <= 64; r++ {
		id, _ := shared.ElectUniformBorda(obs, r, chainID)
		for i := range obs {
			if obs[i].ID == id {
				obs[i].LastProp = r
			}
		}
	}

	for i := range obs {
		obs[i].LastProp = -1
	}

	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)

	t0 := time.Now()
	for r := 1; r <= rounds; r++ {
		id, _ := shared.ElectUniformBorda(obs, r, chainID)
		for i := range obs {
			if obs[i].ID == id {
				obs[i].LastProp = r
			}
		}
	}
	elapsed := time.Since(t0)

	runtime.ReadMemStats(&after)

	nsPerOp = float64(elapsed.Nanoseconds()) / float64(rounds)
	allocBytesPerOp = (after.TotalAlloc - before.TotalAlloc) / uint64(rounds)
	allocsPerOp = float64(after.Mallocs-before.Mallocs) / float64(rounds)
	return
}

func main() {
	fmt.Println("=== DMPE pure-kernel footprint (ElectUniformBorda) ===")
	fmt.Println("Timed path excludes warm-up; MemStats TotalAlloc/Mallocs deltas per op.")
	fmt.Println()
	fmt.Printf("%-6s %12s %14s %12s %12s\n", "n", "rounds", "ns/op", "B/op", "allocs/op")

	configs := []struct {
		n, rounds int
	}{
		{16, 20000},
		{40, 10000},
		{100, 3000},
		{200, 1000},
	}
	for _, c := range configs {
		obs := makeStaticObs(c.n)
		ns, b, a := benchOnce(obs, c.rounds)
		fmt.Printf("%-6d %12d %14.1f %12d %12.2f\n", c.n, c.rounds, ns, b, a)
	}


	fmt.Println()
	fmt.Println("--- Peak HeapAlloc after ElectUniformBorda (single shot, n=100) ---")
	runtime.GC()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	base := ms.HeapAlloc
	obs := makeStaticObs(100)
	shared.ElectUniformBorda(obs, 1, chainID)
	runtime.ReadMemStats(&ms)
	fmt.Printf("HeapAlloc before snapshot+elect: %d B\n", base)
	fmt.Printf("HeapAlloc after  snapshot+elect: %d B (delta %d B)\n", ms.HeapAlloc, int64(ms.HeapAlloc)-int64(base))
	fmt.Println("DONE")
}
