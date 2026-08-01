// Massyl Benarab (ORCID: 0009-0006-9405-3374)
// Younes Aoures (ORCID: 0009-0009-6551-1294)

package consensus

import (
	"math"
	"math/rand"

	shared "dmpe/shared"
)

type RoundResult struct {
	Engine    string
	Round     int
	Proposer  string
	Finalized bool
	Votes     int
	Quorum    int
}

type Engine interface {
	Name() string


	RunRound(round int, proposerID string, obs []shared.Obs, rng *rand.Rand) RoundResult
}

func independentMerit(o shared.Obs) float64 {


	repN := float64(o.Rep) / 1000.0
	missN := float64(o.Missed) / 1000.0
	latN := float64(o.Lat) / 1000.0
	if latN > 1 {
		latN = 1
	}

	return 0.30*repN + 0.25*(1.0-missN) + 0.45*(1.0-latN)
}

func FinalizeIndependent(proposer shared.Obs, rng *rand.Rand) bool {
	m := independentMerit(proposer)

	p := 0.20 + 0.75*m
	if p < 0.05 {
		p = 0.05
	}
	if p > 0.98 {
		p = 0.98
	}
	return rng.Float64() < p
}

func findObs(obs []shared.Obs, id string) shared.Obs {
	for _, o := range obs {
		if o.ID == id {
			return o
		}
	}

	return shared.Obs{ID: id, Rep: 500, Missed: 50, Lat: 50, Load: 200, Aff: 0, LastProp: -1}
}

func quorumIBFT(n int) int {

	f := (n - 1) / 3
	return 2*f + 1
}

func quorumHotStuff(n int) int {

	f := (n - 1) / 3
	return n - f
}

func voteYes(o shared.Obs, rng *rand.Rand) bool {


	m := independentMerit(o)
	p := 0.82 + 0.16*m
	if p > 0.99 {
		p = 0.99
	}
	return rng.Float64() < p
}

func countVotes(obs []shared.Obs, rng *rand.Rand) int {
	c := 0
	for _, o := range obs {
		if voteYes(o, rng) {
			c++
		}
	}
	return c
}

type CliqueEngine struct{}

func (CliqueEngine) Name() string { return "Clique" }

func (CliqueEngine) RunRound(round int, proposerID string, obs []shared.Obs, rng *rand.Rand) RoundResult {


	pObs := findObs(obs, proposerID)
	ok := FinalizeIndependent(pObs, rng)
	return RoundResult{
		Engine:    "Clique",
		Round:     round,
		Proposer:  proposerID,
		Finalized: ok,
		Votes:     1,
		Quorum:    1,
	}
}

type IBFTEngine struct{}

func (IBFTEngine) Name() string { return "IBFT" }

func (IBFTEngine) RunRound(round int, proposerID string, obs []shared.Obs, rng *rand.Rand) RoundResult {
	n := len(obs)
	q := quorumIBFT(n)


	prep := countVotes(obs, rng)
	if prep < q {
		return RoundResult{Engine: "IBFT", Round: round, Proposer: proposerID, Finalized: false, Votes: prep, Quorum: q}
	}

	comm := countVotes(obs, rng)
	if comm < q {
		return RoundResult{Engine: "IBFT", Round: round, Proposer: proposerID, Finalized: false, Votes: comm, Quorum: q}
	}


	pObs := findObs(obs, proposerID)
	ok := FinalizeIndependent(pObs, rng)
	return RoundResult{Engine: "IBFT", Round: round, Proposer: proposerID, Finalized: ok, Votes: comm, Quorum: q}
}

type HotStuffEngine struct{}

func (HotStuffEngine) Name() string { return "HotStuff" }

func (HotStuffEngine) RunRound(round int, proposerID string, obs []shared.Obs, rng *rand.Rand) RoundResult {
	n := len(obs)
	q := quorumHotStuff(n)

	for phase := 0; phase < 4; phase++ {
		votes := countVotes(obs, rng)
		if votes < q {
			return RoundResult{Engine: "HotStuff", Round: round, Proposer: proposerID, Finalized: false, Votes: votes, Quorum: q}
		}
	}
	pObs := findObs(obs, proposerID)
	ok := FinalizeIndependent(pObs, rng)
	return RoundResult{Engine: "HotStuff", Round: round, Proposer: proposerID, Finalized: ok, Votes: q, Quorum: q}
}

func MeritRanking(o shared.Obs) float64 {
	rep := float64(o.Rep) / 1000.0
	miss := 1.0 - float64(o.Missed)/1000.0
	lat := math.Max(0, 1.0-float64(o.Lat)/200.0)
	load := 1.0 - float64(o.Load)/1000.0
	return rep * miss * lat * load
}
