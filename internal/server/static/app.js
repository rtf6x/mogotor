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

const charts = {};
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

function ensureChart(id, label, color) {
  const canvas = document.getElementById(id);
  if (charts[id]) {
    return charts[id];
  }
  charts[id] = new Chart(canvas, {
    type: "line",
    data: {
      labels: [],
      datasets: [{
        label,
        data: [],
        borderColor: color,
        backgroundColor: `${color}33`,
        fill: true,
        tension: 0.2,
        pointRadius: 0,
        borderWidth: 2,
      }],
    },
    options: chartOptions(false),
  });
  return charts[id];
}

function ensureMultiChart(id, datasets, showLegend = false) {
  const canvas = document.getElementById(id);
  if (charts[id]) {
    return charts[id];
  }
  charts[id] = new Chart(canvas, {
    type: "line",
    data: { labels: [], datasets },
    options: chartOptions(showLegend),
  });
  return charts[id];
}

function updateChart(chart, labels, values) {
  chart.data.labels = labels;
  chart.data.datasets[0].data = values;
  chart.update();
}

function updateMultiChart(chart, labels, series) {
  chart.data.labels = labels;
  series.forEach((values, index) => {
    chart.data.datasets[index].data = values;
  });
  chart.update();
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

function renderHistory(points) {
  const labels = points.map((point) => formatChartTime(point.timestamp));
  const cpu = ensureChart("cpu-chart", "CPU %", theme.brand);
  updateChart(cpu, labels, points.map((point) => point.cpuPercent));

  const memory = ensureChart("memory-chart", "Memory %", theme.ok);
  updateChart(memory, labels, points.map((point) => {
    if (!point.memoryTotalBytes) return 0;
    return (point.memoryUsedBytes / point.memoryTotalBytes) * 100;
  }));

  const disk = ensureChart("disk-chart", "Disk %", theme.brandSoft);
  updateChart(disk, labels, points.map((point) => point.diskUsedPercent));

  const network = ensureMultiChart("network-chart", [
    {
      label: "Receive",
      data: [],
      borderColor: theme.brand,
      backgroundColor: "rgba(255, 94, 0, 0.15)",
      fill: false,
      tension: 0.2,
      pointRadius: 0,
      borderWidth: 2,
    },
    {
      label: "Send",
      data: [],
      borderColor: theme.brandSoft,
      backgroundColor: "rgba(250, 189, 110, 0.15)",
      fill: false,
      tension: 0.2,
      pointRadius: 0,
      borderWidth: 2,
    },
  ], true);
  updateMultiChart(network, labels, [
    points.map((point) => point.netRecvBps),
    points.map((point) => point.netSendBps),
  ]);

  const load = ensureMultiChart("load-chart", [
    { label: "1 min", data: [], borderColor: theme.brand, tension: 0.2, pointRadius: 0, borderWidth: 2 },
    { label: "5 min", data: [], borderColor: theme.brandSoft, tension: 0.2, pointRadius: 0, borderWidth: 2 },
    { label: "15 min", data: [], borderColor: theme.muted, tension: 0.2, pointRadius: 0, borderWidth: 2 },
  ], true);
  updateMultiChart(load, labels, [
    points.map((point) => point.load1),
    points.map((point) => point.load5),
    points.map((point) => point.load15),
  ]);
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
  if (kind === "failed_password") return "failed password";
  if (kind === "invalid_user") return "invalid user";
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
      <td>${event.user}</td>
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
setInterval(refresh, 30000);
