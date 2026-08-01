// Massyl Benarab (ORCID: 0009-0006-9405-3374)
// Younes Aoures (ORCID: 0009-0009-6551-1294)

package shared

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sort"
)

type Obs struct {
	ID       string
	Rep      int
	Missed   int
	Lat      int
	Load     int
	Aff      int
	LastProp int
}

const (
	WR         = 100
	WM         = 120
	WL         = 60
	WEll       = 40
	WLam       = 20
	LateShield = 500
	MissShield = 300
)

func sgn(x int) int {
	if x > 0 {
		return 1
	}
	if x < 0 {
		return -1
	}
	return 0
}

func Sigma(a, b Obs) int {
	return WR*sgn(a.Rep-b.Rep) +
		WM*sgn(b.Missed-a.Missed) +
		WL*sgn(b.Lat-a.Lat) +
		WEll*sgn(b.Load-a.Load) +
		WLam*sgn(b.Aff-a.Aff)
}

func SigmaUniform(a, b Obs) int {
	return sgn(a.Rep-b.Rep) +
		sgn(b.Missed-a.Missed) +
		sgn(b.Lat-a.Lat) +
		sgn(b.Load-a.Load) +
		sgn(b.Aff-a.Aff)
}

func SigmaWeighted(a, b Obs, wR, wM, wL, wEll, wLam int) int {
	return wR*sgn(a.Rep-b.Rep) +
		wM*sgn(b.Missed-a.Missed) +
		wL*sgn(b.Lat-a.Lat) +
		wEll*sgn(b.Load-a.Load) +
		wLam*sgn(b.Aff-a.Aff)
}

func SL(n int) int { return n/2 + 1 }

func Eligible(o Obs, round int, n int) bool {
	if o.Lat > LateShield || o.Missed > MissShield {
		return false
	}
	if o.LastProp >= 0 && round-o.LastProp < SL(n) {
		return false
	}
	return true
}

func HashTie(chainID string, round int, id string) uint64 {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%s", chainID, round, id)))
	return binary.BigEndian.Uint64(h[:8])
}

func electWith(obs []Obs, round int, chainID string, sigma func(a, b Obs) int) (string, int, int) {
	n := len(obs)
	cands := make([]Obs, 0, n)
	for _, o := range obs {
		if Eligible(o, round, n) {
			cands = append(cands, o)
		}
	}


	if len(cands) == 0 {
		return ElectClique(obs, round), 0, 1
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].ID < cands[j].ID })

	scores := make([]int, len(cands))
	for i := 0; i < len(cands); i++ {
		for j := 0; j < len(cands); j++ {
			if i == j {
				continue
			}
			scores[i] += sigma(cands[i], cands[j])
		}
	}
	best := 0
	maxScore := scores[0]
	for i := 1; i < len(cands); i++ {
		if scores[i] > maxScore {
			maxScore = scores[i]
			best = i
		}
	}
	tieSet := 0
	for i := 0; i < len(cands); i++ {
		if scores[i] == maxScore {
			tieSet++
		}
	}
	for i := 0; i < len(cands); i++ {
		if scores[i] != maxScore {
			continue
		}
		if HashTie(chainID, round, cands[i].ID) < HashTie(chainID, round, cands[best].ID) {
			best = i
		}
	}
	return cands[best].ID, scores[best], tieSet
}

func ElectBorda(obs []Obs, round int, chainID string) (string, int) {
	id, sc, _ := electWith(obs, round, chainID, Sigma)
	return id, sc
}

func ElectBordaDetail(obs []Obs, round int, chainID string) (string, int, int) {
	return electWith(obs, round, chainID, Sigma)
}

func ElectUniformBorda(obs []Obs, round int, chainID string) (string, int) {
	id, sc, _ := electWith(obs, round, chainID, SigmaUniform)
	return id, sc
}

func ElectUniformBordaDetail(obs []Obs, round int, chainID string) (string, int, int) {
	return electWith(obs, round, chainID, SigmaUniform)
}

func ElectWeightedBorda(obs []Obs, round int, chainID string, wR, wM, wL, wEll, wLam int) (string, int, int) {
	sigma := func(a, b Obs) int { return SigmaWeighted(a, b, wR, wM, wL, wEll, wLam) }
	return electWith(obs, round, chainID, sigma)
}

func ElectClique(obs []Obs, round int) string {
	ids := make([]string, len(obs))
	for i, o := range obs {
		ids[i] = o.ID
	}
	sort.Strings(ids)
	return ids[(round-1)%len(ids)]
}

func eligibleCandidates(obs []Obs, round int) []Obs {
	n := len(obs)
	cands := make([]Obs, 0, n)
	for _, o := range obs {
		if Eligible(o, round, n) {
			cands = append(cands, o)
		}
	}


	sort.Slice(cands, func(i, j int) bool { return cands[i].ID < cands[j].ID })
	return cands
}

func pickMaxScore(cands []Obs, scores []int, round int, chainID string) (string, int, int) {
	best := 0
	maxScore := scores[0]
	for i := 1; i < len(cands); i++ {
		if scores[i] > maxScore {
			maxScore = scores[i]
			best = i
		}
	}
	tieSet := 0
	for i := 0; i < len(cands); i++ {
		if scores[i] == maxScore {
			tieSet++
		}
	}
	for i := 0; i < len(cands); i++ {
		if scores[i] != maxScore {
			continue
		}
		if HashTie(chainID, round, cands[i].ID) < HashTie(chainID, round, cands[best].ID) {
			best = i
		}
	}
	return cands[best].ID, scores[best], tieSet
}

func ScalarScore(o Obs) int {
	return WR*o.Rep - WM*o.Missed - WL*o.Lat - WEll*o.Load - WLam*o.Aff
}

func ElectScalar(obs []Obs, round int, chainID string) (string, int) {
	id, sc, _ := ElectScalarDetail(obs, round, chainID)
	return id, sc
}

func ElectScalarDetail(obs []Obs, round int, chainID string) (string, int, int) {
	cands := eligibleCandidates(obs, round)
	if len(cands) == 0 {
		return ElectClique(obs, round), 0, 1
	}
	scores := make([]int, len(cands))
	for i, o := range cands {
		scores[i] = ScalarScore(o)
	}
	return pickMaxScore(cands, scores, round, chainID)
}

func ElectRepOnly(obs []Obs, round int, chainID string) (string, int) {
	id, sc, _ := ElectRepOnlyDetail(obs, round, chainID)
	return id, sc
}

func ElectRepOnlyDetail(obs []Obs, round int, chainID string) (string, int, int) {
	cands := eligibleCandidates(obs, round)
	if len(cands) == 0 {
		return ElectClique(obs, round), 0, 1
	}
	scores := make([]int, len(cands))
	for i, o := range cands {
		scores[i] = o.Rep
	}
	return pickMaxScore(cands, scores, round, chainID)
}

func ElectEligibleRR(obs []Obs, round int, chainID string) string {
	_ = chainID
	cands := eligibleCandidates(obs, round)
	if len(cands) == 0 {
		return ElectClique(obs, round)
	}
	return cands[(round-1)%len(cands)].ID
}

func ScalarScoreUniform(o Obs) int {
	return o.Rep - o.Missed - o.Lat - o.Load - o.Aff
}

func ElectScalarUniform(obs []Obs, round int, chainID string) (string, int) {
	id, sc, _ := ElectScalarUniformDetail(obs, round, chainID)
	return id, sc
}

func ElectScalarUniformDetail(obs []Obs, round int, chainID string) (string, int, int) {
	cands := eligibleCandidates(obs, round)
	if len(cands) == 0 {
		return ElectClique(obs, round), 0, 1
	}
	scores := make([]int, len(cands))
	for i, o := range cands {
		scores[i] = ScalarScoreUniform(o)
	}
	return pickMaxScore(cands, scores, round, chainID)
}

func EligibleCount(obs []Obs, round int) int {
	n := len(obs)
	c := 0
	for _, o := range obs {
		if Eligible(o, round, n) {
			c++
		}
	}
	return c
}
