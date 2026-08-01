# Massyl Benarab (ORCID: 0009-0006-9405-3374)
# Younes Aoures (ORCID: 0009-0009-6551-1294)

"""Verify that all nodes elected the identical proposer for every round.
Usage:
  docker compose logs --no-log-prefix 2>&1 | grep '^{' > results.jsonl
  python3 verify_determinism.py results.jsonl
"""
import json
import sys
from collections import defaultdict
def main(path: str) -> int:
    by_round = defaultdict(dict)
    elect_us = []
    pull_us = []
    total_us = []
    with open(path) as f:
        for line in f:
            line = line.strip()
            if not line.startswith("{"):
                continue
            try:
                r = json.loads(line)
            except json.JSONDecodeError:
                continue
            by_round[r["round"]][r["node"]] = r["proposer"]
            elect_us.append(r["elect_us"])
            pull_us.append(r["pull_us"])
            total_us.append(r["total_us"])
    if not by_round:
        print("ERROR: no JSON result lines found")
        return 1
    mismatches = 0
    for rnd, nodes in sorted(by_round.items()):
        proposers = set(nodes.values())
        if len(proposers) != 1:
            mismatches += 1
            print(f"MISMATCH round={rnd}: {nodes}")
        else:
            print(f"OK     round={rnd}: proposer={next(iter(proposers))}  nodes={sorted(nodes)}")
    print()
    print(f"rounds checked : {len(by_round)}")
    print(f"mismatches     : {mismatches}")
    if elect_us:
        print(f"elect_us  mean : {sum(elect_us)/len(elect_us):.1f} µs")
        print(f"pull_us   mean : {sum(pull_us)/len(pull_us):.1f} µs")
        print(f"total_us  mean : {sum(total_us)/len(total_us):.1f} µs")
    return 0 if mismatches == 0 else 2
if __name__ == "__main__":
    sys.exit(main(sys.argv[1] if len(sys.argv) > 1 else "results.jsonl"))
