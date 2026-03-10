const ui = {
  runBtn: document.getElementById("runBtn"),
  stopBtn: document.getElementById("stopBtn"),
  resumeBtn: document.getElementById("resumeBtn"),
  clearRunBtn: document.getElementById("clearRunBtn"),
  runStatus: document.getElementById("runStatus"),
  tpMode: document.getElementById("tpMode"),
  stepDelay: document.getElementById("stepDelay"),
  stepDelayLabel: document.getElementById("stepDelayLabel"),
  watchInput: document.getElementById("watchInput"),
  addWatchBtn: document.getElementById("addWatchBtn"),
  watchList: document.getElementById("watchList"),
  selectedTpLabel: document.getElementById("selectedTpLabel"),
  tpVars: document.getElementById("tpVars"),
  gutter: document.getElementById("gutter"),
  editor: document.getElementById("editor"),
  cursorPos: document.getElementById("cursorPos"),
  outputBox: document.getElementById("outputBox"),
  timeline: document.getElementById("timeline"),
};

const app = {
  explicitTimepoints: new Map(),
  executedTimepoints: [],
  outputEntries: [],
  timelines: [],
  activeTimelineId: null,
  timelineCounter: 0,
  timepointEdits: new Map(),
  playbackOutputEntries: null,
  watchVars: ["retries", "status", "orderID"],
  runtimeState: {},
  selectedExecutedId: null,
  mode: "explicit",
  delayMs: 80,
  running: false,
  playbackRunning: false,
  stopRequested: false,
  runStart: 0,
  currentLine: 1,
  runCounter: 0,
};

ui.editor.value = `function processOrder() {
  let orderID = "A-1021";
  let retries = 2;
  let status = "created";

  if (retries > 0) {
    status = "retrying";
  }

  console.log("order", orderID);
  console.log("status", status);
}

processOrder();`;

function wait(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function deepClone(value) {
  if (typeof structuredClone === "function") {
    try {
      return structuredClone(value);
    } catch (_) {
      // fallback below
    }
  }
  return JSON.parse(JSON.stringify(value ?? null));
}

function shortValue(value) {
  if (value === undefined) return "undefined";
  if (typeof value === "string") return JSON.stringify(value);
  if (typeof value === "function") return "[Function]";
  try {
    const raw = JSON.stringify(value);
    if (!raw) return String(value);
    return raw.length > 46 ? `${raw.slice(0, 43)}...` : raw;
  } catch (_) {
    return String(value);
  }
}

function setStatus(text, tone = "idle") {
  ui.runStatus.textContent = text;
  ui.runStatus.style.color = tone === "error" ? "#ff9ca4" : tone === "ok" ? "#71e6b2" : "#8eb0bf";
}

function defaultTpName(line) {
  return `TP-L${line}`;
}

function lineCount() {
  return ui.editor.value.split("\n").length;
}

function updateCursorPos() {
  const index = ui.editor.selectionStart;
  const before = ui.editor.value.slice(0, index);
  const lines = before.split("\n");
  const line = lines.length;
  const col = lines[lines.length - 1].length + 1;
  ui.cursorPos.textContent = `L${line}:C${col}`;
}

function valuesEqual(a, b) {
  if (a === b) return true;
  try {
    return JSON.stringify(a) === JSON.stringify(b);
  } catch (_) {
    return String(a) === String(b);
  }
}

function toEditorValue(value) {
  if (value === undefined) return "undefined";
  if (typeof value === "string") return value;
  try {
    return JSON.stringify(value);
  } catch (_) {
    return String(value);
  }
}

function parseEditorValue(raw) {
  const trimmed = raw.trim();
  if (trimmed === "undefined") return undefined;
  if (trimmed === "") return "";

  const looksLikeJson =
    trimmed.startsWith("{") ||
    trimmed.startsWith("[") ||
    trimmed.startsWith('"') ||
    trimmed === "true" ||
    trimmed === "false" ||
    trimmed === "null" ||
    /^-?\d+(\.\d+)?$/.test(trimmed);

  if (looksLikeJson) {
    try {
      return JSON.parse(trimmed);
    } catch (_) {
      return raw;
    }
  }

  return raw;
}

function clearObject(obj) {
  Object.keys(obj).forEach((key) => {
    delete obj[key];
  });
}

function createTimeline(label, parentTimelineId = null, parentTpId = null, parentNodeIndex = -1) {
  app.timelineCounter += 1;
  return {
    id: `tl-${app.timelineCounter}`,
    label,
    parentTimelineId,
    parentTpId,
    parentNodeIndex,
    nodes: [],
    outputs: [],
  };
}

function getTimeline(timelineId) {
  return app.timelines.find((timeline) => timeline.id === timelineId) || null;
}

function setActiveTimeline(timelineId) {
  const timeline = getTimeline(timelineId);
  if (!timeline) {
    app.activeTimelineId = null;
    app.executedTimepoints = [];
    app.outputEntries = [];
    return;
  }

  app.activeTimelineId = timeline.id;
  app.executedTimepoints = timeline.nodes;
  app.outputEntries = timeline.outputs;

  const hasSelected = app.selectedExecutedId && timeline.nodes.some((node) => node.id === app.selectedExecutedId);
  if (!hasSelected) {
    app.selectedExecutedId = timeline.nodes.length ? timeline.nodes[timeline.nodes.length - 1].id : null;
  }
}

function findTimepointById(timepointId) {
  if (!timepointId) return null;

  for (const timeline of app.timelines) {
    const index = timeline.nodes.findIndex((node) => node.id === timepointId);
    if (index >= 0) {
      return {
        timeline,
        tp: timeline.nodes[index],
        index,
      };
    }
  }

  return null;
}

function selectedTimepointInfo() {
  return findTimepointById(app.selectedExecutedId);
}

function buildEditedSnapshot(tp) {
  const base = deepClone(tp.snapshot || {});
  const edits = app.timepointEdits.get(tp.id) || {};
  Object.entries(edits).forEach(([key, value]) => {
    base[key] = value;
  });
  return base;
}

function saveEditsForTimepoint(timepointId, options = {}) {
  const { notify = true, rerender = true } = options;
  const info = findTimepointById(timepointId);
  if (!info) return false;

  const rows = ui.tpVars.querySelectorAll("input[data-var-name]");
  if (!rows.length) return false;

  const base = info.tp.snapshot || {};
  const nextEdits = {};

  rows.forEach((input) => {
    const key = input.dataset.varName;
    const parsed = parseEditorValue(input.value);
    if (!valuesEqual(parsed, base[key])) {
      nextEdits[key] = parsed;
    }
  });

  if (Object.keys(nextEdits).length > 0) {
    app.timepointEdits.set(timepointId, nextEdits);
  } else {
    app.timepointEdits.delete(timepointId);
  }

  if (notify) {
    setStatus("Timepoint edits saved", "ok");
  }

  if (rerender) {
    renderAll();
  }

  return true;
}

function createRunner(code) {
  return new Function(
    "__ctx",
    `
      return (async function () {
        const state = __ctx.state;
        const console = __ctx.console;
        with (state) {
          ${code}
        }
      })();
    `,
  );
}

function transformLine(line) {
  const varDecl = line.match(/^(\s*)(let|const|var)\s+([A-Za-z_$][\w$]*)\s*(=.*)?;?\s*$/);
  if (varDecl) {
    const indent = varDecl[1] || "";
    const name = varDecl[3];
    const assignExpr = varDecl[4];
    if (assignExpr) return `${indent}state.${name} ${assignExpr};`;
    return `${indent}state.${name} = undefined;`;
  }

  const fnDecl = line.match(/^(\s*)function\s+([A-Za-z_$][\w$]*)\s*\(/);
  if (fnDecl) {
    const indent = fnDecl[1] || "";
    const name = fnDecl[2];
    return line.replace(/^(\s*)function\s+([A-Za-z_$][\w$]*)\s*\(/, `${indent}state.${name} = function ${name}(`);
  }

  return line;
}

function buildInstrumentedCode(source) {
  const lines = source.replace(/\r\n/g, "\n").split("\n");
  const chunks = [];

  lines.forEach((line, idx) => {
    const ln = idx + 1;
    const transformed = transformLine(line);
    if (line.trim() === "") {
      chunks.push("");
      return;
    }

    const trimmed = line.trim();
    const skipHook =
      /^}?\s*(else|catch|finally)\b/.test(trimmed) ||
      /^(case\b|default\b)/.test(trimmed) ||
      /^}\s*(while\b)/.test(trimmed);

    if (!skipHook) {
      chunks.push(`__ctx.__line(${ln});`);
    }
    chunks.push(transformed);
  });

  return {
    lines,
    code: chunks.join("\n"),
  };
}

function shouldHitTimepoint(line) {
  if (app.mode === "implicit") return true;
  const tp = app.explicitTimepoints.get(line);
  return Boolean(tp && tp.enabled !== false);
}

function registerHit(line, elapsedMs, globalHitIndex = null) {
  const explicit = app.explicitTimepoints.get(line);
  const name = explicit?.name || `Implicit-L${line}`;
  const snapshot = deepClone(app.runtimeState);
  const id = `run${app.runCounter}-tp${app.executedTimepoints.length + 1}`;

  const entry = {
    id,
    run: app.runCounter,
    timelineId: app.activeTimelineId,
    line,
    time: elapsedMs,
    mode: explicit ? "explicit" : "implicit",
    name,
    globalHitIndex: globalHitIndex ?? app.executedTimepoints.length,
    snapshot,
  };

  app.executedTimepoints.push(entry);
  app.selectedExecutedId = entry.id;
}

function pushOutput(kind, parts, elapsedMs, line) {
  const text = parts.map((p) => (typeof p === "string" ? p : shortValue(p))).join(" ");
  app.outputEntries.push({
    id: `${kind}-${app.outputEntries.length + 1}`,
    timelineId: app.activeTimelineId,
    kind,
    text,
    line,
    time: elapsedMs,
  });
}

function visibleState() {
  const selectedInfo = selectedTimepointInfo();
  if (selectedInfo) {
    return buildEditedSnapshot(selectedInfo.tp);
  }
  return app.runtimeState || {};
}

function renderGutter() {
  const total = lineCount();
  const frag = document.createDocumentFragment();

  for (let line = 1; line <= total; line += 1) {
    const btn = document.createElement("button");
    btn.type = "button";
    btn.className = "line-btn";
    btn.textContent = String(line);

    if (app.explicitTimepoints.has(line)) btn.classList.add("tp-explicit");
    if (line === app.currentLine) btn.classList.add("current");

    btn.addEventListener("click", () => {
      if (app.running || app.playbackRunning) return;
      if (app.explicitTimepoints.has(line)) {
        app.explicitTimepoints.delete(line);
      } else {
        const suggested = defaultTpName(line);
        const userName = window.prompt(`Timepoint name at line ${line}:`, suggested);
        if (userName === null) return;
        app.explicitTimepoints.set(line, {
          line,
          enabled: true,
          name: (userName || suggested).trim() || suggested,
        });
      }
      renderAll();
    });

    frag.appendChild(btn);
  }

  ui.gutter.replaceChildren(frag);
  ui.gutter.scrollTop = ui.editor.scrollTop;
}

function renderOutput() {
  const source = app.playbackOutputEntries || app.outputEntries;
  if (!source.length) {
    ui.outputBox.innerHTML = '<p class="empty">No output yet.</p>';
    return;
  }

  const frag = document.createDocumentFragment();
  source.forEach((entry) => {
    const row = document.createElement("div");
    row.className = `output-line ${entry.kind === "error" ? "error" : ""}`;

    const ts = document.createElement("span");
    ts.className = "ts";
    ts.textContent = `${Math.round(entry.time)}ms L${entry.line}`;

    const text = document.createElement("span");
    text.textContent = entry.text;

    row.append(ts, text);
    frag.appendChild(row);
  });

  ui.outputBox.replaceChildren(frag);
  ui.outputBox.scrollTop = ui.outputBox.scrollHeight;
}

function renderWatchList() {
  const state = visibleState();
  const frag = document.createDocumentFragment();

  app.watchVars.forEach((name) => {
    const item = document.createElement("li");
    item.className = "watch-item";

    const left = document.createElement("span");
    left.className = "watch-name";
    left.textContent = name;

    const value = document.createElement("span");
    value.className = "watch-val";
    value.textContent = shortValue(state[name]);

    const remove = document.createElement("button");
    remove.className = "btn";
    remove.type = "button";
    remove.textContent = "x";
    remove.addEventListener("click", () => {
      app.watchVars = app.watchVars.filter((v) => v !== name);
      renderWatchList();
    });

    item.append(left, value, remove);
    frag.appendChild(item);
  });

  if (!app.watchVars.length) {
    const empty = document.createElement("li");
    empty.className = "watch-item";
    empty.textContent = "No watched variables.";
    frag.appendChild(empty);
  }

  ui.watchList.replaceChildren(frag);
}

function renderTimepointVars() {
  const selectedInfo = selectedTimepointInfo();
  if (!selectedInfo) {
    ui.selectedTpLabel.textContent = "Select a timepoint in the timeline.";
    ui.tpVars.textContent = "";
    delete ui.tpVars.dataset.tpId;
    return;
  }

  const { timeline, tp } = selectedInfo;
  const baseSnapshot = tp.snapshot || {};
  const editedSnapshot = buildEditedSnapshot(tp);

  ui.selectedTpLabel.textContent = `${tp.name} · line ${tp.line} · ${Math.round(tp.time)}ms · ${timeline.label}`;
  ui.tpVars.innerHTML = "";
  ui.tpVars.dataset.tpId = tp.id;

  const entries = Object.entries(baseSnapshot);
  if (!entries.length) {
    ui.tpVars.textContent = "Empty snapshot.";
    return;
  }

  const grid = document.createElement("div");
  grid.className = "tp-vars-grid";

  entries
    .sort(([a], [b]) => a.localeCompare(b))
    .forEach(([key]) => {
      const row = document.createElement("div");
      row.className = "tp-var-row";

      const keyEl = document.createElement("span");
      keyEl.className = "tp-var-key";
      keyEl.textContent = key;

      const input = document.createElement("input");
      input.className = "tp-var-input";
      input.type = "text";
      input.dataset.varName = key;
      input.value = toEditorValue(editedSnapshot[key]);

      row.append(keyEl, input);
      grid.appendChild(row);
    });

  const actions = document.createElement("div");
  actions.className = "tp-var-actions";

  const saveBtn = document.createElement("button");
  saveBtn.type = "button";
  saveBtn.className = "btn";
  saveBtn.textContent = "Save edits";
  saveBtn.addEventListener("click", () => {
    saveEditsForTimepoint(tp.id, { notify: true, rerender: true });
  });

  const resetBtn = document.createElement("button");
  resetBtn.type = "button";
  resetBtn.className = "btn";
  resetBtn.textContent = "Reset edits";
  resetBtn.addEventListener("click", () => {
    app.timepointEdits.delete(tp.id);
    setStatus("Timepoint edits reset", "idle");
    renderAll();
  });

  actions.append(saveBtn, resetBtn);

  const note = document.createElement("p");
  note.className = "muted tiny";
  note.textContent = "Tip: numbers/objects can be entered as JSON. Plain text is treated as string.";

  ui.tpVars.append(grid, actions, note);
}

function renderTimeline() {
  ui.timeline.innerHTML = "";

  const nonEmptyTimelines = app.timelines.filter((timeline) => timeline.nodes.length > 0);
  if (!nonEmptyTimelines.length) {
    ui.timeline.innerHTML = '<p class="empty">No execution yet. Create timepoints and click Run.</p>';
    return;
  }

  const frag = document.createDocumentFragment();

  nonEmptyTimelines.forEach((timeline) => {
    const row = document.createElement("div");
    row.className = `timeline-row ${timeline.id === app.activeTimelineId ? "active" : ""}`;

    const title = document.createElement("div");
    title.className = "timeline-title";
    title.textContent = timeline.id === app.activeTimelineId ? `${timeline.label} (active)` : timeline.label;

    const track = document.createElement("div");
    track.className = `timeline-track ${timeline.parentNodeIndex >= 0 ? "branch" : ""}`;
    if (timeline.parentNodeIndex >= 0) {
      track.style.marginLeft = `${timeline.parentNodeIndex * 58}px`;
    }

    timeline.nodes.forEach((tp, idx) => {
      const node = document.createElement("button");
      node.type = "button";
      node.className = `timeline-node ${tp.mode === "explicit" ? "explicit" : ""} ${tp.id === app.selectedExecutedId ? "selected" : ""}`;
      node.textContent = String(idx + 1);
      node.title = `${tp.name} (L${tp.line})`;

      const time = document.createElement("span");
      time.className = "timeline-time";
      time.textContent = `${Math.round(tp.time)}ms`;

      const label = document.createElement("span");
      label.className = "timeline-label";
      label.textContent = `${tp.name} · L${tp.line}`;

      node.append(time, label);
      node.addEventListener("click", () => {
        setActiveTimeline(timeline.id);
        app.selectedExecutedId = tp.id;
        app.currentLine = tp.line;
        app.playbackOutputEntries = null;
        renderAll();
      });

      track.appendChild(node);
    });

    row.append(title, track);
    frag.appendChild(row);
  });

  ui.timeline.appendChild(frag);
}

function renderAll() {
  renderGutter();
  renderOutput();
  renderWatchList();
  renderTimepointVars();
  renderTimeline();
  updateCursorPos();
  ui.stepDelayLabel.textContent = `${app.delayMs} ms`;
}

async function executeCode() {
  if (app.running || app.playbackRunning) return;

  app.running = true;
  app.stopRequested = false;
  app.playbackOutputEntries = null;
  app.selectedExecutedId = null;
  app.runtimeState = {};
  app.currentLine = 1;
  app.runCounter += 1;
  app.runStart = performance.now();
  app.timepointEdits.clear();

  const rootTimeline = createTimeline(`Run ${app.runCounter}`);
  app.timelines = [rootTimeline];
  setActiveTimeline(rootTimeline.id);

  setStatus("Running...", "idle");
  renderAll();

  const { code } = buildInstrumentedCode(ui.editor.value);
  const run = createRunner(code);

  const ctx = {
    state: app.runtimeState,
    console: {
      log: (...parts) => pushOutput("log", parts, performance.now() - app.runStart, app.currentLine),
      warn: (...parts) => pushOutput("warn", parts, performance.now() - app.runStart, app.currentLine),
      error: (...parts) => pushOutput("error", parts, performance.now() - app.runStart, app.currentLine),
    },
    __line: (line) => {
      if (app.stopRequested) {
        const stopErr = new Error("STOPPED");
        stopErr.__stop = true;
        throw stopErr;
      }

      app.currentLine = line;
      const elapsed = performance.now() - app.runStart;
      if (shouldHitTimepoint(line)) {
        registerHit(line, elapsed);
      }

      renderAll();
    },
  };

  try {
    await run(ctx);
    setStatus("Execution finished", "ok");
  } catch (err) {
    if (err && err.__stop) {
      pushOutput("warn", ["Execution stopped by user"], performance.now() - app.runStart, app.currentLine);
      setStatus("Stopped", "error");
    } else {
      pushOutput("error", [err?.message || String(err)], performance.now() - app.runStart, app.currentLine);
      setStatus("Execution error", "error");
    }
  } finally {
    app.running = false;
    renderAll();
  }
}

async function runBranchedExecution(selectedInfo, editedSnapshot) {
  app.running = true;
  app.stopRequested = false;
  app.playbackOutputEntries = null;
  app.runCounter += 1;
  app.runtimeState = {};
  app.currentLine = selectedInfo.tp.line;
  app.runStart = performance.now();

  const branchLabel = `Branch ${app.timelineCounter + 1} from ${selectedInfo.tp.name}`;
  const branchTimeline = createTimeline(
    branchLabel,
    selectedInfo.timeline.id,
    selectedInfo.tp.id,
    selectedInfo.index,
  );

  app.timelines.push(branchTimeline);
  setActiveTimeline(branchTimeline.id);
  app.selectedExecutedId = null;

  setStatus("Running branched execution...", "idle");
  renderAll();

  const { code } = buildInstrumentedCode(ui.editor.value);
  const run = createRunner(code);

  let seenHits = -1;
  let branchStarted = false;
  let branchStartClock = 0;

  const ctx = {
    state: app.runtimeState,
    console: {
      log: (...parts) => {
        if (!branchStarted) return;
        pushOutput("log", parts, performance.now() - branchStartClock, app.currentLine);
      },
      warn: (...parts) => {
        if (!branchStarted) return;
        pushOutput("warn", parts, performance.now() - branchStartClock, app.currentLine);
      },
      error: (...parts) => {
        if (!branchStarted) return;
        pushOutput("error", parts, performance.now() - branchStartClock, app.currentLine);
      },
    },
    __line: (line) => {
      if (app.stopRequested) {
        const stopErr = new Error("STOPPED");
        stopErr.__stop = true;
        throw stopErr;
      }

      app.currentLine = line;

      if (!shouldHitTimepoint(line)) {
        if (branchStarted) {
          renderAll();
        }
        return;
      }

      seenHits += 1;
      const targetGlobalIndex =
        selectedInfo.tp.globalHitIndex !== undefined ? selectedInfo.tp.globalHitIndex : selectedInfo.index;
      const reachedSelectedHit = seenHits === targetGlobalIndex && line === selectedInfo.tp.line;

      if (!branchStarted && reachedSelectedHit) {
        clearObject(app.runtimeState);
        Object.assign(app.runtimeState, deepClone(editedSnapshot));
        branchStarted = true;
        branchStartClock = performance.now();

        const explicit = app.explicitTimepoints.get(line);
        const id = `run${app.runCounter}-tp${app.executedTimepoints.length + 1}`;
        const entry = {
          id,
          run: app.runCounter,
          timelineId: app.activeTimelineId,
          line,
          time: 0,
          mode: explicit ? "explicit" : "implicit",
          name: `${selectedInfo.tp.name} (edited)`,
          globalHitIndex: seenHits,
          snapshot: deepClone(app.runtimeState),
        };

        app.executedTimepoints.push(entry);
        app.selectedExecutedId = entry.id;
        renderAll();
        return;
      }

      if (branchStarted) {
        registerHit(line, performance.now() - branchStartClock, seenHits);
        renderAll();
      }
    },
  };

  try {
    await run(ctx);

    if (!branchStarted) {
      app.timelines = app.timelines.filter((timeline) => timeline.id !== branchTimeline.id);
      setActiveTimeline(selectedInfo.timeline.id);
      app.selectedExecutedId = selectedInfo.tp.id;
      setStatus("Could not reach selected timepoint for branch", "error");
      return;
    }

    setStatus("Branched execution finished", "ok");
  } catch (err) {
    if (err && err.__stop) {
      if (!branchStarted) {
        app.timelines = app.timelines.filter((timeline) => timeline.id !== branchTimeline.id);
        setActiveTimeline(selectedInfo.timeline.id);
        app.selectedExecutedId = selectedInfo.tp.id;
      } else {
        pushOutput("warn", ["Branched execution stopped by user"], performance.now() - branchStartClock, app.currentLine);
      }
      setStatus("Stopped", "error");
    } else {
      if (branchStarted) {
        pushOutput("error", [err?.message || String(err)], performance.now() - branchStartClock, app.currentLine);
      }
      setStatus("Execution error", "error");
    }
  } finally {
    app.running = false;
    renderAll();
  }
}

async function resumeFromSelectedTimepoint() {
  if (app.running || app.playbackRunning) return;

  const selectedInfo = selectedTimepointInfo();
  if (!selectedInfo) {
    setStatus("Select a timepoint in the timeline", "error");
    return;
  }

  saveEditsForTimepoint(selectedInfo.tp.id, { notify: false, rerender: false });
  const editedSnapshot = buildEditedSnapshot(selectedInfo.tp);
  const hasEditedValues = !valuesEqual(editedSnapshot, selectedInfo.tp.snapshot || {});

  if (hasEditedValues) {
    await runBranchedExecution(selectedInfo, editedSnapshot);
    return;
  }

  setActiveTimeline(selectedInfo.timeline.id);
  app.stopRequested = false;

  const startIdx = app.executedTimepoints.findIndex((tp) => tp.id === selectedInfo.tp.id);
  if (startIdx < 0) {
    setStatus("Select a timepoint in the timeline", "error");
    return;
  }

  app.playbackRunning = true;
  app.playbackOutputEntries = [];
  const startTime = app.executedTimepoints[startIdx].time;
  setStatus("Resuming from selected timepoint...", "idle");

  for (let i = startIdx; i < app.executedTimepoints.length; i += 1) {
    if (app.stopRequested) break;

    const tp = app.executedTimepoints[i];
    app.selectedExecutedId = tp.id;
    app.currentLine = tp.line;
    app.playbackOutputEntries = app.outputEntries.filter((entry) => entry.time >= startTime && entry.time <= tp.time);
    renderAll();

    await wait(Math.max(40, app.delayMs));
  }

  app.playbackRunning = false;
  if (app.stopRequested) {
    setStatus("Resume stopped", "error");
  } else {
    setStatus("Resume finished", "ok");
  }
}

function stopExecution() {
  if (!app.running && !app.playbackRunning) return;
  app.stopRequested = true;
  setStatus("Stopping...", "error");
}

function clearRun() {
  if (app.running || app.playbackRunning) return;

  app.timelines = [];
  app.executedTimepoints = [];
  app.outputEntries = [];
  app.activeTimelineId = null;
  app.playbackOutputEntries = null;
  app.selectedExecutedId = null;
  app.currentLine = 1;
  app.stopRequested = false;
  app.timepointEdits.clear();

  setStatus("Ready", "idle");
  renderAll();
}

function addWatchVariable() {
  const name = (ui.watchInput.value || "").trim();
  if (!name) return;
  if (!app.watchVars.includes(name)) {
    app.watchVars.push(name);
  }
  ui.watchInput.value = "";
  renderWatchList();
}

ui.editor.addEventListener("input", () => {
  if (app.running) return;
  app.currentLine = 1;
  renderAll();
});

ui.editor.addEventListener("scroll", () => {
  ui.gutter.scrollTop = ui.editor.scrollTop;
});

ui.editor.addEventListener("keyup", updateCursorPos);
ui.editor.addEventListener("click", updateCursorPos);

ui.tpMode.addEventListener("change", () => {
  app.mode = ui.tpMode.value;
  renderAll();
});

ui.stepDelay.addEventListener("input", () => {
  app.delayMs = Number(ui.stepDelay.value);
  ui.stepDelayLabel.textContent = `${app.delayMs} ms`;
});

ui.addWatchBtn.addEventListener("click", addWatchVariable);
ui.watchInput.addEventListener("keydown", (event) => {
  if (event.key === "Enter") addWatchVariable();
});

ui.runBtn.addEventListener("click", executeCode);
ui.stopBtn.addEventListener("click", stopExecution);
ui.resumeBtn.addEventListener("click", resumeFromSelectedTimepoint);
ui.clearRunBtn.addEventListener("click", clearRun);

renderAll();
