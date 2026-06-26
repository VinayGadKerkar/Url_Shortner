/**
 * TEST 2 — Peak throughput (requests per second)
 *
 * Ramps up VUs to find max sustainable RPS under load.
 * Metric for resume: "Handled Xk requests/min at <Yms p95 latency"
 */
import http from 'k6/http'
import { sleep, check } from 'k6'
import { Rate } from 'k6/metrics'

const errorRate = new Rate('errors')
const BASE      = 'http://localhost:8080'
const SHORT     = __ENV.SHORT_CODE || 'test001'

export const options = {
  stages: [
    { duration: '10s', target: 10  },   // ramp up
    { duration: '20s', target: 50  },   // ramp to 50 VUs
    { duration: '20s', target: 100 },   // ramp to 100 VUs
    { duration: '20s', target: 100 },   // hold peak
    { duration: '10s', target: 0   },   // ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<100'],   // 95% of requests under 100ms
    errors:            ['rate<0.01'],   // less than 1% error rate
  },
}

export default function () {
  const res = http.get(`${BASE}/${SHORT}`, { redirects: 0 })
  const ok  = check(res, { 'status 302 or 404': (r) => r.status === 302 || r.status === 404 })
  errorRate.add(!ok)
  sleep(0.01)
}

export function handleSummary(data) {
  const dur  = data.metrics.http_req_duration
  const reqs = data.metrics.http_reqs

  const rps    = reqs?.values?.rate?.toFixed(0)
  const p95    = dur?.values?.['p(95)']?.toFixed(2)
  const p99    = dur?.values?.['p(99)']?.toFixed(2)
  const avg    = dur?.values?.avg?.toFixed(2)
  const total  = reqs?.values?.count

  const summary = `
╔══════════════════════════════════════════════════════╗
║              THROUGHPUT TEST RESULTS                 ║
╠══════════════════════════════════════════════════════╣
║  Peak RPS:      ${rps} req/s                             ║
║  Total reqs:    ${total}                               ║
║  Avg latency:   ${avg}ms                              ║
║  p95 latency:   ${p95}ms                              ║
║  p99 latency:   ${p99}ms                              ║
╚══════════════════════════════════════════════════════╝

RESUME BULLET:
  Sustained ${rps} requests/sec (${Math.round(rps * 60)} req/min) at p95=${p95}ms
  under 100-VU concurrent load (k6 load test)
`
  console.log(summary)
  return {
    'results/throughput_result.txt': summary,
    stdout: '\n',
  }
}
