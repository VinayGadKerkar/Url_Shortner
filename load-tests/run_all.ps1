# run_all.ps1 — runs all k6 load tests and prints resume-ready bullets
# Usage: cd load-tests ; .\run_all.ps1

$ErrorActionPreference = "Continue"
$resultsDir = "results"
New-Item -ItemType Directory -Force -Path $resultsDir | Out-Null

# ── Step 1: Create a test URL to use in redirect tests ────────────────────────
Write-Host "`n[setup] Creating test short URL..." -ForegroundColor Cyan
$body    = '{"long_url":"https://example.com/load-test-target"}'
$headers = @{ "Content-Type" = "application/json" }
try {
    $resp = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/shorten" `
                              -Method POST -Body $body -Headers $headers
    $shortCode = $resp.short_code
    Write-Host "[setup] Short code: $shortCode" -ForegroundColor Green
} catch {
    Write-Host "[setup] Failed to create URL. Is the backend running? (docker compose up -d)" -ForegroundColor Red
    exit 1
}

# ── Step 2: Flush Redis so Test 1 starts truly cold ───────────────────────────
Write-Host "`n[setup] Flushing Redis cache for cold-start test..." -ForegroundColor Cyan
docker exec redis redis-cli FLUSHALL | Out-Null
Write-Host "[setup] Redis flushed." -ForegroundColor Green

# ── Step 3: Run tests ─────────────────────────────────────────────────────────
$tests = @(
    @{ file = "01_cache_latency.js"; name = "Cache Latency Comparison" },
    @{ file = "02_throughput.js";    name = "Peak Throughput"          },
    @{ file = "03_rate_limiter.js";  name = "Rate Limiter Validation"  },
    @{ file = "04_create_url.js";    name = "URL Creation Throughput"  }
)

foreach ($t in $tests) {
    Write-Host "`n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor DarkGray
    Write-Host "  Running: $($t.name)" -ForegroundColor Yellow
    Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor DarkGray

    $env:SHORT_CODE = $shortCode
    k6 run --env SHORT_CODE=$shortCode $t.file
}

# ── Step 4: Print all resume bullets ─────────────────────────────────────────
Write-Host "`n`n" -NoNewline
Write-Host "╔══════════════════════════════════════════════════════════════╗" -ForegroundColor Magenta
Write-Host "║              ALL RESULTS — RESUME BULLETS                   ║" -ForegroundColor Magenta
Write-Host "╚══════════════════════════════════════════════════════════════╝" -ForegroundColor Magenta

$resultFiles = Get-ChildItem -Path $resultsDir -Filter "*.txt" -ErrorAction SilentlyContinue
if ($resultFiles) {
    foreach ($f in $resultFiles) {
        Write-Host "`n── $($f.Name) ──" -ForegroundColor Cyan
        Get-Content $f.FullName | Select-String "RESUME BULLET" -Context 0,3 | ForEach-Object {
            Write-Host $_.Line -ForegroundColor White
            $_.Context.PostContext | ForEach-Object { Write-Host "  $_" -ForegroundColor Green }
        }
    }
} else {
    Write-Host "`nResults will appear in the 'results/' folder after tests complete." -ForegroundColor Gray
}
