let rpsChart = null;
let latencyChart = null;
let eventSource = null;
let currentSnapshot = null;
const maxDataPoints = 30;

const chartTimeLabels = [];
const rpsCurrentData = [];
const rpsAvgData = [];

const latP50Data = [];
const latP95Data = [];
const latP99Data = [];
const latAvgData = [];

function initCharts() {
  const rpsCtx = document.getElementById('rpsChart').getContext('2d');
  rpsChart = new Chart(rpsCtx, {
    type: 'line',
    data: {
      labels: chartTimeLabels,
      datasets: [
        {
          label: 'Current RPS',
          data: rpsCurrentData,
          borderColor: '#38bdf8',
          backgroundColor: 'rgba(56, 189, 248, 0.1)',
          fill: true,
          tension: 0.3,
          borderWidth: 2
        },
        {
          label: 'Average RPS',
          data: rpsAvgData,
          borderColor: '#818cf8',
          borderDash: [5, 5],
          tension: 0.1,
          borderWidth: 1.5
        }
      ]
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      animation: false,
      scales: {
        x: { grid: { color: '#1e293b' }, ticks: { color: '#94a3b8' } },
        y: { beginAtZero: true, grid: { color: '#1e293b' }, ticks: { color: '#94a3b8' } }
      },
      plugins: {
        legend: { labels: { color: '#cbd5e1' } }
      }
    }
  });

  const latCtx = document.getElementById('latencyChart').getContext('2d');
  latencyChart = new Chart(latCtx, {
    type: 'line',
    data: {
      labels: chartTimeLabels,
      datasets: [
        {
          label: 'p50',
          data: latP50Data,
          borderColor: '#34d399',
          tension: 0.2,
          borderWidth: 2
        },
        {
          label: 'p95',
          data: latP95Data,
          borderColor: '#fbbf24',
          tension: 0.2,
          borderWidth: 2
        },
        {
          label: 'p99',
          data: latP99Data,
          borderColor: '#f87171',
          tension: 0.2,
          borderWidth: 2
        },
        {
          label: 'Avg',
          data: latAvgData,
          borderColor: '#a78bfa',
          borderDash: [3, 3],
          tension: 0.2,
          borderWidth: 1.5
        }
      ]
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      animation: false,
      scales: {
        x: { grid: { color: '#1e293b' }, ticks: { color: '#94a3b8' } },
        y: { 
          beginAtZero: true, 
          grid: { color: '#1e293b' }, 
          ticks: { 
            color: '#94a3b8',
            callback: (v) => v + ' ms'
          } 
        }
      },
      plugins: {
        legend: { labels: { color: '#cbd5e1' } }
      }
    }
  });
}

function formatDurationNs(ns) {
  const ms = ns / 1000000;
  if (ms < 1) {
    return (ns / 1000).toFixed(1) + ' µs';
  } else if (ms < 1000) {
    return ms.toFixed(2) + ' ms';
  } else {
    return (ms / 1000).toFixed(2) + ' s';
  }
}

function formatTimeOnly(date) {
  return date.toTimeString().split(' ')[0];
}

function updateUI(snap) {
  currentSnapshot = snap;

  document.getElementById('target-url-text').textContent = snap.target_url || '-';
  document.getElementById('stat-total-req').textContent = (snap.total_requests || 0).toLocaleString();
  document.getElementById('stat-current-rps').textContent = (snap.current_rps || 0).toFixed(0);
  document.getElementById('stat-avg-rps').textContent = (snap.average_rps || 0).toFixed(1) + ' req/s';
  document.getElementById('stat-success-req').textContent = (snap.success_requests || 0).toLocaleString();
  document.getElementById('stat-failed-req').textContent = (snap.failed_requests || 0).toLocaleString();
  
  // Latency metrics
  document.getElementById('lat-p50').textContent = formatDurationNs(snap.p50_latency);
  document.getElementById('lat-p90').textContent = formatDurationNs(snap.p90_latency);
  document.getElementById('lat-p95').textContent = formatDurationNs(snap.p95_latency);
  document.getElementById('lat-p99').textContent = formatDurationNs(snap.p99_latency);
  document.getElementById('lat-avg').textContent = formatDurationNs(snap.avg_latency);
  document.getElementById('lat-min').textContent = formatDurationNs(snap.min_latency);
  document.getElementById('lat-max').textContent = formatDurationNs(snap.max_latency);

  // Status badges
  const badge = document.getElementById('status-badge');
  const pauseBtn = document.getElementById('btn-pause');
  if (snap.is_finished) {
    badge.className = 'px-3 py-1 text-xs font-semibold rounded-full bg-emerald-500/20 text-emerald-400 border border-emerald-500/30';
    badge.textContent = 'COMPLETED';
    pauseBtn.disabled = true;
    pauseBtn.classList.add('opacity-50', 'cursor-not-allowed');
  } else if (snap.is_paused) {
    badge.className = 'px-3 py-1 text-xs font-semibold rounded-full bg-amber-500/20 text-amber-400 border border-amber-500/30';
    badge.textContent = 'PAUSED';
    pauseBtn.textContent = '▶ Resume';
  } else if (snap.is_running) {
    badge.className = 'px-3 py-1 text-xs font-semibold rounded-full bg-sky-500/20 text-sky-400 border border-sky-500/30 animate-pulse';
    badge.textContent = 'RUNNING';
    pauseBtn.textContent = '⏸ Pause';
  }

  // Update Status Code Breakdown
  const scContainer = document.getElementById('status-codes-list');
  scContainer.innerHTML = '';
  if (snap.status_codes && Object.keys(snap.status_codes).length > 0) {
    for (const [code, count] of Object.entries(snap.status_codes)) {
      const row = document.createElement('div');
      row.className = 'flex justify-between items-center text-sm py-1 border-b border-slate-800 last:border-0';
      
      let colorClass = 'text-emerald-400';
      if (code >= 400 && code < 500) colorClass = 'text-amber-400';
      if (code >= 500) colorClass = 'text-rose-400';
      if (code >= 300 && code < 400) colorClass = 'text-sky-400';

      const pct = ((count / snap.total_requests) * 100).toFixed(1);
      row.innerHTML = `
        <span class="font-mono font-bold ${colorClass}">HTTP ${code}</span>
        <span class="text-slate-300 font-mono">${count.toLocaleString()} <span class="text-xs text-slate-500">(${pct}%)</span></span>
      `;
      scContainer.appendChild(row);
    }
  } else {
    scContainer.innerHTML = '<span class="text-xs text-slate-500">No HTTP responses yet.</span>';
  }

  // Update Errors Breakdown
  const errContainer = document.getElementById('errors-list');
  errContainer.innerHTML = '';
  if (snap.errors && Object.keys(snap.errors).length > 0) {
    for (const [errText, count] of Object.entries(snap.errors)) {
      const item = document.createElement('div');
      item.className = 'text-xs text-rose-400 bg-rose-950/30 p-2 rounded border border-rose-900/40 mb-1 flex justify-between';
      item.innerHTML = `<span class="truncate mr-2 font-mono" title="${errText}">${errText}</span><span class="font-bold">${count}</span>`;
      errContainer.appendChild(item);
    }
  } else {
    errContainer.innerHTML = '<span class="text-xs text-slate-500">No network errors.</span>';
  }

  // Update charts only while running or if final frame hasn't been set
  if (!snap.is_finished) {
    const timeLabel = formatTimeOnly(new Date(snap.timestamp));
    if (chartTimeLabels.length >= maxDataPoints) {
      chartTimeLabels.shift();
      rpsCurrentData.shift();
      rpsAvgData.shift();
      latP50Data.shift();
      latP95Data.shift();
      latP99Data.shift();
      latAvgData.shift();
    }

    chartTimeLabels.push(timeLabel);
    rpsCurrentData.push(snap.current_rps);
    rpsAvgData.push(snap.average_rps);

    latP50Data.push(snap.p50_latency / 1000000);
    latP95Data.push(snap.p95_latency / 1000000);
    latP99Data.push(snap.p99_latency / 1000000);
    latAvgData.push(snap.avg_latency / 1000000);

    rpsChart.update();
    latencyChart.update();
  } else {
    // If finished, close SSE connection
    if (eventSource) {
      eventSource.close();
      eventSource = null;
    }
  }
}

function startSSE() {
  eventSource = new EventSource('/api/stream');
  
  eventSource.onmessage = (event) => {
    try {
      const snap = JSON.parse(event.data);
      updateUI(snap);
    } catch (e) {
      console.error('Failed to parse SSE event', e);
    }
  };

  eventSource.onerror = (err) => {
    console.warn('SSE connection closed or lost', err);
  };
}

async function togglePause() {
  await fetch('/api/pause', { method: 'POST' });
}

async function stopTest() {
  if (confirm('Are you sure you want to stop this load test?')) {
    await fetch('/api/stop', { method: 'POST' });
  }
}

function downloadReport(format) {
  if (!currentSnapshot) {
    alert('No metrics snapshot available to export yet.');
    return;
  }

  let content = '';
  let filename = 'traftest-report.md';
  let mimeType = 'text/markdown;charset=utf-8;';

  if (format === 'json') {
    content = JSON.stringify(currentSnapshot, null, 2);
    filename = 'traftest-report.json';
    mimeType = 'application/json;charset=utf-8;';
  } else {
    // Generate Markdown report directly in browser
    content = generateMarkdownReport(currentSnapshot);
  }

  const blob = new Blob([content], { type: mimeType });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}

function generateMarkdownReport(snap) {
  const durationMs = snap.duration / 1000000;
  let lines = [];
  lines.push('# 🚀 Goenma TrafTest - Traffic & Load Test Report\n');
  lines.push(`**Target URL:** \`${snap.target_url}\`  `);
  lines.push(`**Date:** ${new Date(snap.timestamp).toLocaleString()}  `);
  lines.push(`**Duration:** ${(durationMs / 1000).toFixed(2)}s  `);
  lines.push(`**Concurrency (Workers):** ${snap.concurrency}  `);
  lines.push(`**Target RPS:** ${snap.target_rps > 0 ? snap.target_rps + ' req/s' : 'Max uncapped'}  \n`);

  lines.push('## 📊 Summary\n');
  lines.push('| Metric | Value |');
  lines.push('| :--- | :--- |');
  lines.push(`| **Total Requests** | ${snap.total_requests.toLocaleString()} |`);
  const succPct = snap.total_requests ? ((snap.success_requests / snap.total_requests) * 100).toFixed(2) : '0.00';
  const failPct = snap.total_requests ? ((snap.failed_requests / snap.total_requests) * 100).toFixed(2) : '0.00';
  lines.push(`| **Successful (2xx/3xx)** | ${snap.success_requests.toLocaleString()} (${succPct}%) |`);
  lines.push(`| **Failed / Errors** | ${snap.failed_requests.toLocaleString()} (${failPct}%) |`);
  lines.push(`| **Average RPS** | ${snap.average_rps.toFixed(2)} req/s |\n`);

  lines.push('## ⏱️ Latency Percentiles\n');
  lines.push('| Percentile | Latency |');
  lines.push('| :--- | :--- |');
  lines.push(`| **Min** | ${formatDurationNs(snap.min_latency)} |`);
  lines.push(`| **p50 (Median)** | ${formatDurationNs(snap.p50_latency)} |`);
  lines.push(`| **p90** | ${formatDurationNs(snap.p90_latency)} |`);
  lines.push(`| **p95** | ${formatDurationNs(snap.p95_latency)} |`);
  lines.push(`| **p99** | ${formatDurationNs(snap.p99_latency)} |`);
  lines.push(`| **Max** | ${formatDurationNs(snap.max_latency)} |`);
  lines.push(`| **Average** | ${formatDurationNs(snap.avg_latency)} |\n`);

  if (snap.status_codes && Object.keys(snap.status_codes).length > 0) {
    lines.push('## 🏷️ HTTP Status Codes\n');
    lines.push('| Status Code | Count | Ratio |');
    lines.push('| :--- | :--- | :--- |');
    for (const [code, count] of Object.entries(snap.status_codes)) {
      const pct = ((count / snap.total_requests) * 100).toFixed(2);
      lines.push(`| \`${code}\` | ${count.toLocaleString()} | ${pct}% |`);
    }
    lines.push('');
  }

  if (snap.errors && Object.keys(snap.errors).length > 0) {
    lines.push('## ⚠️ Network / Protocol Errors\n');
    lines.push('| Error | Occurrences |');
    lines.push('| :--- | :--- |');
    for (const [errText, count] of Object.entries(snap.errors)) {
      lines.push(`| \`${errText}\` | ${count} |`);
    }
  }

  return lines.join('\n');
}

window.addEventListener('DOMContentLoaded', () => {
  initCharts();
  startSSE();

  document.getElementById('btn-pause').addEventListener('click', togglePause);
  document.getElementById('btn-stop').addEventListener('click', stopTest);
  document.getElementById('btn-export-json').addEventListener('click', () => downloadReport('json'));
  document.getElementById('btn-export-md').addEventListener('click', () => downloadReport('markdown'));

  // Help Modal Handlers
  const modal = document.getElementById('help-modal');
  const btnHelp = document.getElementById('btn-help-modal');
  const btnClose = document.getElementById('btn-close-modal');
  const btnCloseFooter = document.getElementById('btn-close-modal-footer');

  if (btnHelp && modal) {
    btnHelp.addEventListener('click', () => modal.classList.remove('hidden'));
    btnClose.addEventListener('click', () => modal.classList.add('hidden'));
    btnCloseFooter.addEventListener('click', () => modal.classList.add('hidden'));
    modal.addEventListener('click', (e) => {
      if (e.target === modal) modal.classList.add('hidden');
    });
  }
});
