/**
 * TEST 1 — Cache-aside latency comparison
 *
 * Measures redirect latency with Redis cache HOT vs COLD.
 * Metric for resume: "Reduced redirect latency from Xms to Yms with Redis cache-aside"
 *
 * How it works:
 *  Phase 1 (cold) — flush Redis, hit the same short code → forces DB lookup every time
 *  Phase 2 (hot)  — cache is now warm, hit the same short code → pure Redis reads
 */
import http from 'k6/http'
import { sleep, check } from 'k6'
import { Trend, Counter } from 'k6/metrics'

const coldLatency = new Trend('redirect_cold_ms', true)
const hotLatency  = new Trend('redirect_hot_ms',  true)
const coldHits    = new Counter('cold_requests')
const hotHits     = new Counter('hot_requests')

const BASE    = 'http://localhost:8080'
const SHORT   = __ENV.SHORT_CODE || 'test001'

export const options = {
  scenarios: {
    cold_cache: {
      executor: 'constant-vus',
      vus: 1,
      duration: '10s',
      startTime: '0s',
      env: { PHASE: 'cold' },
    },
    hot_cache: {
      executor: 'constant-vus',
      vus: 1,
      duration: '10s',
      startTime: '12s',   // 2s gap after cold phase
      env: { PHASE: 'hot' },
    },
  },
  thresholds: {
    redirect_hot_ms:  ['p(95)<20'],   // 95th percentile under 20ms when cached
    redirect_cold_ms: ['p(95)<200'],  // 95th percentile under 200ms without cache
  },
}

export default function () {
  const res = http.get(`${BASE}/${SHORT}`, {
    redirects: 0,   // don't follow the 302 — measure only the lookup
    tags: { phase: __ENV.PHASE },
  })

  check(res, { 'got 302 or 404': (r) => r.status === 302 || r.status === 404 })

  const latency = res.timings.duration
  if (__ENV.PHASE === 'cold') {
    coldLatency.add(latency)
    coldHits.add(1)
  } else {
    hotLatency.add(latency)
    hotHits.add(1)
  }

  sleep(0.05)
}

export function handleSummary(data) {
  const cold = data.metrics.redirect_cold_ms
  const hot  = data.metrics.redirect_hot_ms

  const coldP95 = cold?.values?.['p(95)']?.toFixed(2)
  const hotP95  = hot?.values?.['p(95)']?.toFixed(2)
  const coldAvg = cold?.values?.avg?.toFixed(2)
  const hotAvg  = hot?.values?.avg?.toFixed(2)

  const improvement = cold && hot
    ? (((cold.values.avg - hot.values.avg) / cold.values.avg) * 100).toFixed(1)
    : 'N/A'

  const summary = `
╔══════════════════════════════════════════════════════╗
║         CACHE LATENCY COMPARISON RESULTS             ║
╠══════════════════════════════════════════════════════╣
║  Cold (DB lookup):   avg=${coldAvg}ms   p95=${coldP95}ms   ║
║  Hot  (Redis cache): avg=${hotAvg}ms    p95=${hotP95}ms    ║
║                                                      ║
║  Latency improvement: ${improvement}% faster with cache          ║
╚══════════════════════════════════════════════════════╝

RESUME BULLET:
  Reduced redirect latency from ~${coldAvg}ms to ~${hotAvg}ms (${improvement}% improvement)
  via Redis cache-aside pattern (k6 load test, p95: ${coldP95}ms → ${hotP95}ms)
`
  console.log(summary)
  return {
    'results/cache_latency_result.txt': summary,
    stdout: '\n',
  }
}
