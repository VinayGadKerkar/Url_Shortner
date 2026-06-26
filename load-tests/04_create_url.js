/**
 * TEST 4 — URL creation throughput
 *
 * Measures how fast the POST /shorten endpoint handles concurrent writes.
 * Metric for resume: "Processed X URL creation requests/sec with <Yms p95 latency"
 */
import http    from 'k6/http'
import { check, sleep } from 'k6'
import { Trend, Counter } from 'k6/metrics'

const createLatency = new Trend('create_url_ms', true)
const successCount  = new Counter('successful_creates')
const BASE          = 'http://localhost:8080'

export const options = {
  stages: [
    { duration: '5s',  target: 5  },
    { duration: '15s', target: 20 },
    { duration: '10s', target: 20 },
    { duration: '5s',  target: 0  },
  ],
  thresholds: {
    create_url_ms:     ['p(95)<500'],
    successful_creates: ['count>50'],
  },
}

let counter = 0

export default function () {
  const uniq = `https://example.com/test/${__VU}-${__ITER}-${Date.now()}`
  const res  = http.post(
    `${BASE}/api/v1/shorten`,
    JSON.stringify({ long_url: uniq }),
    { headers: { 'Content-Type': 'application/json' } },
  )

  const ok = check(res, {
    'status 201': (r) => r.status === 201,
    'has short_code': (r) => {
      try { return JSON.parse(r.body).short_code !== undefined } catch { return false }
    },
  })

  if (ok) {
    createLatency.add(res.timings.duration)
    successCount.add(1)
  }
  sleep(0.1)
}

export function handleSummary(data) {
  const lat  = data.metrics.create_url_ms
  const reqs = data.metrics.http_reqs

  const avg   = lat?.values?.avg?.toFixed(2)
  const p95   = lat?.values?.['p(95)']?.toFixed(2)
  const p99   = lat?.values?.['p(99)']?.toFixed(2)
  const total = data.metrics.successful_creates?.values?.count
  const rps   = reqs?.values?.rate?.toFixed(1)

  const summary = `
╔══════════════════════════════════════════════════════╗
║          URL CREATION THROUGHPUT RESULTS             ║
╠══════════════════════════════════════════════════════╣
║  Successful creates: ${total}                          ║
║  Avg latency:        ${avg}ms                         ║
║  p95 latency:        ${p95}ms                         ║
║  p99 latency:        ${p99}ms                         ║
║  Write RPS:          ${rps} req/s                      ║
╚══════════════════════════════════════════════════════╝

RESUME BULLET:
  API handles ${rps} concurrent URL creation requests/sec
  at p95=${p95}ms write latency (k6 load test, 20 VUs)
`
  console.log(summary)
  return {
    'results/create_url_result.txt': summary,
    stdout: '\n',
  }
}
