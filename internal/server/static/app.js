const theme = {
  brand: "#ff5e00",
  brandSoft: "#fabd6e",
  text: "#f2f2f7",
  muted: "#9aa3b2",
  grid: "rgba(255, 255, 255, 0.08)",
  ok: "#2ecc71",
  warn: "#fabd6e",
  bad: "#e74c3c",
};

const PREVIEW_HOURS = 1;

const CHART_SPECS = {
  cpu: {
    id: "cpu-chart",
    title: "CPU usage",
    multi: false,
    datasets: [{ label: "CPU %", color: theme.brand, fill: true, value: (point) => point.cpuPercent }],
  },
  memory: {
    id: "memory-chart",
    title: "Memory usage",
    multi: false,
    datasets: [{
      label: "Memory %",
      color: theme.ok,
      fill: true,
      value: (point) => (point.memoryTotalBytes ? (point.memoryUsedBytes / point.memoryTotalBytes) * 100 : 0),
    }],
  },
  disk: {
    id: "disk-chart",
    title: "Disk usage",
    multi: false,
    datasets: [{ label: "Disk %", color: theme.brandSoft, fill: true, value: (point) => point.diskUsedPercent }],
  },
  network: {
    id: "network-chart",
    title: "Network throughput",
    multi: true,
    datasets: [
      { label: "Receive", color: theme.brand, fill: false, value: (point) => point.netRecvBps },
      { label: "Send", color: theme.brandSoft, fill: false, value: (point) => point.netSendBps },
    ],
  },
  load: {
    id: "load-chart",
    title: "Load average",
    multi: true,
    datasets: [
      { label: "1 min", color: theme.brand, fill: false, value: (point) => point.load1 },
      { label: "5 min", color: theme.brandSoft, fill: false, value: (point) => point.load5 },
      { label: "15 min", color: theme.muted, fill: false, value: (point) => point.load15 },
    ],
  },
};

const charts = {};
let historyPoints = [];
let modalChart = null;
let activeChartKey = null;
const chartDefaults = {
  responsive: true,
  maintainAspectRatio: false,
  animation: false,
  scales: {
    x: {
      ticks: { color: theme.muted, maxTicksLimit: 8 },
      grid: { color: theme.grid },
    },
    y: {
      ticks: { color: theme.muted },
      grid: { color: theme.grid },
    },
  },
  plugins: {
    legend: {
      labels: { color: theme.text, boxWidth: 12, boxHeight: 12 },
    },
  },
};

function chartOptions(showLegend = false) {
  return {
    ...chartDefaults,
    plugins: {
      ...chartDefaults.plugins,
      legend: {
        ...chartDefaults.plugins.legend,
        display: showLegend,
      },
    },
  };
}

function formatBytes(bytes) {
  if (!bytes) return "0 B";
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  let value = Number(bytes);
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit += 1;
  }
  return `${value.toFixed(value >= 10 ? 0 : 1)} ${units[unit]}`;
}

function formatPercent(value) {
  return `${Number(value || 0).toFixed(1)}%`;
}

function formatTime(iso) {
  const date = new Date(iso);
  return date.toLocaleString();
}

function formatChartTime(iso) {
  const date = new Date(iso);
  return date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

function statusClass(state) {
  const value = String(state || "").toLowerCase();
  if (["active", "online", "running"].includes(value)) return "ok";
  if (["failed", "errored", "stopped", "inactive", "dead"].includes(value)) return "bad";
  return "warn";
}

function pointsInLastHours(points, hours) {
  if (!points.length) {
    return [];
  }
  const cutoff = Date.now() - hours * 60 * 60 * 1000;
  return points.filter((point) => new Date(point.timestamp).getTime() >= cutoff);
}

function buildChartData(points, spec) {
  const labels = points.map((point) => formatChartTime(point.timestamp));
  const datasets = spec.datasets.map((dataset) => ({
    label: dataset.label,
    data: points.map((point) => dataset.value(point)),
    borderColor: dataset.color,
    backgroundColor: dataset.fill ? `${dataset.color}33` : "transparent",
    fill: !!dataset.fill,
    tension: 0.2,
    pointRadius: 0,
    borderWidth: 2,
  }));
  return { labels, datasets };
}

function createChart(canvas, spec, showLegend = false) {
  return new Chart(canvas, {
    type: "line",
    data: buildChartData([], spec),
    options: chartOptions(showLegend),
  });
}

function updateChartInstance(chart, points, spec) {
  const data = buildChartData(points, spec);
  chart.data.labels = data.labels;
  chart.data.datasets.forEach((dataset, index) => {
    dataset.data = data.datasets[index].data;
  });
  chart.update();
}

function ensurePreviewChart(spec) {
  if (charts[spec.id]) {
    return charts[spec.id];
  }
  const canvas = document.getElementById(spec.id);
  charts[spec.id] = createChart(canvas, spec, spec.multi);
  return charts[spec.id];
}

function renderHistory(points) {
  historyPoints = points;
  const previewPoints = pointsInLastHours(points, PREVIEW_HOURS);

  Object.values(CHART_SPECS).forEach((spec) => {
    const chart = ensurePreviewChart(spec);
    updateChartInstance(chart, previewPoints, spec);
  });

  refreshModalChart();
}

function openChartModal(chartKey) {
  const spec = CHART_SPECS[chartKey];
  if (!spec) {
    return;
  }

  activeChartKey = chartKey;
  const modal = document.getElementById("chart-modal");
  document.getElementById("chart-modal-title").textContent = `${spec.title} · 24 hours`;
  modal.hidden = false;
  document.body.classList.add("modal-open");

  const canvas = document.getElementById("chart-modal-canvas");
  if (modalChart) {
    modalChart.destroy();
    modalChart = null;
  }
  modalChart = createChart(canvas, spec, spec.multi);
  updateChartInstance(modalChart, historyPoints, spec);
  requestAnimationFrame(() => modalChart.resize());
}

function closeChartModal() {
  const modal = document.getElementById("chart-modal");
  if (modal.hidden) {
    return;
  }
  modal.hidden = true;
  document.body.classList.remove("modal-open");
  activeChartKey = null;
}

function refreshModalChart() {
  if (!activeChartKey || !modalChart) {
    return;
  }
  const spec = CHART_SPECS[activeChartKey];
  if (!spec) {
    return;
  }
  updateChartInstance(modalChart, historyPoints, spec);
}

function initChartModal() {
  document.querySelectorAll(".chart-box[data-chart]").forEach((box) => {
    box.addEventListener("click", () => openChartModal(box.dataset.chart));
    box.addEventListener("keydown", (event) => {
      if (event.key === "Enter" || event.key === " ") {
        event.preventDefault();
        openChartModal(box.dataset.chart);
      }
    });
  });

  document.getElementById("chart-modal-close").addEventListener("click", closeChartModal);
  document.querySelectorAll("[data-close-modal]").forEach((element) => {
    element.addEventListener("click", closeChartModal);
  });
  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape") {
      closeChartModal();
    }
  });
}

function renderSummary(snapshot) {
  const system = snapshot.system;
  const memPercent = system.memoryTotalBytes
    ? (system.memoryUsedBytes / system.memoryTotalBytes) * 100
    : 0;

  const cards = [
    { label: "CPU", value: formatPercent(system.cpuPercent) },
    { label: "Memory", value: formatPercent(memPercent), sub: `${formatBytes(system.memoryUsedBytes)} / ${formatBytes(system.memoryTotalBytes)}` },
    { label: "Disk", value: formatPercent(system.diskUsedPercent), sub: `${formatBytes(system.diskUsedBytes)} / ${formatBytes(system.diskTotalBytes)}` },
    { label: "Load (1m)", value: Number(system.load1 || 0).toFixed(2), sub: `5m ${Number(system.load5 || 0).toFixed(2)} · 15m ${Number(system.load15 || 0).toFixed(2)}` },
    { label: "Network", value: `${formatBytes(system.netRecvBps)}/s`, sub: `↑ ${formatBytes(system.netSendBps)}/s` },
    { label: "Uptime", value: formatUptime(system.uptimeSeconds) },
  ];

  document.getElementById("summary-cards").innerHTML = cards.map((card) => `
    <div class="card">
      <div class="card-label">${card.label}</div>
      <div class="card-value">${card.value}</div>
      ${card.sub ? `<div class="card-sub">${card.sub}</div>` : ""}
    </div>
  `).join("");
}

function formatUptime(seconds) {
  const total = Number(seconds || 0);
  const days = Math.floor(total / 86400);
  const hours = Math.floor((total % 86400) / 3600);
  const minutes = Math.floor((total % 3600) / 60);
  if (days > 0) return `${days}d ${hours}h`;
  if (hours > 0) return `${hours}h ${minutes}m`;
  return `${minutes}m`;
}

function formatPM2Uptime(uptimeMs, status) {
  const state = String(status || "").toLowerCase();
  if (!uptimeMs || !["online", "launching"].includes(state)) {
    return "—";
  }
  const seconds = Math.max(0, Math.floor((Date.now() - uptimeMs) / 1000));
  return formatUptime(seconds);
}

function renderPM2(pm2) {
  const status = document.getElementById("pm2-status");
  const tbody = document.querySelector("#pm2-table tbody");

  if (!pm2.available) {
    status.textContent = pm2.error || "PM2 unavailable";
    status.className = "status-line error";
    tbody.innerHTML = "";
    return;
  }

  status.textContent = `${pm2.processes.length} process(es)`;
  status.className = "status-line";
  tbody.innerHTML = pm2.processes.map((proc) => `
    <tr>
      <td>${proc.name}</td>
      <td><span class="pill ${statusClass(proc.status)}">${proc.status}</span></td>
      <td>${formatPercent(proc.cpu)}</td>
      <td>${formatBytes(proc.memoryBytes)}</td>
      <td>${formatPM2Uptime(proc.uptimeMs, proc.status)}</td>
      <td>${proc.restarts}</td>
    </tr>
  `).join("");
}

function renderDocker(docker) {
  const status = document.getElementById("docker-status");
  const tbody = document.querySelector("#docker-table tbody");

  if (!docker.available) {
    status.textContent = docker.error || "Docker unavailable";
    status.className = "status-line error";
    tbody.innerHTML = "";
    return;
  }

  status.textContent = `${docker.containers.length} container(s)`;
  status.className = "status-line";
  tbody.innerHTML = docker.containers.map((container) => `
    <tr>
      <td>${container.name || container.id}</td>
      <td>${formatPercent(container.cpuPercent)}</td>
      <td>${formatBytes(container.memoryBytes)}${container.memoryLimit ? ` / ${formatBytes(container.memoryLimit)}` : ""}</td>
      <td>${container.pids}</td>
    </tr>
  `).join("") || `<tr><td colspan="4">No running containers</td></tr>`;
}

function renderServices(services) {
  const root = document.getElementById("services-list");
  root.innerHTML = services.map((service) => `
    <div class="service-item">
      <div>
        <div class="service-name">${service.name}</div>
        <div class="service-desc">${service.description || "—"}${service.mainPid ? ` · PID ${service.mainPid}` : ""}</div>
        ${service.error ? `<div class="service-desc" style="color: var(--bad)">${service.error}</div>` : ""}
      </div>
      <div class="service-status">
        <span class="pill ${statusClass(service.active)}">${service.active}</span>
        <div class="service-desc">${service.subState || ""}${service.memoryBytes ? ` · ${formatBytes(service.memoryBytes)}` : ""}</div>
      </div>
    </div>
  `).join("");
}

function renderMongo(mongo) {
  const root = document.getElementById("mongo-panel");
  if (!mongo.available) {
    root.innerHTML = `<div class="service-item"><div><div class="service-name">MongoDB</div><div class="service-desc">${mongo.error || "Unavailable"}</div></div><div class="service-status"><span class="pill bad">offline</span></div></div>`;
    return;
  }

  const rows = mongo.source === "process"
    ? [
        ["Process memory", mongo.processMemoryBytes ? formatBytes(mongo.processMemoryBytes) : "—"],
        ["Resident memory", mongo.memoryResidentMb != null ? `${mongo.memoryResidentMb} MB` : "—"],
      ]
    : [
        ["Version", mongo.version || "—"],
        ["Uptime", mongo.uptimeSeconds ? formatUptime(mongo.uptimeSeconds) : "—"],
        ["Connections", mongo.connections ? `${mongo.connections}${mongo.connectionsAvailable ? ` / ${mongo.connectionsAvailable}` : ""}` : "—"],
        ["Resident memory", mongo.memoryResidentMb != null ? `${mongo.memoryResidentMb} MB` : "—"],
        ["Virtual memory", mongo.memoryVirtualMb ? `${mongo.memoryVirtualMb} MB` : "—"],
        ["WiredTiger cache", mongo.cacheBytes ? `${formatBytes(mongo.cacheBytes)}${mongo.cacheMaxBytes ? ` / ${formatBytes(mongo.cacheMaxBytes)}` : ""}` : "—"],
        ["Queries", mongo.opsQuery ? Number(mongo.opsQuery).toLocaleString() : "—"],
        ["Updates", mongo.opsUpdate ? Number(mongo.opsUpdate).toLocaleString() : "—"],
      ];

  root.innerHTML = rows.map(([label, value]) => `
    <div class="mongo-item"><span>${label}</span><strong>${value}</strong></div>
  `).join("") + (mongo.source ? `<div class="service-desc">source: ${mongo.source}</div>` : "");
}

function sshKindLabel(kind) {
  if (kind === "failed_password") return "wrong password";
  if (kind === "invalid_user") return "unknown user";
  return kind || "—";
}

function renderSSH(ssh) {
  const loginStatus = document.getElementById("ssh-login-status");
  const failureStatus = document.getElementById("ssh-failure-status");
  const loginBody = document.querySelector("#ssh-login-table tbody");
  const failureBody = document.querySelector("#ssh-failure-table tbody");

  if (!ssh || !ssh.available) {
    const message = ssh?.error || "SSH auth logs unavailable";
    loginStatus.textContent = message;
    loginStatus.className = "status-line error";
    failureStatus.textContent = message;
    failureStatus.className = "status-line error";
    loginBody.innerHTML = "";
    failureBody.innerHTML = "";
    return;
  }

  loginStatus.textContent = `${(ssh.logins || []).length} recent login(s)`;
  loginStatus.className = "status-line";
  failureStatus.textContent = `${(ssh.failures || []).length} recent failure(s)`;
  failureStatus.className = "status-line";

  loginBody.innerHTML = (ssh.logins || []).map((event) => `
    <tr>
      <td>${formatTime(event.timestamp)}</td>
      <td>${event.user}</td>
      <td class="mono">${event.ip}</td>
      <td>${event.method || "—"}</td>
    </tr>
  `).join("") || `<tr><td colspan="4">No recent logins</td></tr>`;

  failureBody.innerHTML = (ssh.failures || []).map((event) => `
    <tr>
      <td>${formatTime(event.timestamp)}</td>
      <td class="${event.kind === "invalid_user" ? "mono ssh-attempted-user" : ""}">${event.user || "—"}</td>
      <td class="mono">${event.ip}</td>
      <td><span class="pill bad">${sshKindLabel(event.kind)}</span></td>
    </tr>
  `).join("") || `<tr><td colspan="4">No recent failures</td></tr>`;
}

async function refresh() {
  const [snapshotRes, historyRes] = await Promise.all([
    fetch("/api/snapshot"),
    fetch("/api/history"),
  ]);

  const snapshot = await snapshotRes.json();
  const history = await historyRes.json();

  document.getElementById("last-updated").textContent = `Updated ${formatTime(snapshot.timestamp)}`;
  document.getElementById("sample-age").textContent = "live";

  renderSummary(snapshot);
  renderHistory(history.points || []);
  renderPM2(snapshot.pm2);
  renderDocker(snapshot.docker);
  renderServices(snapshot.services || []);
  renderMongo(snapshot.mongo);
  renderSSH(snapshot.ssh);
}

refresh();
initChartModal();
setInterval(refresh, 30000);
