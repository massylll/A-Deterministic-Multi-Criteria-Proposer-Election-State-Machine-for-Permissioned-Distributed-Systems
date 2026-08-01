# Massyl Benarab (ORCID: 0009-0006-9405-3374)
# Younes Aoures (ORCID: 0009-0009-6551-1294)

"""Verify proposer agreement across multi-host node logs; split cross-host vs same-host."""
import json
import sys
from collections import defaultdict
def host_of(node: str) -> str:
    return node.split("-")[0]
def main(path: str) -> int:
    by_round = defaultdict(dict)
    elect, pull = [], []
    with open(path, encoding="utf-8", errors="ignore") as f:
        for line in f:
            line = line.strip()
            if not line.startswith("{"):
                continue
            try:
                r = json.loads(line)
            except json.JSONDecodeError:
                continue
            if not all(k in r for k in ("round", "node", "proposer")):
                continue
            by_round[r["round"]][r["node"]] = r["proposer"]
            if "elect_us" in r:
                elect.append(float(r["elect_us"]))
            if "pull_us" in r:
                pull.append(float(r["pull_us"]))
    if not by_round:
        print("ERROR: no JSON result lines found")
        return 1
    g_mis = x_mis = s_mis = 0
    for rnd, nodes in sorted(by_round.items()):
        if len(set(nodes.values())) != 1:
            g_mis += 1
            print(f"MISMATCH round={rnd}: {nodes}")
        items = list(nodes.items())
        for i in range(len(items)):
            for j in range(i + 1, len(items)):
                ni, pi = items[i]
                nj, pj = items[j]
                if pi == pj:
                    continue
                if host_of(ni) != host_of(nj):
                    x_mis += 1
                else:
                    s_mis += 1
    nproc = len(next(iter(by_round.values())))
    print(f"rounds           : {len(by_round)}")
    print(f"processes/round  : {nproc}")
    print(f"mismatches_global: {g_mis}")
    print(f"pair_mismatch_cross_host: {x_mis}")
    print(f"pair_mismatch_same_host : {s_mis}")
    if elect:
        print(f"elect_us mean    : {sum(elect)/len(elect):.1f} us")
    if pull:
        print(f"pull_us  mean    : {sum(pull)/len(pull):.1f} us")
    return 0 if g_mis == 0 else 2
if __name__ == "__main__":
    sys.exit(main(sys.argv[1] if len(sys.argv) > 1 else "all.jsonl"))
