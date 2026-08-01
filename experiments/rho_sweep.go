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

const chainID = "rho-sweep"

func clamp(x, lo, hi int) int {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}

func gauss(rng *rand.Rand) float64 {
	u1 := rng.Float64()
	if u1 < 1e-12 {
		u1 = 1e-12
	}
	u2 := rng.Float64()
	return math.Sqrt(-2*math.Log(u1)) * math.Cos(2*math.Pi*u2)
}

func ncdf(x float64) float64 {
	return 0.5 * (1 + math.Erf(x/math.Sqrt2))
}

func merit(o shared.Obs) float64 {
	rep := float64(o.Rep) / 1000.0
	miss := 1.0 - float64(o.Missed)/1000.0
	lat := math.Max(0, 1.0-float64(o.Lat)/200.0)
	load := 1.0 - float64(o.Load)/1000.0
	return rep * miss * lat * load
}

func makeCorrelated(n int, rho float64, rng *rand.Rand) []shared.Obs {
	if rho > 1 {
		rho = 1
	}
	if rho < -1 {
		rho = -1
	}
	s := math.Sqrt(math.Max(0, 1-rho*rho))
	out := make([]shared.Obs, n)
	for i := 0; i < n; i++ {
		z1 := gauss(rng)
		z2 := gauss(rng)
		zLat := rho*z1 + s*z2

		uRep := ncdf(z1)
		uLat := ncdf(zLat)
		rep := clamp(int(200+uRep*700), 0, 1000)
		lat := clamp(int(20+uLat*350), 0, 480)
		missed := clamp(int(float64(lat)*0.35+rng.Float64()*30), 0, 280)
		load := clamp(100+rng.Intn(250), 0, 1000)
		out[i] = shared.Obs{
			ID:       fmt.Sprintf("V%04d", i),
			Rep:      rep,
			Missed:   missed,
			Lat:      lat,
			Load:     load,
			Aff:      0,
			LastProp: -1,
		}
	}
	return out
}

func empiricalCorr(obs []shared.Obs) float64 {
	n := float64(len(obs))
	var mR, mL float64
	for _, o := range obs {
		mR += float64(o.Rep)
		mL += float64(o.Lat)
	}
	mR /= n
	mL /= n
	var num, dR, dL float64
	for _, o := range obs {
		dr := float64(o.Rep) - mR
		dl := float64(o.Lat) - mL
		num += dr * dl
		dR += dr * dr
		dL += dl * dl
	}
	if dR < 1e-9 || dL < 1e-9 {
		return 0
	}
	return num / math.Sqrt(dR*dL)
}

func applyMildNoise(obs []shared.Obs, rng *rand.Rand) {
	for i := range obs {
		obs[i].Rep = clamp(obs[i].Rep+rng.Intn(11)-5, 0, 1000)
		obs[i].Missed = clamp(obs[i].Missed+rng.Intn(7)-3, 0, 280)
		obs[i].Lat = clamp(obs[i].Lat+rng.Intn(7)-3, 0, 480)
	}
}

func poolQuality(counts map[string]int, merits map[string]float64, R int) float64 {
	var pq float64
	for id, c := range counts {
		pq += (float64(c) / float64(R)) * merits[id]
	}
	return pq
}

func iqm(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
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
	if len(xs) == 0 {
		return 0, 0
	}
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
		{"Clique", func(obs []shared.Obs, r int) string {
			return shared.ElectClique(obs, r)
		}},
	}
}

func runOnce(n, R int, rho float64, pol pol, seed int64) (pq float64, empRho float64) {
	rng := rand.New(rand.NewSource(seed))
	obs := makeCorrelated(n, rho, rng)
	empRho = empiricalCorr(obs)
	counts := make(map[string]int, n)
	merits := make(map[string]float64, n)
	for i := range obs {
		merits[obs[i].ID] = merit(obs[i])
		counts[obs[i].ID] = 0
	}
	for r := 1; r <= R; r++ {
		applyMildNoise(obs, rng)
		id := pol.elect(obs, r)
		counts[id]++
		for i := range obs {
			if obs[i].ID == id {
				obs[i].LastProp = r
			}
		}
	}
	return poolQuality(counts, merits, R), empRho
}

func main() {
	fmt.Println("=== Correlation continuum (ρ-sweep) ===")
	fmt.Println("Gaussian-copula corr(Rep,Lat)=ρ; R=2000; seeds=9; n=16,40")
	fmt.Println("ScalarU = unit-weight linear (fair counterpart to uniform Borda)")
	fmt.Println()

	rhos := []float64{-1, -0.75, -0.5, -0.25, 0, 0.25, 0.5, 0.75, 1}
	seeds := make([]int64, 9)
	for i := range seeds {
		seeds[i] = 100 + int64(i)
	}
	pols := policies()

	t0 := time.Now()
	for _, n := range []int{16, 40} {
		fmt.Printf("--- n=%d ---\n", n)
		fmt.Printf("%6s %8s", "rho", "empRho")
		for _, p := range pols {
			fmt.Printf(" %10s", p.name+"_iqm")
		}
		fmt.Printf(" %10s %10s\n", "dPQ_SU", "dPQ_S")
		for _, rho := range rhos {
			pqByPol := make(map[string][]float64)
			var emp []float64
			for _, p := range pols {
				for si, s := range seeds {
					pq, er := runOnce(n, 2000, rho, p, s+int64(n)*10000+int64(si)*97+int64(rho*1000))
					pqByPol[p.name] = append(pqByPol[p.name], pq)
					if p.name == "DMPE" {
						emp = append(emp, er)
					}
				}
			}
			empM, _ := meanStd(emp)
			fmt.Printf("%6.2f %8.3f", rho, empM)
			for _, p := range pols {
				fmt.Printf(" %10.4f", iqm(pqByPol[p.name]))
			}
			dSU := iqm(pqByPol["DMPE"]) - iqm(pqByPol["ScalarU"])
			dS := iqm(pqByPol["DMPE"]) - iqm(pqByPol["Scalar"])
			fmt.Printf(" %10.4f %10.4f\n", dSU, dS)
		}
		fmt.Println()
	}
	fmt.Printf("DONE in %s\n", time.Since(t0).Round(time.Millisecond))
}
