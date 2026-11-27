"use strict";

// The token arrives once in the URL printed by `adf console`; keep it for this tab only.
const params = new URLSearchParams(location.search);
if (params.get("t")) {
  sessionStorage.setItem("adf-token", params.get("t"));
  history.replaceState(null, "", location.pathname + location.hash);
}
const token = sessionStorage.getItem("adf-token") || "";

const SCREEN_W = 1280;
const SCREEN_H = 800;
const AGENT_ACTIVE_MS = 6000; // agents act every few seconds, so keep the tally lit between actions
const ERROR_SHOWN_MS = 10 * 60 * 1000;
const isMac = /Mac/.test(navigator.platform);

const state = {
  desktops: [],
  pending: new Set(), // names being created
  busy: new Map(), // name -> label of a running action, such as "Pausing"
  status: null,
  events: [],
  lastEvent: new Map(),
  cursor: null,
  focus: null,
  control: false,
  filter: "all",
  size: localStorage.getItem("adf-size") || "m",
  feedFilter: "all",
};

const $ = (id) => document.getElementById(id);
const enc = encodeURIComponent;

function el(tag, className, text) {
  const node = document.createElement(tag);
  if (className) node.className = className;
  if (text !== undefined) node.textContent = text;
  return node;
}

async function api(path, opts = {}) {
  const res = await fetch(path, { ...opts, headers: { ...(opts.headers || {}), Authorization: "Bearer " + token } });
  if (!res.ok) {
    let msg = res.status === 401
      ? "This page needs the link that adf console printed. Open that link to connect."
      : `The request failed with status ${res.status}.`;
    try {
      const body = await res.json();
      if (body.error) msg = body.error;
    } catch (_) { /* not JSON */ }
    throw new Error(msg);
  }
  return res;
}

function toast(msg, kind = "info") {
  const node = el("p", "toast " + kind, msg);
  node.setAttribute("role", kind === "error" ? "alert" : "status");
  $("toasts").append(node);
  setTimeout(() => node.remove(), kind === "error" ? 9000 : 4500);
}

// Formatting

function fmtBytes(bytes) {
  const gb = bytes / 2 ** 30;
  return gb >= 1 ? gb.toFixed(1) + " GB" : Math.round(bytes / 2 ** 20) + " MB";
}

function ago(ms) {
  const s = Math.max(0, (Date.now() - ms) / 1000);
  if (s < 5) return "just now";
  if (s < 60) return Math.round(s) + " s ago";
  if (s < 3600) return Math.round(s / 60) + " min ago";
  return Math.round(s / 3600) + " h ago";
}

function humanSeconds(sec) {
  return sec % 60 === 0 ? `${sec / 60} min` : `${sec} s`;
}

const describe = (e) => e.action.replaceAll("_", " ") + (e.detail ? " " + e.detail : "");
const who = (e) => (e.source === "console" ? "You" : "Agent");
const desktop = (name) => state.desktops.find((d) => d.name === name);
const recent = (e, ms) => Boolean(e) && Date.now() - Date.parse(e.time) < ms;

// What a person should know first about a desktop.
function condition(d) {
  if (!d) return { key: "gone", lamp: "off", text: "Deleted", tally: "" };
  if (d.state === "creating") return { key: "creating", lamp: "off", text: "Creating", tally: "" };
  if (state.busy.has(d.name)) return { key: "busy", lamp: "off", text: state.busy.get(d.name), tally: "" };
  if (state.focus === d.name && state.control) return { key: "human", lamp: "human", text: "You have control", tally: "human" };
  if (d.state === "paused") return { key: "paused", lamp: "off", text: "Paused", tally: "" };
  if (d.state !== "running") return { key: "stopped", lamp: "off", text: "Stopped", tally: "" };
  const e = state.lastEvent.get(d.name);
  if (e && !e.ok && recent(e, ERROR_SHOWN_MS)) return { key: "error", lamp: "error", text: "Last action failed", tally: "" };
  if (e && e.source === "agent" && recent(e, AGENT_ACTIVE_MS)) return { key: "agent", lamp: "agent", text: "Agent acting", tally: "agent" };
  return { key: "ready", lamp: "ready", text: "Ready", tally: "" };
}

function renderMetrics(node, d) {
  node.replaceChildren();
  if (!d || d.state !== "running" || !d.stats) return;
  const cpu = el("span", d.stats.cpu_percent >= 150 ? "hot" : "");
  cpu.append("CPU ", el("b", "", Math.round(d.stats.cpu_percent) + "%"));
  const ram = el("span", d.stats.mem_used_bytes / d.stats.mem_total_bytes > 0.9 ? "hot" : "");
  ram.append("RAM ", el("b", "", fmtBytes(d.stats.mem_used_bytes)));
  ram.title = `${fmtBytes(d.stats.mem_used_bytes)} of ${fmtBytes(d.stats.mem_total_bytes)} in use`;
  node.append(cpu, ram);
}

function setOverlay(node, d, withAction) {
  const c = condition(d);
  let title = "";
  let detail = "";
  let action = "";
  switch (c.key) {
    case "creating": title = "Creating desktop"; detail = "Usually about a second; the first one builds the template."; break;
    case "busy": title = c.text; break;
    case "paused": title = "Paused"; detail = "No memory in use. The next agent action resumes it with its windows intact."; action = "Resume"; break;
    case "stopped": title = "Stopped"; detail = "Its files are kept. The next agent action starts it again."; action = "Start"; break;
    case "gone": title = "This desktop was deleted"; break;
  }
  node.hidden = !title;
  if (!title) return;
  node.replaceChildren(el("strong", "", title));
  if (detail) node.append(el("span", "", detail));
  if (withAction && action) {
    const btn = el("button", "btn primary", action);
    btn.type = "button";
    btn.addEventListener("click", () => sleepOrWake(d.name));
    node.append(btn);
  }
}

// Frames: each view fetches its next frame only after the previous one arrives, never while
// the tab is hidden, and only for running desktops.

class FrameLoop {
  constructor(img, name, interval, onFrame) {
    this.img = img;
    this.name = name;
    this.interval = interval;
    this.onFrame = onFrame;
    this.running = false;
    this.url = null;
  }
  start() {
    if (this.running) return;
    this.running = true;
    this.tick();
  }
  stop() {
    this.running = false;
    clearTimeout(this.timer);
  }
  async tick() {
    if (!this.running) return;
    const began = performance.now();
    const d = desktop(this.name);
    if (!document.hidden && d && d.state === "running" && !state.busy.has(this.name)) {
      const monitor = this.img.closest(".monitor");
      try {
        const res = await api(`/api/desktops/${enc(this.name)}/screen`);
        const url = URL.createObjectURL(await res.blob());
        const prev = this.url;
        this.url = url;
        this.img.onload = () => { if (prev) URL.revokeObjectURL(prev); };
        this.img.src = url;
        monitor.classList.remove("stale");
        if (this.onFrame) this.onFrame();
      } catch (_) {
        monitor.classList.add("stale");
      }
    }
    if (!this.running) return;
    const every = typeof this.interval === "function" ? this.interval() : this.interval;
    this.timer = setTimeout(() => this.tick(), Math.max(0, every - (performance.now() - began)));
  }
}

// Thumbnails slow down as more of them are on screen.
function thumbInterval() {
  let visible = 0;
  for (const tile of tiles.values()) if (tile.visible) visible++;
  if (visible <= 6) return 1000;
  if (visible <= 12) return 1500;
  if (visible <= 24) return 2500;
  return 4000;
}

// Wall

const tiles = new Map();
const observer = new IntersectionObserver((entries) => {
  for (const entry of entries) {
    const tile = tiles.get(entry.target.dataset.name);
    if (!tile) continue;
    tile.visible = entry.isIntersecting;
    if (tile.visible && !state.focus) tile.loop.start();
    else tile.loop.stop();
  }
});

function wallList() {
  const list = [...state.desktops];
  for (const name of state.pending) if (!desktop(name)) list.push({ name, state: "creating" });
  return list;
}

function matches(d, filter) {
  const c = condition(d);
  switch (filter) {
    case "active": return c.key === "agent" || c.key === "human";
    case "asleep": return c.key === "paused" || c.key === "stopped";
    case "attention": return c.key === "error";
    default: return true;
  }
}

function makeTile(name) {
  const node = $("tile-template").content.firstElementChild.cloneNode(true);
  const href = "#/d/" + enc(name);
  node.dataset.name = name;
  const link = node.querySelector(".screen-link");
  link.href = href;
  link.setAttribute("aria-label", `Open ${name}`);
  const nameLink = node.querySelector(".name");
  nameLink.href = href;
  nameLink.textContent = name;
  node.querySelector("img").alt = `Screen of ${name}`;
  node.querySelector('[data-act="sleep"]').addEventListener("click", () => sleepOrWake(name));
  armDelete(node.querySelector('[data-act="delete"]'), name);
  return { el: node, visible: false, loop: new FrameLoop(node.querySelector("img"), name, thumbInterval) };
}

function updateTile(tile, d) {
  const node = tile.el;
  const c = condition(d);
  node.dataset.state = d.state;
  node.dataset.tally = c.tally;
  node.dataset.cond = c.key;
  node.querySelector(".lamp").dataset.state = c.lamp;
  node.querySelector(".state-text").textContent = c.text;
  renderMetrics(node.querySelector(".metrics"), d);
  setOverlay(node.querySelector(".overlay"), d, false);

  const last = node.querySelector(".last");
  last.replaceChildren();
  const e = state.lastEvent.get(d.name);
  if (e) {
    last.append(el("span", "who", who(e)), el("span", "what", describe(e)), el("span", "when", ago(Date.parse(e.time))));
    last.classList.toggle("failed", !e.ok);
    last.title = e.ok ? describe(e) : `${describe(e)}: ${e.error || "failed"}`;
  } else {
    last.append(el("span", "who", d.state === "creating" ? "Starting up" : "No activity yet"));
    last.classList.remove("failed");
  }

  const sleep = node.querySelector('[data-act="sleep"]');
  const label = d.state === "running" ? "Pause" : d.state === "paused" ? "Resume" : "Start";
  sleep.textContent = state.busy.get(d.name) || label;
  sleep.disabled = state.busy.has(d.name);
  sleep.setAttribute("aria-label", `${label} ${d.name}`);
  node.querySelector(".tile-actions").hidden = d.state === "creating";

  if (d.state !== "running") tile.loop.stop();
  else if (tile.visible && !state.focus) tile.loop.start();
}

function renderWall() {
  const wall = $("wall");
  wall.dataset.size = state.size;
  const list = wallList();
  const names = new Set(list.map((d) => d.name));
  for (const [name, tile] of tiles) {
    if (names.has(name)) continue;
    tile.loop.stop();
    observer.unobserve(tile.el);
    tile.el.remove();
    tiles.delete(name);
  }
  const counts = { all: 0, active: 0, asleep: 0, attention: 0 };
  let shown = 0;
  for (const d of list) {
    let tile = tiles.get(d.name);
    if (!tile) {
      tile = makeTile(d.name);
      tiles.set(d.name, tile);
      wall.append(tile.el);
      observer.observe(tile.el);
    }
    updateTile(tile, d);
    for (const f of Object.keys(counts)) if (matches(d, f)) counts[f]++;
    const show = d.state === "creating" || matches(d, state.filter);
    tile.el.hidden = !show;
    if (show) shown++;
  }
  for (const [f, n] of Object.entries(counts)) {
    const node = document.querySelector(`[data-count="${f}"]`);
    if (node) node.textContent = f === "all" || n > 0 ? String(n) : "";
  }
  const hasAny = list.length > 0;
  $("toolbar").hidden = !hasAny;
  $("empty").hidden = hasAny || Boolean(state.focus);
  $("no-match").hidden = !(hasAny && shown === 0);
}

// Header: how much room the computer has, and the rules in force.

function renderRoom() {
  const s = state.status;
  if (!s) return;
  const c = s.capacity;
  const room = document.querySelector(".room");
  if (c.host_known) {
    const total = c.host.total_mem_bytes;
    const free = c.host.available_mem_bytes;
    const usedPct = Math.min(100, Math.max(0, ((total - free) / total) * 100));
    $("room-used").style.width = usedPct + "%";
    $("room-reserve").style.width = Math.min(100, (c.limits.reserve_bytes / total) * 100) + "%";
    const meter = $("room-meter");
    meter.setAttribute("aria-valuenow", String(Math.round(usedPct)));
    meter.setAttribute("aria-valuetext", `${fmtBytes(free)} free of ${fmtBytes(total)}`);
    $("room-free").textContent = `${fmtBytes(free)} free of ${fmtBytes(total)}`;
  } else {
    $("room-free").textContent = "Memory not measured on this computer";
  }
  const fit = c.more_that_fit;
  room.dataset.level = fit === 0 ? "full" : fit > 0 && fit <= 2 ? "low" : "ok";
  $("room-fit").textContent = fit < 0 ? "" : fit === 0 ? "No room for another desktop" : `Room for ${fit} more`;
  $("room-fit").title = fit === 0 ? "Pause or delete a desktop to make room" : "";

  const counts = [
    ["running", c.running, "ready"],
    ["paused", c.paused, "off"],
    ["stopped", c.stopped, "off"],
  ].filter(([k, n]) => n > 0 || k === "running");
  $("counts").replaceChildren(...counts.map(([k, n, lamp]) => {
    const li = el("li");
    const l = el("span", "lamp");
    l.dataset.state = n > 0 ? lamp : "off";
    li.append(l, el("b", "", String(n)), " " + k);
    return li;
  }));

  $("template-note").textContent = {
    running: "Template ready",
    stopped: "Template stopped; it starts with the next new desktop",
    missing: "The first new desktop builds the template (1 to 2 minutes)",
  }[s.template] || `Template ${s.template}`;
  $("idle-note").textContent = s.idle_pause_seconds > 0
    ? `Idle desktops pause after ${humanSeconds(s.idle_pause_seconds)}`
    : "Idle desktops are not paused";

  const path = s.adf_path ? s.adf_path : "adf";
  $("mcp-cmd").textContent = `claude mcp add adf -- ${/\s/.test(path) ? `"${path}"` : path} mcp`;
}

// Focused view

let focusLoop = null;
const frameTimes = [];

function countFrame() {
  const now = performance.now();
  frameTimes.push(now);
  while (frameTimes.length && now - frameTimes[0] > 2000) frameTimes.shift();
}

function renderFocus() {
  if (!state.focus) return;
  const d = desktop(state.focus) || (state.pending.has(state.focus) ? { name: state.focus, state: "creating" } : null);
  const c = condition(d);
  $("focus-lamp").dataset.state = c.lamp;
  $("focus-state-text").textContent = c.text;
  renderMetrics($("focus-metrics"), d);
  const monitor = $("focus-monitor");
  monitor.dataset.tally = c.tally;
  monitor.dataset.state = d ? d.state : "gone";
  setOverlay($("focus-overlay"), d, true);

  const running = d && d.state === "running" && !state.busy.has(d.name);
  const sleep = $("sleep-btn");
  sleep.textContent = state.busy.get(state.focus) || (!d || d.state === "running" ? "Pause" : d.state === "paused" ? "Resume" : "Start");
  sleep.disabled = !d || d.state === "creating" || state.busy.has(state.focus);
  $("control-btn").disabled = !running;
  $("save-btn").disabled = !running;
  $("delete-btn").disabled = !d || d.state === "creating";
  if (!running && state.control) setControl(false);

  const fps = frameTimes.length > 1 ? (frameTimes.length - 1) / ((frameTimes[frameTimes.length - 1] - frameTimes[0]) / 1000) : 0;
  $("stage-meta").textContent = running ? (fps > 0 ? `Live, ${fps.toFixed(1)} frames per second` : "Connecting") : "";
}

function renderFeed() {
  if (!state.focus) return;
  const items = state.events
    .filter((e) => e.desktop === state.focus && (state.feedFilter === "all" || e.source === state.feedFilter))
    .slice(-150)
    .reverse();
  const rows = items.map((e) => {
    const li = el("li");
    const time = el("time", "", new Date(e.time).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" }));
    time.dateTime = e.time;
    li.append(time, el("span", "what", describe(e)), el("span", "dur", e.duration_ms + " ms"), el("span", "who", who(e)));
    if (!e.ok) li.append(el("span", "err", e.error || "Failed"));
    return li;
  });
  if (!rows.length) {
    rows.push(el("li", "none", state.feedFilter === "console"
      ? "Nothing from you yet. Take control to drive this desktop yourself."
      : "Nothing yet. Agent actions on this desktop appear here as they happen."));
  }
  $("feed").replaceChildren(...rows);
}

function setControl(on) {
  state.control = on;
  const btn = $("control-btn");
  btn.setAttribute("aria-pressed", String(on));
  btn.textContent = on ? "Release control" : "Take control";
  const monitor = $("focus-monitor");
  monitor.tabIndex = on ? 0 : -1;
  $("control-bar").hidden = !on;
  if (on) monitor.focus();
  renderFocus();
}

function route() {
  const m = location.hash.match(/^#\/d\/(.+)$/);
  const name = m ? decodeURIComponent(m[1]) : null;
  if (name === state.focus) return;
  if (state.control) setControl(false);
  if (focusLoop) focusLoop.stop();
  focusLoop = null;
  frameTimes.length = 0;
  state.focus = name;
  $("wall-view").hidden = Boolean(name);
  $("focus-view").hidden = !name;
  for (const tile of tiles.values()) {
    if (!name && tile.visible) tile.loop.start();
    else tile.loop.stop();
  }
  if (name) {
    $("focus-name").textContent = name;
    document.title = `${name}: adf console`;
    $("focus-screen").removeAttribute("src");
    $("focus-screen").alt = `Live screen of ${name}`;
    focusLoop = new FrameLoop($("focus-screen"), name, 125, countFrame);
    focusLoop.start();
    renderFocus();
    renderFeed();
  } else {
    document.title = "adf console";
  }
  renderWall();
}

// Actions

async function sleepOrWake(name) {
  const d = desktop(name);
  if (!d || state.busy.has(name)) return;
  const pausing = d.state === "running";
  state.busy.set(name, pausing ? "Pausing" : d.state === "paused" ? "Resuming" : "Starting");
  renderAll();
  try {
    await api(`/api/desktops/${enc(name)}/${pausing ? "pause" : "start"}`, { method: "POST" });
    toast(pausing ? `Paused ${name}. It uses no memory until the next action.` : `${name} is running`);
  } catch (err) {
    toast(err.message, "error");
  } finally {
    state.busy.delete(name);
    await Promise.all([refreshDesktops(), refreshStatus()]);
  }
}

// Deleting asks for a second click instead of opening a dialog.
function armDelete(btn, nameOrFn) {
  let timer = null;
  const reset = () => {
    timer = null;
    btn.classList.remove("armed");
    btn.textContent = "Delete";
  };
  btn.addEventListener("click", async () => {
    const name = typeof nameOrFn === "function" ? nameOrFn() : nameOrFn;
    if (!name) return;
    if (!timer) {
      btn.classList.add("armed");
      btn.textContent = "Confirm delete";
      timer = setTimeout(reset, 4000);
      return;
    }
    clearTimeout(timer);
    timer = null;
    btn.disabled = true;
    btn.textContent = "Deleting";
    try {
      await api(`/api/desktops/${enc(name)}`, { method: "DELETE" });
      toast(`Deleted ${name}`);
      if (state.focus === name) location.hash = "#/";
      await Promise.all([refreshDesktops(), refreshStatus()]);
    } catch (err) {
      toast(err.message, "error");
    } finally {
      btn.disabled = false;
      reset();
    }
  });
}

function nextName() {
  const taken = new Set([...state.desktops.map((d) => d.name), ...state.pending]);
  for (let i = 1; ; i++) if (!taken.has(`desk-${i}`)) return `desk-${i}`;
}

$("create-form").addEventListener("submit", async (ev) => {
  ev.preventDefault();
  const input = $("create-name");
  const name = input.value.trim() || nextName();
  if (state.pending.has(name) || desktop(name)) {
    toast(`A desktop named ${name} already exists`, "error");
    return;
  }
  input.value = "";
  state.pending.add(name);
  renderAll();
  if (state.status && state.status.template !== "running") {
    toast(`Creating ${name}. The first desktop builds the template, which takes a minute or two.`);
  }
  try {
    await api("/api/desktops", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ name }) });
    await refreshDesktops();
  } catch (err) {
    toast(err.message, "error");
  } finally {
    state.pending.delete(name);
    await Promise.all([refreshDesktops(), refreshStatus()]);
  }
});

$("control-btn").addEventListener("click", () => setControl(!state.control));
$("sleep-btn").addEventListener("click", () => sleepOrWake(state.focus));
armDelete($("delete-btn"), () => state.focus);

$("save-btn").addEventListener("click", async () => {
  try {
    const res = await api(`/api/desktops/${enc(state.focus)}/screen`);
    const url = URL.createObjectURL(await res.blob());
    const a = el("a");
    a.href = url;
    a.download = `${state.focus}-${new Date().toISOString().replace(/[:.]/g, "-")}.jpg`;
    a.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  } catch (err) {
    toast(err.message, "error");
  }
});

$("copy-cmd").addEventListener("click", async () => {
  try {
    await navigator.clipboard.writeText($("mcp-cmd").textContent);
    toast("Copied. Paste it into a terminal, then start a new Claude Code session.");
  } catch (_) {
    toast("Copy failed; select the command and copy it by hand.", "error");
  }
});

function wireSeg(groupId, attr, onPick) {
  const group = $(groupId);
  group.addEventListener("click", (ev) => {
    const btn = ev.target.closest("button");
    if (!btn) return;
    for (const b of group.querySelectorAll("button")) b.setAttribute("aria-checked", String(b === btn));
    onPick(btn.dataset[attr]);
  });
}
wireSeg("filters", "filter", (v) => { state.filter = v; renderWall(); });
wireSeg("sizes", "size", (v) => { state.size = v; localStorage.setItem("adf-size", v); renderWall(); });
wireSeg("feed-filters", "feed", (v) => { state.feedFilter = v; renderFeed(); });
for (const b of $("sizes").querySelectorAll("button")) b.setAttribute("aria-checked", String(b.dataset.size === state.size));

// Takeover input, sent in order

let sendChain = Promise.resolve();
function send(input) {
  const name = state.focus;
  sendChain = sendChain.then(() =>
    api(`/api/desktops/${enc(name)}/input`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(input),
    }).catch((err) => toast(err.message, "error")));
  return sendChain;
}

function toScreen(ev) {
  const r = $("focus-screen").getBoundingClientRect();
  const scale = Math.min(r.width / SCREEN_W, r.height / SCREEN_H);
  const left = r.left + (r.width - SCREEN_W * scale) / 2;
  const top = r.top + (r.height - SCREEN_H * scale) / 2;
  const x = Math.round((ev.clientX - left) / scale);
  const y = Math.round((ev.clientY - top) / scale);
  if (x < 0 || y < 0 || x >= SCREEN_W || y >= SCREEN_H) return null;
  return { x, y };
}

const screenEl = $("focus-screen");
let pressed = null;
screenEl.addEventListener("mousedown", (ev) => {
  if (!state.control) return;
  ev.preventDefault();
  $("focus-monitor").focus();
  pressed = toScreen(ev);
});
screenEl.addEventListener("mouseup", (ev) => {
  if (!state.control || !pressed) return;
  const start = pressed;
  pressed = null;
  const end = toScreen(ev);
  if (!end) return;
  const button = ["left", "middle", "right"][ev.button] || "left";
  const moved = Math.abs(end.x - start.x) + Math.abs(end.y - start.y) > 6;
  // Each browser click becomes one remote click; two quick clicks make a remote double-click.
  if (moved && button === "left") send({ kind: "drag", x: start.x, y: start.y, to_x: end.x, to_y: end.y });
  else send({ kind: "click", x: end.x, y: end.y, button });
});
screenEl.addEventListener("contextmenu", (ev) => { if (state.control) ev.preventDefault(); });

let wheelTotal = 0;
let wheelAt = null;
let wheelTimer = null;
screenEl.addEventListener("wheel", (ev) => {
  if (!state.control) return;
  ev.preventDefault();
  const p = toScreen(ev);
  if (!p) return;
  wheelAt = p;
  wheelTotal += ev.deltaY;
  clearTimeout(wheelTimer);
  wheelTimer = setTimeout(() => {
    const amount = Math.max(1, Math.min(10, Math.round(Math.abs(wheelTotal) / 100)));
    send({ kind: "scroll", x: wheelAt.x, y: wheelAt.y, dir: wheelTotal > 0 ? "down" : "up", amount });
    wheelTotal = 0;
  }, 80);
}, { passive: false });

const MODIFIERS = new Set(["Shift", "Control", "Alt", "Meta", "CapsLock", "Fn"]);
let typed = "";
let typeTimer = null;
function flushTyped() {
  clearTimeout(typeTimer);
  if (!typed) return;
  const text = typed;
  typed = "";
  send({ kind: "type", text });
}
$("focus-monitor").addEventListener("keydown", (ev) => {
  if (!state.control || MODIFIERS.has(ev.key)) return;
  ev.preventDefault();
  ev.stopPropagation();
  // On a Mac, Command shortcuts map to Ctrl on the Linux desktop.
  const ctrl = ev.ctrlKey || (isMac && ev.metaKey);
  const meta = isMac ? false : ev.metaKey;
  if (ev.key.length === 1 && !ctrl && !ev.altKey && !meta) {
    typed += ev.key;
    clearTimeout(typeTimer);
    typeTimer = setTimeout(flushTyped, 150);
    return;
  }
  flushTyped();
  send({ kind: "key", key: ev.key, ctrl, alt: ev.altKey, shift: ev.shiftKey, meta });
});

for (const btn of document.querySelectorAll(".quick-keys [data-key]")) {
  btn.addEventListener("click", () => send({ kind: "key", ...JSON.parse(btn.dataset.key) }));
}
$("send-form").addEventListener("submit", (ev) => {
  ev.preventDefault();
  const input = $("send-text");
  if (!input.value) return;
  send({ kind: "type", text: input.value });
  input.value = "";
});

// Escape on the focused view goes back to the wall, unless keys are going to the desktop.
document.addEventListener("keydown", (ev) => {
  if (ev.key !== "Escape" || !state.focus) return;
  if (ev.target.closest && ev.target.closest("input, textarea, #focus-monitor")) return;
  if (state.control) setControl(false);
  else location.hash = "#/";
});

// Polling, skipped while the tab is hidden.

async function refreshDesktops() {
  try {
    state.desktops = (await (await api("/api/desktops")).json()).desktops;
    renderAll();
  } catch (err) {
    toast(err.message, "error");
  }
}

async function refreshStatus() {
  try {
    state.status = await (await api("/api/status")).json();
    renderRoom();
  } catch (_) { /* reported by refreshDesktops */ }
}

async function refreshActivity() {
  try {
    const query = state.cursor === null ? "" : "?cursor=" + state.cursor;
    const body = await (await api("/api/activity" + query)).json();
    state.cursor = body.cursor;
    for (const e of body.events) {
      state.events.push(e);
      state.lastEvent.set(e.desktop, e);
    }
    if (state.events.length > 1500) state.events.splice(0, state.events.length - 1500);
    if (body.events.length) {
      renderAll();
      renderFeed();
    }
  } catch (_) { /* reported by refreshDesktops */ }
}

function renderAll() {
  renderWall();
  renderFocus();
}

function every(ms, fn) {
  setInterval(() => { if (!document.hidden) fn(); }, ms);
}

window.addEventListener("hashchange", route);
document.addEventListener("visibilitychange", () => {
  if (!document.hidden) {
    refreshDesktops();
    refreshStatus();
    refreshActivity();
  }
});

if (!token) toast("This page needs the link that adf console printed. Open that link to connect.", "error");
route();
refreshStatus();
refreshDesktops();
refreshActivity();
every(2000, refreshDesktops);
every(1000, refreshActivity);
every(4000, refreshStatus);
// Lamps and "s ago" labels age even when nothing new arrives.
every(1000, renderAll);
