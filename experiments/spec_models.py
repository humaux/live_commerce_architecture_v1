#!/usr/bin/env python3
"""Executable *specification models*, not the SaaS implementation.

No network, database, real payment, ad, model, or shipping request is made.
Safety removals below are mutation/negative-control tests. Only the queue model
is an optimization ablation. Passing does not certify production correctness.
Run: python3 experiments/spec_models.py --out experiments/results
"""
from __future__ import annotations
import argparse
import hashlib
import itertools
import json
import math
import platform
from collections import defaultdict, deque
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Callable

Result = dict[str, Any]

def dedup(protected: bool) -> Result:
    seen: set[tuple[str, str, str]] = set()
    actions: list[str] = []
    for event in [('A', 'page-A', 'c1')] * 3:
        if protected and event in seen:
            continue
        seen.add(event)
        actions.append('cart-update')
    return {'actions': len(actions), 'expected_actions': 1}

def tenant_scope(protected: bool) -> Result:
    rows = [{'tenant': 'B', 'external': 'c1', 'value': 'B-secret'},
            {'tenant': 'A', 'external': 'c1', 'value': 'A-value'}]
    matches = [r for r in rows if r['external'] == 'c1' and
               (not protected or r['tenant'] == 'A')]
    return {'returned_tenant': matches[0]['tenant'], 'authorized_tenant': 'A'}

def atomic_intent(protected: bool) -> Result:
    observations = []
    # Crashes at three well-defined cuts; model atomicity, not a real WAL test.
    for cut in ('before_commit', 'after_commit', 'after_schedule'):
        order = cut != 'before_commit'
        job = order if protected else cut == 'after_schedule'
        observations.append({'cut': cut, 'order': order, 'job': job})
    return {'observations': observations,
            'lost_jobs': sum(r['order'] and not r['job'] for r in observations)}

def stock(protected: bool) -> Result:
    schedules = []
    for p in itertools.permutations(('A_read', 'A_commit', 'B_read', 'B_commit')):
        if p.index('A_read') < p.index('A_commit') and p.index('B_read') < p.index('B_commit'):
            schedules.append(p)
    bad, examples = 0, []
    for schedule in schedules:
        available, allocated = 1, 0
        observed: dict[str, int] = {}
        for op in schedule:
            actor, stage = op.split('_')
            if stage == 'read':
                observed[actor] = available
            else:
                admissible = available > 0 if protected else observed[actor] > 0
                if admissible:
                    # Unprotected path is a stale write: counter can look fine
                    # while two reservations have been issued for one unit.
                    available = available - 1 if protected else observed[actor] - 1
                    allocated += 1
        if allocated > 1 or available < 0:
            bad += 1
            examples.append({'schedule': schedule, 'allocated': allocated, 'available': available})
    return {'schedules': len(schedules), 'violations': bad, 'counterexamples': examples}

def consent(protected: bool) -> Result:
    granted_at_enqueue, granted_at_dispatch = True, False
    sent = granted_at_dispatch if protected else granted_at_enqueue
    return {'sent_after_revocation': sent}

def binding(protected: bool) -> Result:
    frozen = ('A', 'account-A', 1)
    current = ('B', 'account-B', 2)
    if protected and frozen != current:
        return {'sent': False, 'charged_tenant': None, 'state': 'STALE_BINDING'}
    return {'sent': True, 'charged_tenant': current[0], 'state': 'ACKNOWLEDGED'}

def unknown_outcome(protected: bool) -> Result:
    # Provider commits purchase and the response is lost. This provider has no
    # contractual idempotency/query guarantee in the model.
    effects = 1
    if protected:
        return {'external_purchases': effects, 'state': 'UNKNOWN', 'auto_retry': False}
    effects += 1
    return {'external_purchases': effects, 'state': 'ACKNOWLEDGED', 'auto_retry': True}

def link_identity(protected: bool) -> Result:
    # Bob pays using Alice's forwarded link. Attribution is retained but is not
    # evidence of account ownership.
    return {'customer_order_owner': 'Bob' if protected else 'Alice',
            'touchpoint_owner': 'Alice', 'payer': 'Bob',
            'identity_merge': not protected}

def late_payment(protected: bool) -> Result:
    # A's reservation expired; B has consumed the only unit. A later pays.
    return {'stock': 1, 'fulfillable_orders': ['B'] if protected else ['B', 'A'],
            'A_state': 'PAID_ALLOCATION_FAILED_REFUND_PENDING' if protected else 'FULFILLABLE'}

def cache_scope(protected: bool) -> Result:
    cache: dict[tuple[str, ...], str] = {}
    def key(host: str) -> tuple[str, ...]:
        return (host, '/p/101', 'USD', 'en') if protected else ('/p/101', 'USD', 'en')
    cache[key('a.example.test')] = 'product-A'
    value = cache.get(key('b.example.test'), 'product-B')
    return {'B_page': value}

def send_expiry(protected: bool) -> Result:
    # A queued private reply loses eligibility before the provider call begins.
    eligible_on_enqueue, live_at_dispatch = True, False
    sent = live_at_dispatch if protected else eligible_on_enqueue
    return {'sent_after_live_ended': sent}

def payment_account(protected: bool) -> Result:
    expected = ('A', 'acct-A', 'USD', 1900)
    received = ('A', 'acct-B', 'USD', 1900)
    return {'marked_paid': (received == expected) if protected else received[0] == expected[0]}

CASES: list[tuple[str, str, Callable[[bool], Result], Callable[[Result], bool]]] = [
    ('M01', 'comment-idempotency', dedup, lambda r: r['actions'] == 1),
    ('M02', 'tenant-scoped-lookup', tenant_scope, lambda r: r['returned_tenant'] == r['authorized_tenant']),
    ('M03', 'transactional-intent', atomic_intent, lambda r: r['lost_jobs'] == 0),
    ('M04', 'atomic-stock-reservation', stock, lambda r: r['violations'] == 0),
    ('M05', 'dispatch-consent-recheck', consent, lambda r: not r['sent_after_revocation']),
    ('M06', 'frozen-credential-binding', binding, lambda r: not r['sent'] and r['state'] == 'STALE_BINDING'),
    ('M07', 'unknown-outcome-no-blind-retry', unknown_outcome, lambda r: r['external_purchases'] == 1 and r['state'] == 'UNKNOWN'),
    ('M08', 'attribution-not-identity', link_identity, lambda r: r['customer_order_owner'] == r['payer'] and not r['identity_merge']),
    ('M09', 'late-payment-no-oversell', late_payment, lambda r: len(r['fulfillable_orders']) <= r['stock']),
    ('M10', 'hostname-scoped-cache', cache_scope, lambda r: r['B_page'] == 'product-B'),
    ('M11', 'message-expiry-at-dispatch', send_expiry, lambda r: not r['sent_after_live_ended']),
    ('M12', 'payment-account-binding', payment_account, lambda r: not r['marked_paid']),
]

def p_nearest(values: list[int], p: float) -> int:
    return sorted(values)[max(0, math.ceil(p * len(values)) - 1)]

def queue_variant(fair: bool) -> Result:
    # Completely controlled discrete-event model: 510 unit-time jobs, arrival
    # time zero, one service slot. Tenant A's 500 jobs arrive before B's 10.
    jobs = [('A', n) for n in range(500)] + [('B', n) for n in range(10)]
    if fair:
        queues: dict[str, deque[tuple[str, int]]] = defaultdict(deque)
        for j in jobs:
            queues[j[0]].append(j)
        schedule = []
        names = deque(queues)
        while names:
            name = names.popleft()
            schedule.append(queues[name].popleft())
            if queues[name]:
                names.append(name)
    else:
        schedule = jobs
    finish: dict[str, list[int]] = defaultdict(list)
    for t, j in enumerate(schedule, start=1):
        finish[j[0]].append(t)
    return {'variant': 'round_robin' if fair else 'fifo', 'jobs': len(schedule),
            'total_service_ticks': len(schedule),
            'overall_mean_completion_ticks': sum(sum(v) for v in finish.values()) / len(schedule),
            'by_tenant': {k: {'n': len(v), 'mean_completion_ticks': sum(v)/len(v),
                              'p95_completion_ticks': p_nearest(v, .95),
                              'max_completion_ticks': max(v)} for k, v in finish.items()}}

def run(out: Path) -> dict[str, Any]:
    out.mkdir(parents=True, exist_ok=True)
    if len(CASES) != 12 or len({c[0] for c in CASES}) != 12:
        raise SystemExit("Expected 12 distinct specification cases; an empty suite is not a pass")
    safety = []
    for case_id, name, experiment, oracle in CASES:
        baseline = experiment(True)
        mutant = experiment(False)
        baseline_ok = bool(oracle(baseline))
        mutant_killed = not bool(oracle(mutant))
        safety.append({'id': case_id, 'mechanism': name, 'baseline_pass': baseline_ok,
                       'mutant_killed': mutant_killed, 'baseline_observations': baseline,
                       'mutant_observations': mutant})
    fair, fifo = queue_variant(True), queue_variant(False)
    source = Path(__file__).read_bytes()
    result = {'schema_version': 1,
              'executed_at_utc': datetime.now(timezone.utc).isoformat(),
              'environment': {'python': platform.python_version(), 'platform': platform.platform()},
              'source_sha256': hashlib.sha256(source).hexdigest(),
              'evidence_level': 'EXECUTABLE_SPEC_MODEL_ONLY',
              'not_tested': ['production_code', 'PostgreSQL', 'River', 'Go/Rust/Node_language_comparison',
                             'Meta', 'LiveKit', 'Stripe', 'carriers', 'Codex_parallel_execution'],
              'safety_negative_controls': safety,
              'optimization_ablation': {'id': 'A00', 'scope': 'synthetic_queue_model',
                                       'full': fair, 'without_fair_scheduler': fifo},
              'summary': {'baselines_passed': sum(r['baseline_pass'] for r in safety),
                          'mutants_killed': sum(r['mutant_killed'] for r in safety),
                          'cases': len(safety)}}
    (out / 'spec-results.json').write_text(json.dumps(result, ensure_ascii=False, indent=2)+'\n', encoding='utf-8')
    lines = ['# 可执行规格模型实验记录', '',
             '**范围：Python 离散模型；不是 SaaS 产品、数据库、真实平台或语言性能验收。**', '',
             f"运行时间（UTC）：`{result['executed_at_utc']}`", '',
             f"脚本 SHA-256：`{result['source_sha256']}`", '',
             '## 安全机制的负对照／变异测试', '',
             '| ID | 删除的机制 | 基线满足判据 | 删除后被判据捕获 |',
             '|---|---|---|---|']
    for r in safety:
        lines.append(f"| {r['id']} | {r['mechanism']} | {r['baseline_pass']} | {r['mutant_killed']} |")
    lines += ['', '这些机制不得在生产关闭。测试仅说明预设反例在本规格模型中被识别；',
              '不能证明所有故障已覆盖，也不能证明真实实现正确。原始观测见 JSON。', '',
              '## A00：公平调度优化消融（合成模型）', '',
              '相同的 510 个任务、单服务槽、每任务 1 tick、全部在时刻 0 可用。A 有 500 个、B 有 10 个。',
              '比较轮询租户调度与全局 FIFO，除调度规则外没有其他变动。', '',
              '| 变体 | B 的 P95 完成时间（tick） | 总服务时间（tick） | 全体平均完成时间（tick） |',
              '|---|---:|---:|---:|']
    for r in (fair, fifo):
        lines.append(f"| {r['variant']} | {r['by_tenant']['B']['p95_completion_ticks']} | {r['total_service_ticks']} | {r['overall_mean_completion_ticks']} |")
    lines += ['', '解释：公平性在这个刻意构造的洪峰场景中改变小租户等待，不增加总吞吐。',
              'tick 不是毫秒；这是确定性模型，无独立真实采样，不能给生产性能置信区间。',
              '不能据此宣称 River 已有相同性能，更不能据此证明 Go 比 Rust 快。', '',
              '## 尚未执行', '',
              '语言对比、真实 PostgreSQL/River 压测、LiveKit 多路推流、外部 API 和 Codex 子 Agent 的 1/2/4 并行实验均为 NOT_RUN。', '']
    (out / 'spec-results.md').write_text('\n'.join(lines), encoding='utf-8')
    if any(not r['baseline_pass'] or not r['mutant_killed'] for r in safety):
        raise SystemExit('Specification baseline or negative-control check failed')
    return result

if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', type=Path, default=Path(__file__).parent/'results')
    args = parser.parse_args()
    report = run(args.out)
    print(json.dumps(report['summary'], ensure_ascii=False))
    print('A00 B-P95:', report['optimization_ablation']['full']['by_tenant']['B']['p95_completion_ticks'],
          'vs', report['optimization_ablation']['without_fair_scheduler']['by_tenant']['B']['p95_completion_ticks'], 'ticks')
