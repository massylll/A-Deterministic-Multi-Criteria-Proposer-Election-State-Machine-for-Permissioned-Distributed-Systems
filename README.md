# DMPE

**Deterministic Multi-Criteria Proposer-Election State Machine for Permissioned Distributed Systems**

[![License: CC BY 4.0](https://img.shields.io/badge/License-CC%20BY%204.0-lightgrey.svg)](https://creativecommons.org/licenses/by/4.0/)

DMPE is a modular preprocessing layer for proposer election. From an immutable
observation snapshot of finalized history it applies eligibility filtering,
pairwise multi-criteria comparison, Borda aggregation, and hashed tie resolution,
then returns a unique proposer identifier without election-time consensus messages.

This repository is the companion artifact: pure election kernel, observation-aware
baselines, experimental campaigns, figures, and result logs referenced by the paper.

## Authors

- **Massyl Benarab** — ORCID [0009-0006-9405-3374](https://orcid.org/0009-0006-9405-3374)
- **Younes Aoures** — ORCID [0009-0009-6551-1294](https://orcid.org/0009-0009-6551-1294)

## License

This work is licensed under [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/).
See [LICENSE](LICENSE).

## Citation

If you use this artifact, please cite the paper:

```bibtex
@article{dmpe2026,
  title   = {DMPE: A Deterministic Multi-Criteria Proposer-Election State Machine
             for Permissioned Distributed Systems},
  author  = {Benarab, Massyl and Aoures, Younes},
  year    = {2026},
  note    = {Preprint; artifact: this repository}
  doi     = {https://doi.org/10.5281/zenodo.21730117}
}
```

Also see [`CITATION.cff`](CITATION.cff).

## Repository layout

```
shared/           Pure election kernel (package shared)
consensus/        Local Clique / IBFT / HotStuff-style engine consumers
experiments/      Determinism, baselines, rho-sweep, fallback, footprint, …
results/          Logged campaign outputs (pure-path, physical two-host, …)
figs/             Paper figures (PDF/PNG)
scripts/          Plotting and verification helpers
history-server/   HTTP history prefix service (multi-container / two-host)
node/             Replica process for multi-container determinism
docs/             Artifact and deposit notes
docker-compose.yml
```

## Requirements

- Go 1.21+
- Optional: Docker (multi-container determinism)

## Quick start

```bash
# Build the pure kernel
go build ./shared/

# Observation-aware baselines (co-varying populations)
go run ./experiments/baselines_campaign.go

# Anti-correlated population
go run ./experiments/baselines_anticorr.go

# Correlation continuum (rho-sweep)
go run ./experiments/rho_sweep.go

# Outcome-coupled integrated path
go run ./experiments/integrated_baselines.go

# Forced-fallback concentration stress
go run ./experiments/fallback_stress.go

# Multi-engine held-out FSR
go run ./experiments/multi_engine_heldout.go

# Kernel footprint microbench
go run ./experiments/footprint_bench.go
```

Multi-container determinism:

```bash
docker compose up --build
```

## Scope

- Headline quality and latency figures are single-host or controlled-harness measurements.
- Reconstruction determinism is confirmed on Docker and on two physical LAN hosts.
- Wide-area / geo-distributed finality are outside the present claims.
- Synthetic finalisation models are construct-level corroborators, not live multi-host traces.

## Contact

Please open a GitHub issue on the repository for artifact questions.
