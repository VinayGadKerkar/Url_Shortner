/**
 * TEST 3 — Rate limiter validation
 *
 * Fires more than 60 req/min from a single IP, verifies 429s are returned.
 * Metric for resume: "Implemented Redis-backed rate limiting, blocking X% of
 * abusive traffic while allowing legitimate requests through"
 */
import http from 'k6/http'
import { check } from 'k6'
import { Counter, Rate } from 'k6/metrics'

const allowed   = new Counter('allowed_requests')
const throttled = new Counter('throttled_requests')
const errorRate = new Rate('error_rate')
const BASE      = 'http://localhost:8080'

export const options = {
  scenarios: {
    burst: {
      executor: 'constant-arrival-rate',
      rate: 120,           // 120 req/s — well above the 60/min limit
      timeUnit: '1m',
      duration: '30s',
      preAllocatedVUs: 10,
    },
  },
}

export default function () {
  const res = http.get(`${BASE}/health`)

  if (res.status === 429) {
    throttled.add(1)
    check(res, {
      'rate limit returns JSON': (r) => r.body.includes('rate limit exceeded'),
      'has Retry-After header':  (r) => r.headers['Retry-After'] !== undefined,
    })
  } else if (res.status === 200) {
    allowed.add(1)
  } else {
    errorRate.add(1)
  }
}

export function handleSummary(data) {
  const allow    = data.metrics.allowed_requests?.values?.count    || 0
  const block    = data.metrics.throttled_requests?.values?.count  || 0
  const total    = allow + block
  const blockPct = total > 0 ? ((block / total) * 100).toFixed(1) : '0'
  const allowPct = total > 0 ? ((allow / total) * 100).toFixed(1) : '0'

  const summary = `
╔══════════════════════════════════════════════════════╗
║            RATE LIMITER TEST RESULTS                 ║
╠══════════════════════════════════════════════════════╣
║  Total requests:  ${total}                               ║
║  Allowed (200):   ${allow} (${allowPct}%)                      ║
║  Throttled (429): ${block} (${blockPct}%)                     ║
╚══════════════════════════════════════════════════════╝

RESUME BULLET:
  Implemented IP-based rate limiting (60 req/min),
  correctly throttled ${blockPct}% of burst traffic in load test
`
  console.log(summary)
  return {
    'results/rate_limiter_result.txt': summary,
    stdout: '\n',
  }
}
