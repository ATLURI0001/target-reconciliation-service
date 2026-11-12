import xml.etree.ElementTree as ET
import os

assets_dir = r"C:\Users\atlur\.gemini\antigravity\scratch\target-reconciliation-service\docs\assets"
os.makedirs(assets_dir, exist_ok=True)

# 1. HERO BANNER (docs/assets/banner.svg)
banner_svg = '''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1200 420" width="100%" height="100%">
  <defs>
    <linearGradient id="bgGrad" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#0a0f1d"/>
      <stop offset="50%" stop-color="#0f172a"/>
      <stop offset="100%" stop-color="#050814"/>
    </linearGradient>
    <linearGradient id="glowGrad" x1="0%" y1="0%" x2="100%" y2="0%">
      <stop offset="0%" stop-color="#38bdf8" stop-opacity="0.8"/>
      <stop offset="50%" stop-color="#818cf8" stop-opacity="0.9"/>
      <stop offset="100%" stop-color="#c084fc" stop-opacity="0.8"/>
    </linearGradient>
    <linearGradient id="cardGrad" x1="0%" y1="0%" x2="0%" y2="100%">
      <stop offset="0%" stop-color="#1e293b" stop-opacity="0.85"/>
      <stop offset="100%" stop-color="#0f172a" stop-opacity="0.75"/>
    </linearGradient>
    <linearGradient id="badgeGrad" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#0ea5e9" stop-opacity="0.2"/>
      <stop offset="100%" stop-color="#6366f1" stop-opacity="0.2"/>
    </linearGradient>
    <filter id="dropShadow" x="-10%" y="-10%" width="120%" height="130%">
      <feDropShadow dx="0" dy="8" stdDeviation="12" flood-color="#000" flood-opacity="0.5"/>
    </filter>
    <filter id="neonGlow" x="-20%" y="-20%" width="140%" height="140%">
      <feGaussianBlur stdDeviation="8" result="coloredBlur"/>
      <feMerge>
        <feMergeNode in="coloredBlur"/>
        <feMergeNode in="SourceGraphic"/>
      </feMerge>
    </filter>
    <pattern id="gridPattern" width="40" height="40" patternUnits="userSpaceOnUse">
      <path d="M 40 0 L 0 0 0 40" fill="none" stroke="#334155" stroke-width="0.75" stroke-opacity="0.25"/>
    </pattern>
  </defs>

  <!-- Background -->
  <rect width="1200" height="420" rx="16" fill="url(#bgGrad)"/>
  <rect width="1200" height="420" rx="16" fill="url(#gridPattern)"/>

  <!-- Subtle glow rings -->
  <circle cx="1050" cy="80" r="220" fill="#38bdf8" fill-opacity="0.06" filter="blur(60px)"/>
  <circle cx="150" cy="350" r="260" fill="#818cf8" fill-opacity="0.06" filter="blur(70px)"/>

  <!-- Top Decorative Bar -->
  <rect x="0" y="0" width="1200" height="4" fill="url(#glowGrad)"/>

  <!-- Main Content Container -->
  <g transform="translate(60, 50)">
    <!-- Version Tag & Status -->
    <g transform="translate(0, 0)">
      <rect x="0" y="0" width="130" height="28" rx="14" fill="#0369a1" fill-opacity="0.3" stroke="#38bdf8" stroke-width="1"/>
      <circle cx="16" cy="14" r="4" fill="#38bdf8"/>
      <text x="28" y="19" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif" font-size="12" font-weight="600" fill="#7dd3fc" letter-spacing="0.5">v0.2.0-dev</text>

      <rect x="140" y="0" width="230" height="28" rx="14" fill="#1e293b" stroke="#475569" stroke-width="1"/>
      <text x="154" y="19" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif" font-size="12" font-weight="500" fill="#cbd5e1">Kubernetes Reliability Plane</text>
    </g>

    <!-- Project Title -->
    <text x="0" y="82" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif" font-size="44" font-weight="800" fill="#f8fafc" letter-spacing="-1">
      TargetReconciliationService
    </text>

    <!-- Subtitle / Mission -->
    <text x="0" y="122" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif" font-size="18" font-weight="400" fill="#94a3b8" letter-spacing="-0.2">
      A Kubernetes-native reliability control plane with policy-governed, auditable recovery.
    </text>

    <!-- Key Capability Cards -->
    <g transform="translate(0, 160)">
      <!-- Card 1 -->
      <g transform="translate(0, 0)">
        <rect width="250" height="120" rx="12" fill="url(#cardGrad)" stroke="#334155" stroke-width="1" filter="url(#dropShadow)"/>
        <circle cx="34" cy="34" r="16" fill="#0284c7" fill-opacity="0.2"/>
        <path d="M 28 34 L 33 39 L 41 29" fill="none" stroke="#38bdf8" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"/>
        <text x="60" y="39" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="15" font-weight="700" fill="#f1f5f9">Policy Governed</text>
        <text x="20" y="75" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="12" fill="#94a3b8">Dry-run by default, explicit</text>
        <text x="20" y="93" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="12" fill="#94a3b8">operator approvals &amp; fences</text>
      </g>

      <!-- Card 2 -->
      <g transform="translate(270, 0)">
        <rect width="250" height="120" rx="12" fill="url(#cardGrad)" stroke="#334155" stroke-width="1" filter="url(#dropShadow)"/>
        <circle cx="34" cy="34" r="16" fill="#6366f1" fill-opacity="0.2"/>
        <path d="M 26 34 L 42 34 M 34 26 L 34 42" fill="none" stroke="#818cf8" stroke-width="2.2" stroke-linecap="round"/>
        <circle cx="34" cy="34" r="7" fill="none" stroke="#818cf8" stroke-width="2"/>
        <text x="60" y="39" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="15" font-weight="700" fill="#f1f5f9">Atomic Rollback</text>
        <text x="20" y="75" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="12" fill="#94a3b8">Digest-pinned precondition</text>
        <text x="20" y="93" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="12" fill="#94a3b8">recovery with durable markers</text>
      </g>

      <!-- Card 3 -->
      <g transform="translate(540, 0)">
        <rect width="250" height="120" rx="12" fill="url(#cardGrad)" stroke="#334155" stroke-width="1" filter="url(#dropShadow)"/>
        <circle cx="34" cy="34" r="16" fill="#10b981" fill-opacity="0.2"/>
        <path d="M 27 27 L 41 27 L 41 41 L 27 41 Z M 31 32 L 37 32 M 31 36 L 35 36" fill="none" stroke="#34d399" stroke-width="2" stroke-linecap="round"/>
        <text x="60" y="39" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="15" font-weight="700" fill="#f1f5f9">Immutable Audit</text>
        <text x="20" y="75" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="12" fill="#94a3b8">PostgreSQL transactional log</text>
        <text x="20" y="93" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="12" fill="#94a3b8">with strict trigger-level locks</text>
      </g>

      <!-- Card 4 -->
      <g transform="translate(810, 0)">
        <rect width="250" height="120" rx="12" fill="url(#cardGrad)" stroke="#334155" stroke-width="1" filter="url(#dropShadow)"/>
        <circle cx="34" cy="34" r="16" fill="#f59e0b" fill-opacity="0.2"/>
        <path d="M 28 41 L 28 32 M 34 41 L 34 26 M 40 41 L 40 36" stroke="#fbbf24" stroke-width="2.5" stroke-linecap="round"/>
        <text x="60" y="39" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="15" font-weight="700" fill="#f1f5f9">Real-time Metrics</text>
        <text x="20" y="75" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="12" fill="#94a3b8">Prometheus alerts, Grafana</text>
        <text x="20" y="93" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="12" fill="#94a3b8">dashboards &amp; live telemetry</text>
      </g>
    </g>

    <!-- Bottom Technology Pill Bar -->
    <g transform="translate(0, 310)">
      <text x="0" y="16" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" font-weight="700" fill="#64748b" letter-spacing="1">BUILT WITH</text>
      
      <g transform="translate(80, 0)">
        <!-- Pill: Go -->
        <rect x="0" y="0" width="75" height="24" rx="6" fill="#1e293b" stroke="#334155" stroke-width="1"/>
        <text x="12" y="16" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" font-weight="600" fill="#38bdf8">Go 1.24</text>
        
        <!-- Pill: Kubernetes -->
        <rect x="85" y="0" width="100" height="24" rx="6" fill="#1e293b" stroke="#334155" stroke-width="1"/>
        <text x="96" y="16" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" font-weight="600" fill="#60a5fa">Kubernetes</text>
        
        <!-- Pill: PostgreSQL -->
        <rect x="195" y="0" width="95" height="24" rx="6" fill="#1e293b" stroke="#334155" stroke-width="1"/>
        <text x="206" y="16" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" font-weight="600" fill="#93c5fd">PostgreSQL</text>
        
        <!-- Pill: Next.js -->
        <rect x="300" y="0" width="80" height="24" rx="6" fill="#1e293b" stroke="#334155" stroke-width="1"/>
        <text x="312" y="16" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" font-weight="600" fill="#f8fafc">Next.js</text>

        <!-- Pill: Prometheus -->
        <rect x="390" y="0" width="105" height="24" rx="6" fill="#1e293b" stroke="#334155" stroke-width="1"/>
        <text x="402" y="16" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" font-weight="600" fill="#fb923c">Prometheus</text>

        <!-- Pill: Grafana -->
        <rect x="505" y="0" width="85" height="24" rx="6" fill="#1e293b" stroke="#334155" stroke-width="1"/>
        <text x="517" y="16" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" font-weight="600" fill="#f59e0b">Grafana</text>

        <!-- Pill: Docker & Helm -->
        <rect x="600" y="0" width="115" height="24" rx="6" fill="#1e293b" stroke="#334155" stroke-width="1"/>
        <text x="612" y="16" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" font-weight="600" fill="#2dd4bf">Docker &amp; Helm</text>
      </g>
    </g>
  </g>
</svg>'''

# 2. WORKFLOW LIFECYCLE (docs/assets/workflow-lifecycle.svg)
workflow_svg = '''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1100 520" width="100%" height="100%">
  <defs>
    <linearGradient id="wfBg" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#090d16"/>
      <stop offset="100%" stop-color="#0f172a"/>
    </linearGradient>
    <linearGradient id="card1Grad" x1="0%" y1="0%" x2="0%" y2="100%">
      <stop offset="0%" stop-color="#1e293b"/>
      <stop offset="100%" stop-color="#0f172a"/>
    </linearGradient>
    <filter id="wfGlow" x="-10%" y="-10%" width="120%" height="120%">
      <feDropShadow dx="0" dy="6" stdDeviation="10" flood-color="#000000" flood-opacity="0.6"/>
    </filter>
    <marker id="arrow" viewBox="0 0 10 10" refX="6" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse">
      <path d="M 0 1 L 8 5 L 0 9 z" fill="#64748b"/>
    </marker>
    <marker id="arrowSuccess" viewBox="0 0 10 10" refX="6" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse">
      <path d="M 0 1 L 8 5 L 0 9 z" fill="#10b981"/>
    </marker>
  </defs>

  <!-- Canvas Background -->
  <rect width="1100" height="520" rx="16" fill="url(#wfBg)"/>

  <!-- Title Block -->
  <g transform="translate(50, 40)">
    <text x="0" y="0" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif" font-size="12" font-weight="700" fill="#38bdf8" letter-spacing="1.5">RECONCILIATION ENGINE</text>
    <text x="0" y="28" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif" font-size="26" font-weight="700" fill="#f8fafc">Policy-Governed Incident Lifecycle &amp; Rollback Flow</text>
    <text x="0" y="52" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif" font-size="14" fill="#94a3b8">From sustained unreadiness detection through atomic digest rollback and transactional verification.</text>
  </g>

  <!-- 4 Step Flow -->
  <g transform="translate(50, 130)">
    <!-- Connecting flow line -->
    <path d="M 190 85 L 290 85" fill="none" stroke="#475569" stroke-width="2" stroke-dasharray="4,4" marker-end="url(#arrow)"/>
    <path d="M 470 85 L 570 85" fill="none" stroke="#475569" stroke-width="2" stroke-dasharray="4,4" marker-end="url(#arrow)"/>
    <path d="M 750 85 L 850 85" fill="none" stroke="#10b981" stroke-width="2" stroke-dasharray="4,4" marker-end="url(#arrowSuccess)"/>

    <!-- STEP 1: DETECT -->
    <g transform="translate(0, 0)">
      <rect width="210" height="230" rx="12" fill="url(#card1Grad)" stroke="#ef4444" stroke-width="1.5" stroke-opacity="0.8" filter="url(#wfGlow)"/>
      <rect x="0" y="0" width="210" height="38" rx="12" fill="#ef4444" fill-opacity="0.12"/>
      <circle cx="24" cy="19" r="6" fill="#ef4444"/>
      <text x="38" y="24" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="13" font-weight="700" fill="#fca5a5">1. Sustained Failure</text>

      <g transform="translate(16, 55)">
        <text x="0" y="15" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="12" font-weight="600" fill="#e2e8f0">Telemetry Telemetry</text>
        <text x="0" y="32" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" fill="#94a3b8">• Freshness window: 30s</text>
        <text x="0" y="48" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" fill="#94a3b8">• Rollout status unready</text>
        <text x="0" y="64" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" fill="#94a3b8">• Replica disparity check</text>

        <rect x="0" y="85" width="178" height="58" rx="6" fill="#18181b" stroke="#27272a" stroke-width="1"/>
        <text x="10" y="105" font-family="monospace" font-size="10" fill="#f87171">Status: Unready</text>
        <text x="10" y="122" font-family="monospace" font-size="10" fill="#cbd5e1">avail: 0 / desired: 3</text>
        <text x="10" y="136" font-family="monospace" font-size="9" fill="#71717a">dedup key: ep-9840</text>
      </g>
    </g>

    <!-- STEP 2: PLAN & AUTHORIZE -->
    <g transform="translate(280, 0)">
      <rect width="210" height="230" rx="12" fill="url(#card1Grad)" stroke="#f59e0b" stroke-width="1.5" stroke-opacity="0.8" filter="url(#wfGlow)"/>
      <rect x="0" y="0" width="210" height="38" rx="12" fill="#f59e0b" fill-opacity="0.12"/>
      <circle cx="24" cy="19" r="6" fill="#f59e0b"/>
      <text x="38" y="24" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="13" font-weight="700" fill="#fde68a">2. Policy Gate</text>

      <g transform="translate(16, 55)">
        <text x="0" y="15" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="12" font-weight="600" fill="#e2e8f0">Authorization Check</text>
        <text x="0" y="32" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" fill="#94a3b8">• Dry-run default mode</text>
        <text x="0" y="48" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" fill="#94a3b8">• GitOps ownership fence</text>
        <text x="0" y="64" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" fill="#94a3b8">• Explicit Operator approval</text>

        <rect x="0" y="85" width="178" height="58" rx="6" fill="#18181b" stroke="#27272a" stroke-width="1"/>
        <text x="10" y="105" font-family="monospace" font-size="10" fill="#fbbf24">AwaitingApproval</text>
        <text x="10" y="122" font-family="monospace" font-size="10" fill="#cbd5e1">role: trs_operator</text>
        <text x="10" y="136" font-family="monospace" font-size="9" fill="#71717a">concurrency: 1/1</text>
      </g>
    </g>

    <!-- STEP 3: ATOMIC ROLLBACK -->
    <g transform="translate(560, 0)">
      <rect width="210" height="230" rx="12" fill="url(#card1Grad)" stroke="#38bdf8" stroke-width="1.5" stroke-opacity="0.8" filter="url(#wfGlow)"/>
      <rect x="0" y="0" width="210" height="38" rx="12" fill="#38bdf8" fill-opacity="0.12"/>
      <circle cx="24" cy="19" r="6" fill="#38bdf8"/>
      <text x="38" y="24" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="13" font-weight="700" fill="#bae6fd">3. Atomic Dispatch</text>

      <g transform="translate(16, 55)">
        <text x="0" y="15" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="12" font-weight="600" fill="#e2e8f0">Precondition Patch</text>
        <text x="0" y="32" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" fill="#94a3b8">• UID + resourceVersion check</text>
        <text x="0" y="48" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" fill="#94a3b8">• sha256 digest pinned</text>
        <text x="0" y="64" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" fill="#94a3b8">• Pod execution annotation</text>

        <rect x="0" y="85" width="178" height="58" rx="6" fill="#18181b" stroke="#27272a" stroke-width="1"/>
        <text x="10" y="105" font-family="monospace" font-size="10" fill="#38bdf8">State: Executing</text>
        <text x="10" y="122" font-family="monospace" font-size="10" fill="#cbd5e1">patch: image@sha256</text>
        <text x="10" y="136" font-family="monospace" font-size="9" fill="#71717a">deadline: 120s limit</text>
      </g>
    </g>

    <!-- STEP 4: VERIFY & AUDIT -->
    <g transform="translate(840, 0)">
      <rect width="210" height="230" rx="12" fill="url(#card1Grad)" stroke="#10b981" stroke-width="1.5" stroke-opacity="0.8" filter="url(#wfGlow)"/>
      <rect x="0" y="0" width="210" height="38" rx="12" fill="#10b981" fill-opacity="0.12"/>
      <circle cx="24" cy="19" r="6" fill="#10b981"/>
      <text x="38" y="24" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="13" font-weight="700" fill="#a7f3d0">4. Audit &amp; Verify</text>

      <g transform="translate(16, 55)">
        <text x="0" y="15" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="12" font-weight="600" fill="#e2e8f0">Closed-Loop Seal</text>
        <text x="0" y="32" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" fill="#94a3b8">• observedGeneration ready</text>
        <text x="0" y="48" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" fill="#94a3b8">• No surplus old pods</text>
        <text x="0" y="64" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" fill="#94a3b8">• Immutable PG rp_audit log</text>

        <rect x="0" y="85" width="178" height="58" rx="6" fill="#18181b" stroke="#27272a" stroke-width="1"/>
        <text x="10" y="105" font-family="monospace" font-size="10" fill="#34d399">State: Resolved</text>
        <text x="10" y="122" font-family="monospace" font-size="10" fill="#cbd5e1">replicas: 3/3 ready</text>
        <text x="10" y="136" font-family="monospace" font-size="9" fill="#71717a">audit sealed: seq 42</text>
      </g>
    </g>
  </g>

  <!-- Bottom Details Bar -->
  <g transform="translate(50, 390)">
    <rect width="1000" height="90" rx="10" fill="#111827" stroke="#1f2937" stroke-width="1"/>
    
    <g transform="translate(24, 20)">
      <circle cx="10" cy="10" r="10" fill="#0284c7" fill-opacity="0.2"/>
      <text x="8" y="14" font-family="monospace" font-size="11" font-weight="700" fill="#38bdf8">i</text>
      <text x="30" y="14" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="13" font-weight="700" fill="#f8fafc">Key Safety Guarantees</text>
      
      <text x="30" y="38" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="12" fill="#94a3b8">
        • <tspan fill="#e2e8f0" font-weight="600">Deterministic Recovery Image:</tspan> Pinned by sha256 digest from ReliabilityTarget spec, never guessed.
      </text>
      <text x="30" y="56" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="12" fill="#94a3b8">
        • <tspan fill="#e2e8f0" font-weight="600">Concurrency &amp; Ownership Fences:</tspan> GitOps managed targets are rejected; maximum 1 simultaneous recovery per target.
      </text>
    </g>

    <g transform="translate(700, 26)">
      <rect width="270" height="42" rx="8" fill="#1e293b" stroke="#334155" stroke-width="1"/>
      <text x="16" y="26" font-family="monospace" font-size="11" fill="#7dd3fc">CRD: ReliabilityTarget/v1alpha1</text>
    </g>
  </g>
</svg>'''

# 3. METRICS DASHBOARD (docs/assets/metrics-dashboard.svg)
metrics_svg = '''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1100 600" width="100%" height="100%">
  <defs>
    <linearGradient id="dashBg" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#0b0f19"/>
      <stop offset="100%" stop-color="#111827"/>
    </linearGradient>
    <linearGradient id="panelGrad" x1="0%" y1="0%" x2="0%" y2="100%">
      <stop offset="0%" stop-color="#182234"/>
      <stop offset="100%" stop-color="#0f172a"/>
    </linearGradient>
    <linearGradient id="greenArea" x1="0%" y1="0%" x2="0%" y2="100%">
      <stop offset="0%" stop-color="#10b981" stop-opacity="0.35"/>
      <stop offset="100%" stop-color="#10b981" stop-opacity="0.0"/>
    </linearGradient>
    <linearGradient id="redArea" x1="0%" y1="0%" x2="0%" y2="100%">
      <stop offset="0%" stop-color="#ef4444" stop-opacity="0.3"/>
      <stop offset="100%" stop-color="#ef4444" stop-opacity="0.0"/>
    </linearGradient>
    <linearGradient id="blueArea" x1="0%" y1="0%" x2="0%" y2="100%">
      <stop offset="0%" stop-color="#38bdf8" stop-opacity="0.4"/>
      <stop offset="100%" stop-color="#38bdf8" stop-opacity="0.0"/>
    </linearGradient>
    <filter id="panelShadow" x="-5%" y="-5%" width="110%" height="115%">
      <feDropShadow dx="0" dy="4" stdDeviation="6" flood-color="#000000" flood-opacity="0.4"/>
    </filter>
  </defs>

  <!-- Background -->
  <rect width="1100" height="600" rx="16" fill="url(#dashBg)"/>

  <!-- Grafana-style Header -->
  <g transform="translate(30, 24)">
    <!-- Dashboard Title -->
    <rect x="0" y="0" width="32" height="32" rx="6" fill="#f59e0b" fill-opacity="0.15"/>
    <path d="M 6 22 L 13 14 L 19 18 L 26 10" fill="none" stroke="#f59e0b" stroke-width="2.5" stroke-linecap="round"/>
    <text x="44" y="22" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif" font-size="18" font-weight="700" fill="#f8fafc">TargetReconciliationService · Core Reliability &amp; Observability</text>
    
    <!-- Time range selector -->
    <g transform="translate(860, 2)">
      <rect width="180" height="28" rx="6" fill="#1e293b" stroke="#334155" stroke-width="1"/>
      <circle cx="16" cy="14" r="4" fill="#10b981"/>
      <text x="28" y="19" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" font-weight="600" fill="#cbd5e1">Last 30m • Refresh 5s</text>
    </g>
  </g>

  <!-- Stat Banner Row -->
  <g transform="translate(30, 75)">
    <!-- Metric 1: System Status -->
    <g transform="translate(0, 0)">
      <rect width="245" height="70" rx="8" fill="url(#panelGrad)" stroke="#334155" stroke-width="1" filter="url(#panelShadow)"/>
      <text x="16" y="24" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" font-weight="600" fill="#94a3b8" letter-spacing="0.5">CONTROL PLANE STATUS</text>
      <text x="16" y="52" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="20" font-weight="700" fill="#34d399">HEALTHY</text>
      <circle cx="115" cy="46" r="5" fill="#10b981"/>
      <text x="130" y="51" font-family="monospace" font-size="11" fill="#64748b">up=1</text>
    </g>

    <!-- Metric 2: Telemetry Freshness -->
    <g transform="translate(265, 0)">
      <rect width="245" height="70" rx="8" fill="url(#panelGrad)" stroke="#334155" stroke-width="1" filter="url(#panelShadow)"/>
      <text x="16" y="24" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" font-weight="600" fill="#94a3b8" letter-spacing="0.5">TELEMETRY FRESHNESS</text>
      <text x="16" y="52" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="20" font-weight="700" fill="#38bdf8">1.8s</text>
      <text x="68" y="51" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" fill="#64748b">threshold &lt; 30s</text>
    </g>

    <!-- Metric 3: Active Reliability Targets -->
    <g transform="translate(530, 0)">
      <rect width="245" height="70" rx="8" fill="url(#panelGrad)" stroke="#334155" stroke-width="1" filter="url(#panelShadow)"/>
      <text x="16" y="24" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" font-weight="600" fill="#94a3b8" letter-spacing="0.5">MANAGED TARGETS</text>
      <text x="16" y="52" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="20" font-weight="700" fill="#818cf8">8 Registered</text>
      <text x="150" y="51" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" fill="#64748b">100% Opt-In</text>
    </g>

    <!-- Metric 4: Recovery Success -->
    <g transform="translate(795, 0)">
      <rect width="245" height="70" rx="8" fill="url(#panelGrad)" stroke="#334155" stroke-width="1" filter="url(#panelShadow)"/>
      <text x="16" y="24" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" font-weight="600" fill="#94a3b8" letter-spacing="0.5">ROLLBACK VERIFICATION</text>
      <text x="16" y="52" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="20" font-weight="700" fill="#f59e0b">100%</text>
      <text x="80" y="51" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" fill="#64748b">0 unverified</text>
    </g>
  </g>

  <!-- Main Chart Row -->
  <g transform="translate(30, 165)">
    <!-- Chart 1: Deployment Replicas Convergence (Available vs Desired) -->
    <g transform="translate(0, 0)">
      <rect width="510" height="210" rx="8" fill="url(#panelGrad)" stroke="#334155" stroke-width="1" filter="url(#panelShadow)"/>
      <text x="16" y="25" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="13" font-weight="600" fill="#f1f5f9">Deployment Replicas (Available vs Desired)</text>
      <text x="16" y="42" font-family="monospace" font-size="10" fill="#64748b">trs_deployment_available_replicas vs trs_deployment_desired_replicas</text>

      <!-- Chart axes & grid -->
      <g transform="translate(45, 60)">
        <line x1="0" y1="0" x2="440" y2="0" stroke="#1e293b" stroke-width="1"/>
        <line x1="0" y1="35" x2="440" y2="35" stroke="#1e293b" stroke-width="1"/>
        <line x1="0" y1="70" x2="440" y2="70" stroke="#1e293b" stroke-width="1"/>
        <line x1="0" y1="105" x2="440" y2="105" stroke="#334155" stroke-width="1"/>

        <!-- Y labels -->
        <text x="-25" y="5" font-family="monospace" font-size="10" fill="#64748b">3.0</text>
        <text x="-25" y="40" font-family="monospace" font-size="10" fill="#64748b">2.0</text>
        <text x="-25" y="75" font-family="monospace" font-size="10" fill="#64748b">1.0</text>
        <text x="-25" y="109" font-family="monospace" font-size="10" fill="#64748b">0.0</text>

        <!-- Desired line (constant 3) -->
        <line x1="0" y1="0" x2="440" y2="0" stroke="#94a3b8" stroke-width="1.5" stroke-dasharray="4,4"/>

        <!-- Failure & Rollback curve -->
        <!-- Starts healthy at 3 -> drops to 0 at failure -> recovery executed -> rises back to 3 -->
        <path d="M 0 0 L 100 0 L 120 105 L 260 105 L 300 0 L 440 0" fill="none" stroke="#10b981" stroke-width="2.5"/>
        <path d="M 120 105 L 260 105" fill="none" stroke="#ef4444" stroke-width="2.5"/>

        <!-- Markers and annotations -->
        <circle cx="120" cy="105" r="4" fill="#ef4444"/>
        <text x="100" y="125" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="9" font-weight="600" fill="#f87171">Failed Rollout</text>

        <circle cx="260" cy="105" r="4" fill="#38bdf8"/>
        <text x="225" y="90" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="9" font-weight="600" fill="#38bdf8">Atomic Rollback</text>

        <circle cx="300" cy="0" r="4" fill="#10b981"/>
        <text x="310" y="20" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="9" font-weight="600" fill="#34d399">Verified 3/3</text>
      </g>
    </g>

    <!-- Chart 2: Telemetry Heartbeat & HTTP Metrics -->
    <g transform="translate(530, 0)">
      <rect width="510" height="210" rx="8" fill="url(#panelGrad)" stroke="#334155" stroke-width="1" filter="url(#panelShadow)"/>
      <text x="16" y="25" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="13" font-weight="600" fill="#f1f5f9">Cluster Telemetry Heartbeat Age</text>
      <text x="16" y="42" font-family="monospace" font-size="10" fill="#64748b">time() - trs_cluster_last_seen_seconds (alert threshold: 30s)</text>

      <g transform="translate(45, 60)">
        <line x1="0" y1="15" x2="440" y2="15" stroke="#ef4444" stroke-width="1.5" stroke-dasharray="3,3"/>
        <text x="340" y="10" font-family="monospace" font-size="9" fill="#f87171">30s Stale Alert Limit</text>

        <line x1="0" y1="50" x2="440" y2="50" stroke="#1e293b" stroke-width="1"/>
        <line x1="0" y1="80" x2="440" y2="80" stroke="#1e293b" stroke-width="1"/>
        <line x1="0" y1="105" x2="440" y2="105" stroke="#334155" stroke-width="1"/>

        <!-- Y labels -->
        <text x="-25" y="19" font-family="monospace" font-size="10" fill="#64748b">30s</text>
        <text x="-25" y="55" font-family="monospace" font-size="10" fill="#64748b">15s</text>
        <text x="-25" y="109" font-family="monospace" font-size="10" fill="#64748b">0s</text>

        <!-- Fresh saw-tooth pattern (reconciliation loops every ~3s) -->
        <path d="M 0 102 L 30 92 L 31 104 L 70 90 L 71 104 L 110 93 L 111 104 L 150 91 L 151 104 L 190 92 L 191 104 L 230 90 L 231 104 L 270 92 L 271 104 L 310 91 L 311 104 L 350 92 L 351 104 L 390 90 L 391 104 L 430 92" fill="none" stroke="#38bdf8" stroke-width="2"/>
        <text x="180" y="125" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="10" fill="#38bdf8">Continuous sub-second healthy heartbeats</text>
      </g>
    </g>
  </g>

  <!-- Live Audit Stream Panel (Bottom) -->
  <g transform="translate(30, 395)">
    <rect width="1040" height="180" rx="8" fill="url(#panelGrad)" stroke="#334155" stroke-width="1" filter="url(#panelShadow)"/>
    <text x="16" y="25" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="13" font-weight="600" fill="#f1f5f9">Immutable PostgreSQL Audit Timeline (rp_audit)</text>
    <text x="16" y="42" font-family="monospace" font-size="10" fill="#64748b">SELECT sequence_id, timestamp, target, action, principal, state FROM rp_audit ORDER BY sequence_id DESC</text>

    <!-- Table Header -->
    <g transform="translate(16, 60)">
      <rect width="1008" height="26" fill="#1e293b" rx="4"/>
      <text x="15" y="17" font-family="monospace" font-size="10" font-weight="700" fill="#94a3b8">SEQ</text>
      <text x="70" y="17" font-family="monospace" font-size="10" font-weight="700" fill="#94a3b8">TIMESTAMP</text>
      <text x="210" y="17" font-family="monospace" font-size="10" font-weight="700" fill="#94a3b8">INCIDENT</text>
      <text x="320" y="17" font-family="monospace" font-size="10" font-weight="700" fill="#94a3b8">TARGET WORKLOAD</text>
      <text x="500" y="17" font-family="monospace" font-size="10" font-weight="700" fill="#94a3b8">ACTION DISPATCHED</text>
      <text x="730" y="17" font-family="monospace" font-size="10" font-weight="700" fill="#94a3b8">PRINCIPAL</text>
      <text x="890" y="17" font-family="monospace" font-size="10" font-weight="700" fill="#94a3b8">STATE</text>
    </g>

    <!-- Table Rows -->
    <g transform="translate(16, 92)">
      <!-- Row 1 -->
      <g transform="translate(0, 0)">
        <text x="15" y="14" font-family="monospace" font-size="10" fill="#64748b">#042</text>
        <text x="70" y="14" font-family="monospace" font-size="10" fill="#cbd5e1">12:38:12 UTC</text>
        <text x="210" y="14" font-family="monospace" font-size="10" fill="#38bdf8">inc-8902-cat</text>
        <text x="320" y="14" font-family="monospace" font-size="10" fill="#f8fafc">catalog-deployment</text>
        <text x="500" y="14" font-family="monospace" font-size="10" fill="#34d399">VERIFICATION_CONFIRMED (3/3)</text>
        <text x="730" y="14" font-family="monospace" font-size="10" fill="#94a3b8">k8s/reconciler</text>
        <rect x="888" y="2" width="70" height="18" rx="4" fill="#065f46"/>
        <text x="902" y="15" font-family="monospace" font-size="9" font-weight="700" fill="#6ee7b7">Resolved</text>
      </g>

      <!-- Row 2 -->
      <g transform="translate(0, 24)">
        <text x="15" y="14" font-family="monospace" font-size="10" fill="#64748b">#041</text>
        <text x="70" y="14" font-family="monospace" font-size="10" fill="#cbd5e1">12:37:45 UTC</text>
        <text x="210" y="14" font-family="monospace" font-size="10" fill="#38bdf8">inc-8902-cat</text>
        <text x="320" y="14" font-family="monospace" font-size="10" fill="#f8fafc">catalog-deployment</text>
        <text x="500" y="14" font-family="monospace" font-size="10" fill="#60a5fa">DIGEST_ROLLBACK_APPLIED</text>
        <text x="730" y="14" font-family="monospace" font-size="10" fill="#94a3b8">k8s/reconciler</text>
        <rect x="888" y="2" width="70" height="18" rx="4" fill="#1e3a8a"/>
        <text x="900" y="15" font-family="monospace" font-size="9" font-weight="700" fill="#93c5fd">Executing</text>
      </g>

      <!-- Row 3 -->
      <g transform="translate(0, 48)">
        <text x="15" y="14" font-family="monospace" font-size="10" fill="#64748b">#040</text>
        <text x="70" y="14" font-family="monospace" font-size="10" fill="#cbd5e1">12:37:10 UTC</text>
        <text x="210" y="14" font-family="monospace" font-size="10" fill="#38bdf8">inc-8902-cat</text>
        <text x="320" y="14" font-family="monospace" font-size="10" fill="#f8fafc">catalog-deployment</text>
        <text x="500" y="14" font-family="monospace" font-size="10" fill="#fbbf24">PLAN_APPROVED (dryRun=false)</text>
        <text x="730" y="14" font-family="monospace" font-size="10" fill="#94a3b8">operator/alex</text>
        <rect x="888" y="2" width="70" height="18" rx="4" fill="#78350f"/>
        <text x="902" y="15" font-family="monospace" font-size="9" font-weight="700" fill="#fde68a">Approved</text>
      </g>
    </g>
  </g>
</svg>'''

# 4. CONTROL PLANE ARCHITECTURE (docs/assets/architecture.svg)
arch_svg = '''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1100 540" width="100%" height="100%">
  <defs>
    <linearGradient id="archBg" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#0a0f1d"/>
      <stop offset="100%" stop-color="#0f172a"/>
    </linearGradient>
    <linearGradient id="boxGrad" x1="0%" y1="0%" x2="0%" y2="100%">
      <stop offset="0%" stop-color="#1e293b"/>
      <stop offset="100%" stop-color="#0f172a"/>
    </linearGradient>
    <filter id="boxShadow" x="-10%" y="-10%" width="120%" height="120%">
      <feDropShadow dx="0" dy="6" stdDeviation="8" flood-color="#000000" flood-opacity="0.5"/>
    </filter>
    <marker id="mArrow" viewBox="0 0 10 10" refX="6" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse">
      <path d="M 0 1 L 8 5 L 0 9 z" fill="#38bdf8"/>
    </marker>
    <marker id="mArrowGray" viewBox="0 0 10 10" refX="6" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse">
      <path d="M 0 1 L 8 5 L 0 9 z" fill="#64748b"/>
    </marker>
  </defs>

  <!-- Canvas Background -->
  <rect width="1100" height="540" rx="16" fill="url(#archBg)"/>

  <!-- Header -->
  <g transform="translate(50, 40)">
    <text x="0" y="0" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif" font-size="12" font-weight="700" fill="#38bdf8" letter-spacing="1.5">CONTROL PLANE ARCHITECTURE</text>
    <text x="0" y="28" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif" font-size="26" font-weight="700" fill="#f8fafc">Component Boundaries &amp; Dataflow</text>
    <text x="0" y="52" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif" font-size="14" fill="#94a3b8">Transactional isolation between Kubernetes runtime controllers and policy state.</text>
  </g>

  <!-- Connectors -->
  <g transform="translate(50, 140)">
    <!-- UI to API -->
    <path d="M 125 110 L 260 110" fill="none" stroke="#38bdf8" stroke-width="2" marker-end="url(#mArrow)"/>
    
    <!-- API to Workflow -->
    <path d="M 440 110 L 530 110" fill="none" stroke="#38bdf8" stroke-width="2" marker-end="url(#mArrow)"/>
    
    <!-- Workflow to Postgres -->
    <path d="M 640 180 L 640 250" fill="none" stroke="#34d399" stroke-width="2" marker-end="url(#mArrow)"/>

    <!-- Reconciler to Workflow -->
    <path d="M 750 110 L 840 110" fill="none" stroke="#818cf8" stroke-width="2" marker-end="url(#mArrow)"/>

    <!-- Reconciler to K8s API -->
    <path d="M 940 180 L 940 250" fill="none" stroke="#60a5fa" stroke-width="2" marker-end="url(#mArrow)"/>
  </g>

  <!-- Component Boxes -->
  <g transform="translate(50, 140)">
    <!-- BOX 1: CLIENT LAYER -->
    <g transform="translate(0, 30)">
      <rect width="180" height="150" rx="10" fill="url(#boxGrad)" stroke="#38bdf8" stroke-width="1.5" filter="url(#boxShadow)"/>
      <rect x="0" y="0" width="180" height="32" rx="10" fill="#0284c7" fill-opacity="0.15"/>
      <text x="16" y="21" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="12" font-weight="700" fill="#7dd3fc">OPERATOR CLIENTS</text>
      
      <rect x="14" y="46" width="152" height="36" rx="6" fill="#111827" stroke="#334155" stroke-width="1"/>
      <text x="24" y="68" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="12" font-weight="600" fill="#f8fafc">Next.js Web Console</text>

      <rect x="14" y="94" width="152" height="36" rx="6" fill="#111827" stroke="#334155" stroke-width="1"/>
      <text x="24" y="116" font-family="monospace" font-size="12" font-weight="600" fill="#38bdf8">trsctl CLI tool</text>
    </g>

    <!-- BOX 2: MANAGEMENT API -->
    <g transform="translate(260, 20)">
      <rect width="180" height="170" rx="10" fill="url(#boxGrad)" stroke="#475569" stroke-width="1.5" filter="url(#boxShadow)"/>
      <rect x="0" y="0" width="180" height="32" rx="10" fill="#334155" fill-opacity="0.25"/>
      <text x="16" y="21" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="12" font-weight="700" fill="#f1f5f9">MANAGEMENT API</text>
      
      <text x="16" y="55" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" fill="#94a3b8">• Go HTTP Server (:8080)</text>
      <text x="16" y="73" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" fill="#94a3b8">• Token Auth (Viewer/Op)</text>
      <text x="16" y="91" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" fill="#94a3b8">• Rate Limiter (20 req/s)</text>
      <text x="16" y="109" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" fill="#94a3b8">• Max 32 concurrency</text>
      <text x="16" y="127" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" fill="#94a3b8">• Prometheus Metrics</text>

      <rect x="14" y="137" width="152" height="20" rx="4" fill="#0369a1" fill-opacity="0.2"/>
      <text x="22" y="151" font-family="monospace" font-size="9" fill="#38bdf8">GET /v1/snapshot</text>
    </g>

    <!-- BOX 3: WORKFLOW ENGINE -->
    <g transform="translate(530, 20)">
      <rect width="220" height="170" rx="10" fill="url(#boxGrad)" stroke="#818cf8" stroke-width="1.5" filter="url(#boxShadow)"/>
      <rect x="0" y="0" width="220" height="32" rx="10" fill="#6366f1" fill-opacity="0.15"/>
      <text x="16" y="21" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="12" font-weight="700" fill="#c7d2fe">CORE WORKFLOW SERVICE</text>
      
      <text x="16" y="55" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" fill="#94a3b8">• Deterministic Planner</text>
      <text x="16" y="73" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" fill="#94a3b8">• DryRun &amp; Approval Engine</text>
      <text x="16" y="91" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" fill="#94a3b8">• GitOps Fence Enforcement</text>
      <text x="16" y="109" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" fill="#94a3b8">• Concurrent Admission Guard</text>
      <text x="16" y="127" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" fill="#94a3b8">• Verification Evaluator</text>
    </g>

    <!-- BOX 4: KUBERNETES RECONCILER -->
    <g transform="translate(840, 20)">
      <rect width="210" height="170" rx="10" fill="url(#boxGrad)" stroke="#60a5fa" stroke-width="1.5" filter="url(#boxShadow)"/>
      <rect x="0" y="0" width="210" height="32" rx="10" fill="#2563eb" fill-opacity="0.15"/>
      <text x="16" y="21" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="12" font-weight="700" fill="#bfdbfe">K8s RECONCILER</text>
      
      <text x="16" y="55" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" fill="#94a3b8">• controller-runtime</text>
      <text x="16" y="73" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" fill="#94a3b8">• Leader Election enabled</text>
      <text x="16" y="91" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" fill="#94a3b8">• Uncached Deployment reads</text>
      <text x="16" y="109" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" fill="#94a3b8">• CRD status subresource</text>
      <text x="16" y="127" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" fill="#94a3b8">• Atomic patch with UID</text>
    </g>

    <!-- BOTTOM ROW: STORAGE & KUBERNETES -->
    <!-- Postgres State -->
    <g transform="translate(530, 240)">
      <rect width="220" height="110" rx="10" fill="url(#boxGrad)" stroke="#10b981" stroke-width="1.5" filter="url(#boxShadow)"/>
      <rect x="0" y="0" width="220" height="28" rx="10" fill="#059669" fill-opacity="0.15"/>
      <text x="16" y="19" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" font-weight="700" fill="#6ee7b7">POSTGRESQL PERSISTENCE</text>
      
      <text x="16" y="48" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" fill="#cbd5e1">• State JSONB aggregate</text>
      <text x="16" y="66" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" fill="#cbd5e1">• Append-only rp_audit table</text>
      <text x="16" y="84" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" fill="#cbd5e1">• SQL Triggers block updates</text>
    </g>

    <!-- Kubernetes Target Cluster -->
    <g transform="translate(840, 240)">
      <rect width="210" height="110" rx="10" fill="url(#boxGrad)" stroke="#3b82f6" stroke-width="1.5" filter="url(#boxShadow)"/>
      <rect x="0" y="0" width="210" height="28" rx="10" fill="#1d4ed8" fill-opacity="0.15"/>
      <text x="16" y="19" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" font-weight="700" fill="#93c5fd">KUBERNETES TARGETS</text>
      
      <text x="16" y="48" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" fill="#cbd5e1">• CRD: ReliabilityTarget</text>
      <text x="16" y="66" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" fill="#cbd5e1">• Target Deployments</text>
      <text x="16" y="84" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif" font-size="11" fill="#cbd5e1">• Scoped RBAC ServiceAccount</text>
    </g>
  </g>
</svg>'''

files = {
    "banner.svg": banner_svg,
    "workflow-lifecycle.svg": workflow_svg,
    "metrics-dashboard.svg": metrics_svg,
    "architecture.svg": arch_svg
}

for name, content in files.items():
    filepath = os.path.join(assets_dir, name)
    # Validate XML
    ET.fromstring(content)
    with open(filepath, "w", encoding="utf-8") as f:
        f.write(content)
    print(f"Successfully generated and validated {name} ({len(content)} bytes)")
