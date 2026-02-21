package report

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Eval Report</title>
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; display: flex; height: 100vh; background: #f5f5f5; color: #1a1a1a; }

/* Sidebar */
.sidebar { width: 320px; min-width: 320px; background: #fff; border-right: 1px solid #ddd; display: flex; flex-direction: column; overflow: hidden; }
.sidebar-header { padding: 16px; border-bottom: 1px solid #ddd; }
.sidebar-header h1 { font-size: 16px; margin-bottom: 4px; }
.sidebar-header .meta { font-size: 12px; color: #666; }
.sidebar-header .summary { font-size: 13px; margin-top: 8px; }
.summary .pass-count { color: #16a34a; font-weight: 600; }
.summary .fail-count { color: #dc2626; font-weight: 600; }

.filter-bar { padding: 8px 16px; border-bottom: 1px solid #ddd; display: flex; gap: 8px; align-items: center; }
.filter-bar input { flex: 1; padding: 6px 8px; border: 1px solid #ddd; border-radius: 4px; font-size: 13px; outline: none; }
.filter-bar input:focus { border-color: #2563eb; }

.filter-buttons { display: flex; padding: 8px 16px; gap: 4px; border-bottom: 1px solid #ddd; }
.filter-btn { padding: 4px 10px; border: 1px solid #ddd; border-radius: 4px; background: #fff; cursor: pointer; font-size: 12px; }
.filter-btn.active { background: #2563eb; color: #fff; border-color: #2563eb; }

.eval-list { flex: 1; overflow-y: auto; }
.eval-item { padding: 10px 16px; border-bottom: 1px solid #eee; cursor: pointer; display: flex; align-items: center; gap: 8px; font-size: 13px; }
.eval-item:hover { background: #f0f0f0; }
.eval-item.selected { background: #e8f0fe; }
.eval-item.hidden { display: none; }
.badge { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
.badge.pass { background: #16a34a; }
.badge.fail { background: #dc2626; }
.eval-name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

/* Main content */
.main { flex: 1; overflow-y: auto; padding: 24px; }
.main.empty { display: flex; align-items: center; justify-content: center; color: #999; }

/* Eval header */
.eval-header { margin-bottom: 16px; display: flex; align-items: center; gap: 12px; }
.eval-header h2 { font-size: 18px; }
.eval-status { padding: 3px 10px; border-radius: 4px; font-size: 12px; font-weight: 600; }
.eval-status.pass { background: #dcfce7; color: #166534; }
.eval-status.fail { background: #fee2e2; color: #991b1b; }
.eval-message { margin-bottom: 16px; padding: 10px 14px; background: #fee2e2; border-radius: 6px; font-size: 13px; color: #991b1b; }

/* Tools panel */
.tools-panel { margin-bottom: 16px; }
.tools-panel summary { cursor: pointer; font-size: 13px; font-weight: 600; color: #666; padding: 8px 0; }
.tools-grid { display: flex; flex-direction: column; gap: 8px; padding: 8px 0; }
.tool-card { border: 1px solid #e5e7eb; border-radius: 6px; padding: 10px 14px; background: #fafafa; }
.tool-name { font-weight: 600; font-size: 13px; color: #92400e; }
.tool-desc { font-size: 12px; color: #666; margin-top: 2px; }
.tool-params { font-size: 11px; color: #888; margin-top: 4px; font-family: monospace; white-space: pre-wrap; max-height: 200px; overflow-y: auto; }

/* Message cards */
.message { margin-bottom: 12px; border-radius: 8px; padding: 12px 16px; border-left: 4px solid transparent; background: #fff; box-shadow: 0 1px 2px rgba(0,0,0,0.05); }
.message.system { border-left-color: #f59e0b; background: #fffbeb; }
.message.user { border-left-color: #2563eb; }
.message.assistant { border-left-color: #374151; }
.message.tool { border-left-color: #16a34a; background: #f0fdf4; }

.msg-role { font-size: 11px; font-weight: 700; text-transform: uppercase; letter-spacing: 0.05em; margin-bottom: 6px; color: #888; }
.msg-content { font-size: 14px; line-height: 1.6; white-space: pre-wrap; word-break: break-word; }

/* Reasoning */
.reasoning { margin-bottom: 8px; }
.reasoning summary { cursor: pointer; font-size: 12px; color: #999; font-style: italic; }
.reasoning-content { padding: 8px 12px; margin-top: 4px; background: #f9fafb; border-radius: 4px; font-size: 13px; line-height: 1.5; white-space: pre-wrap; color: #666; font-style: italic; max-height: 400px; overflow-y: auto; }

/* Tool calls */
.tool-call { margin-top: 8px; border: 1px solid #fde68a; border-radius: 6px; padding: 10px 14px; background: #fffbeb; }
.tc-header { display: flex; align-items: center; gap: 8px; margin-bottom: 4px; }
.tc-name { font-weight: 600; font-size: 13px; color: #92400e; }
.tc-id { font-size: 11px; color: #b45309; font-family: monospace; }
.tc-args { font-family: monospace; font-size: 12px; white-space: pre-wrap; color: #78350f; max-height: 300px; overflow-y: auto; }

/* Tool response */
.tool-call-id { font-size: 11px; color: #16a34a; font-family: monospace; margin-bottom: 4px; }
.tool-content { font-family: monospace; font-size: 13px; white-space: pre-wrap; word-break: break-word; }
</style>
</head>
<body>

<div class="sidebar">
  <div class="sidebar-header">
    <h1>Eval Report</h1>
    <div class="meta" id="meta"></div>
    <div class="summary" id="summary"></div>
  </div>
  <div class="filter-bar">
    <input type="text" id="filter-input" placeholder="Filter evals...">
  </div>
  <div class="filter-buttons">
    <button class="filter-btn active" data-filter="all">All</button>
    <button class="filter-btn" data-filter="passed">Passed</button>
    <button class="filter-btn" data-filter="failed">Failed</button>
  </div>
  <div class="eval-list" id="eval-list"></div>
</div>

<div class="main empty" id="main">
  <span>Select an eval to view its conversation</span>
</div>

<script>
const DATA = {{.DataJSON}};

function init() {
  if (!DATA) return;

  document.getElementById("meta").textContent = DATA.model + " \u2014 " + DATA.timestamp;
  const passedSpan = '<span class="pass-count">' + DATA.passed + ' passed</span>';
  const failedCount = DATA.total - DATA.passed;
  const failedSpan = failedCount > 0 ? ', <span class="fail-count">' + failedCount + ' failed</span>' : '';
  document.getElementById("summary").innerHTML = passedSpan + failedSpan + ' of ' + DATA.total + ' total';

  const list = document.getElementById("eval-list");
  DATA.evals.forEach(function(ev, i) {
    const item = document.createElement("div");
    item.className = "eval-item";
    item.dataset.index = i;
    item.dataset.passed = ev.passed;
    item.innerHTML = '<span class="badge ' + (ev.passed ? 'pass' : 'fail') + '"></span><span class="eval-name">' + escapeHtml(ev.name) + '</span>';
    item.addEventListener("click", function() { selectEval(i); });
    list.appendChild(item);
  });

  // Filter input
  document.getElementById("filter-input").addEventListener("input", applyFilters);

  // Filter buttons
  document.querySelectorAll(".filter-btn").forEach(function(btn) {
    btn.addEventListener("click", function() {
      document.querySelectorAll(".filter-btn").forEach(function(b) { b.classList.remove("active"); });
      btn.classList.add("active");
      applyFilters();
    });
  });

  // Select first eval
  if (DATA.evals.length > 0) selectEval(0);
}

function applyFilters() {
  var text = document.getElementById("filter-input").value.toLowerCase();
  var statusFilter = document.querySelector(".filter-btn.active").dataset.filter;
  document.querySelectorAll(".eval-item").forEach(function(item) {
    var name = DATA.evals[item.dataset.index].name.toLowerCase();
    var passed = item.dataset.passed === "true";
    var matchText = !text || name.indexOf(text) !== -1;
    var matchStatus = statusFilter === "all" || (statusFilter === "passed" && passed) || (statusFilter === "failed" && !passed);
    item.classList.toggle("hidden", !(matchText && matchStatus));
  });
}

function selectEval(index) {
  document.querySelectorAll(".eval-item").forEach(function(item) {
    item.classList.toggle("selected", parseInt(item.dataset.index) === index);
  });
  renderEval(DATA.evals[index]);
}

function renderEval(ev) {
  var main = document.getElementById("main");
  main.className = "main";
  var html = '';

  // Header
  html += '<div class="eval-header">';
  html += '<h2>' + escapeHtml(ev.name) + '</h2>';
  html += '<span class="eval-status ' + (ev.passed ? 'pass' : 'fail') + '">' + (ev.passed ? 'PASSED' : 'FAILED') + '</span>';
  html += '</div>';

  // Failure message
  if (!ev.passed && ev.message) {
    html += '<div class="eval-message">' + escapeHtml(ev.message) + '</div>';
  }

  // Tools
  if (ev.tools && ev.tools.length > 0) {
    html += '<details class="tools-panel"><summary>Tools (' + ev.tools.length + ')</summary><div class="tools-grid">';
    ev.tools.forEach(function(tool) {
      var fn = tool.function || {};
      html += '<div class="tool-card">';
      html += '<div class="tool-name">' + escapeHtml(fn.name || '') + '</div>';
      if (fn.description) html += '<div class="tool-desc">' + escapeHtml(fn.description) + '</div>';
      if (fn.parameters) html += '<div class="tool-params">' + escapeHtml(JSON.stringify(fn.parameters, null, 2)) + '</div>';
      html += '</div>';
    });
    html += '</div></details>';
  }

  // Messages
  if (ev.messages) {
    ev.messages.forEach(function(msg) {
      html += renderMessage(msg);
    });
  }

  main.innerHTML = html;
  main.scrollTop = 0;
}

function renderMessage(msg) {
  var role = msg.role || 'unknown';
  var html = '<div class="message ' + role + '">';
  html += '<div class="msg-role">' + escapeHtml(role);
  if (role === 'tool' && msg.tool_call_id) {
    html += ' <span style="font-weight:400;text-transform:none;letter-spacing:0">(call: ' + escapeHtml(msg.tool_call_id) + ')</span>';
  }
  html += '</div>';

  // Reasoning content
  if (msg.reasoning_content) {
    html += '<details class="reasoning"><summary>Reasoning</summary>';
    html += '<div class="reasoning-content">' + escapeHtml(msg.reasoning_content) + '</div>';
    html += '</details>';
  }

  // Content
  if (msg.content) {
    html += '<div class="msg-content">' + escapeHtml(msg.content) + '</div>';
  }

  // Tool calls
  if (msg.tool_calls && msg.tool_calls.length > 0) {
    msg.tool_calls.forEach(function(tc) {
      html += '<div class="tool-call">';
      html += '<div class="tc-header">';
      html += '<span class="tc-name">' + escapeHtml(tc.function ? tc.function.name : '') + '</span>';
      if (tc.id) html += '<span class="tc-id">' + escapeHtml(tc.id) + '</span>';
      html += '</div>';
      if (tc.function && tc.function.arguments) {
        var args = tc.function.arguments;
        try { args = JSON.stringify(JSON.parse(args), null, 2); } catch(e) {}
        html += '<div class="tc-args">' + escapeHtml(args) + '</div>';
      }
      html += '</div>';
    });
  }

  html += '</div>';
  return html;
}

function escapeHtml(s) {
  if (!s) return '';
  return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
}

init();
</script>
</body>
</html>`
