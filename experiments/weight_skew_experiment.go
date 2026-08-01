// Massyl Benarab (ORCID: 0009-0006-9405-3374)
// Younes Aoures (ORCID: 0009-0009-6551-1294)

package main

import (
	"fmt"
	"math"
	"math/rand"
)

type Obs struct {
	ID     string
	Rep    int
	Missed int
	Late   int
	Load   int
	Lat    int
}

func merit(o Obs) float64 {
	rep := float64(o.Rep) / 1000.0
	miss := 1.0 - float64(o.Missed)/1000.0
	lat := math.Max(0, 1.0-float64(o.Lat)/200.0)
	load := 1.0 - float64(o.Load)/1000.0
	return rep * miss * lat * load
}

func elect(obs []Obs, wRep, wM, wL, wLoad, wLat int) int {
	n := len(obs)
	score := make([]int, n)
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			s := wRep*(obs[i].Rep-obs[j].Rep) +
				wM*(obs[j].Missed-obs[i].Missed) +
				wL*(obs[j].Late-obs[i].Late) +
				wLoad*(obs[j].Load-obs[i].Load) +
				wLat*(obs[j].Lat-obs[i].Lat)
			if s > 0 {
				score[i] += s
			} else if s < 0 {
				score[j] += -s
			}
		}
	}
	best, bi := score[0], 0
	for i := 1; i < n; i++ {
		if score[i] > best {
			best, bi = score[i], i
		}
	}
	return bi
}

func poolQuality(obs []Obs, wins []int) float64 {
	total := 0
	for _, w := range wins {
		total += w
	}
	if total == 0 {
		return 0
	}
	var pq float64
	for i, w := range wins {
		pq += (float64(w) / float64(total)) * merit(obs[i])
	}
	return pq
}

func makeSkewedPopulation(n, nSlow int, rng *rand.Rand) []Obs {
	out := make([]Obs, n)
	for i := 0; i < n; i++ {
		out[i] = Obs{
			ID:     fmt.Sprintf("V%04d", i),
			Rep:    700 + rng.Intn(200),
			Missed: rng.Intn(50),
			Late:   rng.Intn(50),
			Load:   200 + rng.Intn(200),
			Lat:    20 + rng.Intn(30),
		}
		if i < nSlow {
			out[i].Lat = 150 + rng.Intn(40)
		}
	}
	return out
}

func main() {
	const (
		n       = 16
		nSlow   = 4
		rounds  = 2000
		seed    = 42
	)
	rng := rand.New(rand.NewSource(seed))


	uniform := [5]int{1, 1, 1, 1, 1}

	latHeavy := [5]int{40, 40, 40, 20, 200}

	obs := makeSkewedPopulation(n, nSlow, rng)
	winsU := make([]int, n)
	winsL := make([]int, n)

	for r := 0; r < rounds; r++ {

		cur := make([]Obs, n)
		copy(cur, obs)
		for i := range cur {
			cur[i].Rep = clamp(cur[i].Rep+rng.Intn(21)-10, 0, 1000)
			cur[i].Load = clamp(cur[i].Load+rng.Intn(21)-10, 0, 1000)
		}
		wu := elect(cur, uniform[0], uniform[1], uniform[2], uniform[3], uniform[4])
		wl := elect(cur, latHeavy[0], latHeavy[1], latHeavy[2], latHeavy[3], latHeavy[4])
		winsU[wu]++
		winsL[wl]++
	}

	pqU := poolQuality(obs, winsU)
	pqL := poolQuality(obs, winsL)


	slowU, slowL := 0, 0
	for i := 0; i < nSlow; i++ {
		slowU += winsU[i]
		slowL += winsL[i]
	}

	fmt.Printf("latency-skew experiment  n=%d  nSlow=%d  rounds=%d  seed=%d\n", n, nSlow, rounds, seed)
	fmt.Printf("uniform   PQ=%.4f  slow_class_share=%.3f\n", pqU, float64(slowU)/float64(rounds))
	fmt.Printf("lat-heavy PQ=%.4f  slow_class_share=%.3f\n", pqL, float64(slowL)/float64(rounds))
	fmt.Printf("delta_PQ (lat-heavy - uniform)=%.4f\n", pqL-pqU)
	fmt.Printf("Interpretation: lat-heavy should reduce slow_class_share and raise PQ if weights matter under skew.\n")
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
