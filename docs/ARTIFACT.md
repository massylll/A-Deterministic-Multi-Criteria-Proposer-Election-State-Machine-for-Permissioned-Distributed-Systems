# Artifact overview

Authors: Massyl Benarab (ORCID 0009-0006-9405-3374); Younes Aoures (ORCID 0009-0009-6551-1294).

License: CC BY 4.0 (see ../LICENSE).

## Contents

- `shared/pure.go` — pure deterministic election kernel
- `consensus/` — local engine consumers (Clique / IBFT / HotStuff-style)
- `experiments/` — baselines, rho-sweep, integrated path, fallback stress, footprint, multi-engine
- `results/` — campaign logs used in the paper
- `figs/` — figures
- `scripts/` — plotting and verification helpers

## Reproduction

See the repository root README for `go run` commands. Go 1.21+.

## Scope

Single-host / controlled-harness quality metrics; synthetic FSR is construct-level only;
determinism validated on Docker and two physical LAN hosts.
