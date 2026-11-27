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
const isMac = /Mac/.test(navigator.platform);

const state = {
  desktops: [],
  template: "missing",
  events: [],
  lastEvent: new Map(),
  cursor: null,
  focus: null,
  control: false,
};

const $ = (id) => document.getElementById(id);

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

function toast(msg) {
  const el = $("toast");
  el.textContent = msg;
  el.hidden = false;
  clearTimeout(toast.timer);
  toast.timer = setTimeout(() => { el.hidden = true; }, 6000);
}

// Formatting

function fmtMem(bytes) {
  const mb = bytes / 2 ** 20;
  return mb >= 1024 ? (mb / 1024).toFixed(1) + " GB" : Math.round(mb) + " MB";
}

function metricsText(d) {
  if (d.state !== "running") return "Stopped";
  if (!d.stats) return "";
  return `${Math.round(d.stats.cpu_percent)}% CPU, ${fmtMem(d.stats.mem_used_bytes)} of ${fmtMem(d.stats.mem_total_bytes)}`;
}

function ago(ms) {
  const s = Math.max(0, (Date.now() - ms) / 1000);
  if (s < 5) return "just now";
  if (s < 60) return Math.round(s) + " s ago";
  if (s < 3600) return Math.round(s / 60) + " min ago";
  return Math.round(s / 3600) + " h ago";
}

function describe(e) {
  return e.action.replaceAll("_", " ") + (e.detail ? " " + e.detail : "");
}

function who(e) {
  return e.source === "console" ? "You" : "Agent";
}

function lampState(d) {
  if (!d) return "off";
  if (state.focus === d.name && state.control) return "human";
  if (d.state !== "running") return "off";
  const e = state.lastEvent.get(d.name);
  const age = e ? Date.now() - Date.parse(e.time) : Infinity;
  if (e && !e.ok && age < 10000) return "error";
  if (e && e.source === "agent" && age < 3000) return "agent";
  return "ready";
}

const isRunning = (name) => state.desktops.some((d) => d.name === name && d.state === "running");

// Frames: each view fetches the next frame only after the previous one arrives.

class FrameLoop {
  constructor(img, name, interval) {
    this.img = img;
    this.name = name;
    this.interval = interval;
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
    const monitor = this.img.parentElement;
    if (isRunning(this.name)) {
      try {
        const res = await api(`/api/desktops/${encodeURIComponent(this.name)}/screen`);
        const url = URL.createObjectURL(await res.blob());
        const prev = this.url;
        this.url = url;
        this.img.onload = () => { if (prev) URL.revokeObjectURL(prev); };
        this.img.src = url;
        monitor.classList.remove("stale");
      } catch (_) {
        monitor.classList.add("stale");
      }
    } else {
      monitor.classList.add("stale");
    }
    if (!this.running) return;
    const wait = Math.max(0, this.interval - (performance.now() - began));
    this.timer = setTimeout(() => this.tick(), wait);
  }
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

function renderWall() {
  const wall = $("wall");
  const names = new Set(state.desktops.map((d) => d.name));
  for (const [name, tile] of tiles) {
    if (names.has(name)) continue;
    tile.loop.stop();
    observer.unobserve(tile.el);
    tile.el.remove();
    tiles.delete(name);
  }
  for (const d of state.desktops) {
    let tile = tiles.get(d.name);
    if (!tile) {
      const el = $("tile-template").content.firstElementChild.cloneNode(true);
      el.dataset.name = d.name;
      el.href = "#/d/" + encodeURIComponent(d.name);
      el.setAttribute("aria-label", `Open ${d.name}`);
      el.querySelector(".name").textContent = d.name;
      el.querySelector("img").alt = `Screen of ${d.name}`;
      tile = { el, loop: new FrameLoop(el.querySelector("img"), d.name, 1000), visible: false };
      tiles.set(d.name, tile);
      wall.append(el);
      observer.observe(el);
    }
    tile.el.querySelector(".lamp").dataset.state = lampState(d);
    tile.el.querySelector(".metrics").textContent = metricsText(d);
    const e = state.lastEvent.get(d.name);
    tile.el.querySelector(".last").textContent = e
      ? `${who(e)}: ${describe(e)}, ${ago(Date.parse(e.time))}`
      : "No activity yet";
  }
  $("empty").hidden = state.desktops.length > 0 || Boolean(state.focus);
}

function renderTemplate() {
  const text = {
    running: "Desktop template ready",
    stopped: "Desktop template stopped; it starts on the next new desktop",
    missing: "No desktop template yet; the first new desktop builds it",
  }[state.template] || "Desktop template " + state.template;
  $("template-text").textContent = text;
  $("template-lamp").dataset.state = state.template === "running" ? "ready" : "off";
}

// Focused view

let focusLoop = null;

function renderFocusHeader() {
  if (!state.focus) return;
  const d = state.desktops.find((x) => x.name === state.focus);
  const lamp = lampState(d);
  $("focus-lamp").dataset.state = lamp;
  $("focus-metrics").textContent = d ? metricsText(d) : "This desktop no longer exists";
  $("focus-mode").textContent = state.control ? "You have control" : lamp === "agent" ? "Agent is acting" : "Watching";
}

function renderFeed() {
  if (!state.focus) return;
  const items = state.events.filter((e) => e.desktop === state.focus).slice(-100).reverse();
  const rows = items.map((e) => {
    const li = document.createElement("li");
    const time = document.createElement("time");
    time.dateTime = e.time;
    time.textContent = new Date(e.time).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" });
    const what = document.createElement("span");
    what.className = "what";
    what.textContent = describe(e);
    const dur = document.createElement("span");
    dur.className = "dur";
    dur.textContent = e.duration_ms + " ms";
    const by = document.createElement("span");
    by.className = "who";
    by.textContent = who(e);
    li.append(time, what, dur, by);
    if (!e.ok) {
      const err = document.createElement("span");
      err.className = "err";
      err.textContent = e.error || "Failed";
      li.append(err);
    }
    return li;
  });
  if (!rows.length) {
    const li = document.createElement("li");
    li.className = "none";
    li.textContent = "Nothing yet. Agent actions on this desktop appear here as they happen.";
    rows.push(li);
  }
  $("feed").replaceChildren(...rows);
}

function setControl(on) {
  state.control = on;
  const btn = $("control-btn");
  btn.setAttribute("aria-pressed", String(on));
  btn.textContent = on ? "Release control" : "Take control";
  const monitor = $("focus-monitor");
  monitor.classList.toggle("controlled", on);
  monitor.tabIndex = on ? 0 : -1;
  $("control-hint").hidden = !on;
  if (on) monitor.focus();
  renderFocusHeader();
}

function route() {
  const m = location.hash.match(/^#\/d\/(.+)$/);
  const name = m ? decodeURIComponent(m[1]) : null;
  if (name === state.focus) return;
  if (state.control) setControl(false);
  if (focusLoop) focusLoop.stop();
  focusLoop = null;
  state.focus = name;
  $("wall").hidden = Boolean(name);
  $("focus").hidden = !name;
  for (const tile of tiles.values()) {
    if (!name && tile.visible) tile.loop.start();
    else tile.loop.stop();
  }
  if (name) {
    $("focus-name").textContent = name;
    $("focus-screen").removeAttribute("src");
    $("focus-screen").alt = `Live screen of ${name}`;
    focusLoop = new FrameLoop($("focus-screen"), name, 125);
    focusLoop.start();
    renderFocusHeader();
    renderFeed();
  }
  renderWall();
}

// Takeover input, sent in order

let sendChain = Promise.resolve();
function send(input) {
  const name = state.focus;
  sendChain = sendChain.then(() =>
    api(`/api/desktops/${encodeURIComponent(name)}/input`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(input),
    }).catch((err) => toast(err.message)));
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

$("control-btn").addEventListener("click", () => setControl(!state.control));

$("delete-btn").addEventListener("click", async () => {
  const name = state.focus;
  if (!confirm(`Delete ${name}? Everything on this desktop is lost.`)) return;
  try {
    await api(`/api/desktops/${encodeURIComponent(name)}`, { method: "DELETE" });
    toast(`Deleted ${name}`);
    location.hash = "#/";
    await refreshDesktops();
  } catch (err) {
    toast(err.message);
  }
});

$("create-form").addEventListener("submit", async (ev) => {
  ev.preventDefault();
  const input = $("create-name");
  const name = input.value.trim();
  const btn = ev.target.querySelector("button");
  btn.disabled = true;
  btn.textContent = "Creating";
  if (state.template !== "running") toast(`Creating ${name}. The first desktop builds the template, which takes a minute or two.`);
  try {
    await api("/api/desktops", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ name }) });
    input.value = "";
    toast(`Created ${name}`);
    await Promise.all([refreshDesktops(), refreshStatus()]);
  } catch (err) {
    toast(err.message);
  } finally {
    btn.disabled = false;
    btn.textContent = "New desktop";
  }
});

// Polling

async function refreshDesktops() {
  try {
    const body = await (await api("/api/desktops")).json();
    state.desktops = body.desktops;
    renderWall();
    renderFocusHeader();
  } catch (err) {
    toast(err.message);
  }
}

async function refreshStatus() {
  try {
    state.template = (await (await api("/api/status")).json()).template;
    renderTemplate();
  } catch (_) { /* shown by refreshDesktops */ }
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
    if (state.events.length > 1000) state.events.splice(0, state.events.length - 1000);
    if (body.events.length) {
      renderWall();
      renderFocusHeader();
      renderFeed();
    }
  } catch (_) { /* shown by refreshDesktops */ }
}

window.addEventListener("hashchange", route);
if (!token) toast("This page needs the link that adf console printed. Open that link to connect.");
route();
refreshStatus();
refreshDesktops();
refreshActivity();
setInterval(refreshDesktops, 2000);
setInterval(refreshActivity, 1000);
setInterval(refreshStatus, 5000);
// Lamps and "s ago" labels age even when nothing new arrives.
setInterval(() => { renderWall(); renderFocusHeader(); }, 1000);
