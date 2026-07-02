// app.js — vanilla client for the token management admin GUI.
// No framework, no build step, no bundler — served verbatim by the
// embedded file server (see internal/admin/static.go).

const SESSION_KEY = "observe-write-proxy:grafana-token";

// The Grafana token lives in memory for the life of the page; sessionStorage
// only mirrors it so a same-tab reload keeps it. Never localStorage, never a
// cookie, never echoed back into page markup.
let grafanaToken = sessionStorage.getItem(SESSION_KEY) || "";

function setGrafanaToken(token) {
  grafanaToken = token;
  if (token) {
    sessionStorage.setItem(SESSION_KEY, token);
  } else {
    sessionStorage.removeItem(SESSION_KEY);
  }
}

// clearGrafanaToken drops the held token and re-prompts the operator —
// used on a 401 so the UI never keeps acting as if it were still authorized.
function clearGrafanaToken() {
  setGrafanaToken("");
  const input = document.getElementById("grafana-token");
  if (input) {
    input.value = "";
    input.focus();
  }
  clearTokenTable();
}

// clearTokenTable empties the rendered token list and hides the new-token
// panel — called whenever the held Grafana token is no longer valid, so a
// 401 never leaves a stale table of plaintext write tokens on screen as if
// the operator were still authorized.
function clearTokenTable() {
  const tbody = document.querySelector("#tokens tbody");
  if (tbody) {
    tbody.textContent = "";
  }
  const panel = document.getElementById("new-token-panel");
  if (panel) {
    panel.hidden = true;
  }
}

function setStatus(message) {
  const el = document.getElementById("status");
  if (el) {
    el.textContent = message || "";
  }
}

// ApiError carries the HTTP status (or -1 for a network failure) so callers
// can branch (e.g. revert a checkbox) without re-parsing anything.
class ApiError extends Error {
  constructor(status, message) {
    super(message);
    this.status = status;
  }
}

// api() is the single fetch entry point for every /api/tokens call. It
// attaches the held Grafana token and centralizes failure handling so every
// caller re-prompts/reverts uniformly:
//   - 401 (missing/invalid Grafana token): clear the held token, show a
//     re-enter message, focus the token input, and reject. Callers must not
//     render stale data on this path.
//   - any other non-OK status (503 "grafana unreachable", other 5xx, or a
//     network error): show the failure in the status line but KEEP the held
//     token — Grafana being momentarily unreachable must not wipe a token
//     that is still valid.
// The branch is decided from res.status/res.ok before the body is ever
// touched, because the API mixes JSON and empty responses: only GET (array)
// and POST (201, created token) return JSON; PATCH returns 200 with an
// empty body, DELETE returns 204 with no body, and every error path is
// plain text (http.Error). Calling res.json() unconditionally would throw
// on the empty PATCH/DELETE responses and be misread as a failure — which
// for the toggle would wrongly trigger a revert on a change the server
// actually accepted. On success this returns the parsed JSON body, or null
// when the response carries none.
async function api(method, path, body) {
  const headers = { Authorization: `Bearer ${grafanaToken}` };
  if (body !== undefined) {
    headers["Content-Type"] = "application/json";
  }

  let res;
  try {
    res = await fetch(path, {
      method,
      headers,
      body: body !== undefined ? JSON.stringify(body) : undefined,
    });
  } catch (err) {
    setStatus("Network error reaching the proxy — check your connection and try again.");
    throw new ApiError(-1, "network error");
  }

  if (res.status === 401) {
    clearGrafanaToken();
    setStatus("Grafana token missing or invalid — please re-enter it.");
    throw new ApiError(401, "unauthorized");
  }

  if (!res.ok) {
    let detail = "";
    try {
      detail = await res.text();
    } catch (_) {
      // Fall back to the bare status if even the text body can't be read.
    }
    setStatus(`Request failed (${res.status})${detail ? `: ${detail}` : ""}`);
    throw new ApiError(res.status, detail || `status ${res.status}`);
  }

  if (res.status === 204) {
    return null;
  }
  const text = await res.text();
  if (!text) {
    return null;
  }
  return JSON.parse(text);
}

// formatCreatedAt renders the store's epoch-seconds created_at as a
// readable local timestamp (Date expects milliseconds, hence the *1000).
function formatCreatedAt(epochSeconds) {
  return new Date(epochSeconds * 1000).toLocaleString();
}

// renderTokens rebuilds the table body from scratch via DOM APIs
// (createElement/textContent, never string HTML interpolation) so an
// operator-supplied name can never inject markup.
function renderTokens(tokens) {
  const tbody = document.querySelector("#tokens tbody");
  tbody.textContent = "";

  for (const t of tokens) {
    const row = document.createElement("tr");

    const nameCell = document.createElement("td");
    nameCell.textContent = t.name;
    row.appendChild(nameCell);

    const tokenCell = document.createElement("td");
    tokenCell.className = "token-cell";
    const tokenText = document.createElement("span");
    tokenText.textContent = t.token;
    tokenCell.appendChild(tokenText);
    // navigator.clipboard is undefined on plain http:// non-localhost
    // origins (the shared-server deployment) — guard it and degrade to the
    // selectable cell instead of throwing. Never log the copied value.
    if (navigator.clipboard?.writeText) {
      const copyBtn = document.createElement("button");
      copyBtn.type = "button";
      copyBtn.textContent = "Copy";
      copyBtn.addEventListener("click", () => {
        navigator.clipboard.writeText(t.token).catch(() => {
          // Selectable cell text remains the fallback on failure.
        });
      });
      tokenCell.appendChild(document.createTextNode(" "));
      tokenCell.appendChild(copyBtn);
    }
    row.appendChild(tokenCell);

    const createdCell = document.createElement("td");
    createdCell.textContent = formatCreatedAt(t.created_at);
    row.appendChild(createdCell);

    const activeCell = document.createElement("td");
    const checkbox = document.createElement("input");
    checkbox.type = "checkbox";
    checkbox.checked = t.active;
    checkbox.addEventListener("change", () => onToggleActive(t.id, checkbox));
    activeCell.appendChild(checkbox);
    row.appendChild(activeCell);

    const actionsCell = document.createElement("td");
    const deleteBtn = document.createElement("button");
    deleteBtn.type = "button";
    deleteBtn.textContent = "Delete";
    deleteBtn.className = "danger";
    deleteBtn.addEventListener("click", () => onDelete(t.id));
    actionsCell.appendChild(deleteBtn);
    row.appendChild(actionsCell);

    tbody.appendChild(row);
  }
}

// loadTokens fetches and renders the current token list. A rehydrated but
// now-invalid sessionStorage token flows through api()'s own 401 handling
// (clear + re-prompt) rather than rendering an empty/broken grid.
async function loadTokens() {
  if (!grafanaToken) {
    return;
  }
  try {
    const tokens = await api("GET", "/api/tokens");
    renderTokens(tokens || []);
    setStatus("");
  } catch (err) {
    // api() already surfaced the message (and, on 401, cleared the token
    // and re-prompted) — nothing further to render.
  }
}

function showNewToken(token) {
  const panel = document.getElementById("new-token-panel");
  const value = document.getElementById("new-token-value");
  value.textContent = token;
  panel.hidden = false;
}

async function onCreate() {
  const nameInput = document.getElementById("new-name");
  const name = nameInput.value.trim();
  if (!name) {
    setStatus("Enter a name before creating a token.");
    return;
  }

  try {
    const created = await api("POST", "/api/tokens", { name });
    showNewToken(created.token);
    nameInput.value = "";
    setStatus("");
    await loadTokens();
  } catch (err) {
    // api() already surfaced the failure/re-prompt.
  }
}

// onToggleActive fires the moment the checkbox changes — there is no save
// step. On failure it reverts the checkbox to its prior state so the UI
// never claims an active change the server rejected.
async function onToggleActive(id, checkbox) {
  const previous = !checkbox.checked;
  const next = checkbox.checked;
  try {
    await api("PATCH", `/api/tokens/${id}`, { active: next });
    setStatus("");
    await loadTokens();
  } catch (err) {
    checkbox.checked = previous;
  }
}

async function onDelete(id) {
  if (!confirm("Delete this token permanently? This cannot be undone.")) {
    return;
  }
  try {
    await api("DELETE", `/api/tokens/${id}`);
    setStatus("");
    await loadTokens();
  } catch (err) {
    // api() already surfaced the failure/re-prompt.
  }
}

document.getElementById("token-bar").addEventListener("submit", (event) => {
  event.preventDefault();
  const input = document.getElementById("grafana-token");
  const value = input.value.trim();
  setGrafanaToken(value);
  if (value) {
    loadTokens();
  } else {
    clearTokenTable();
  }
});

document.getElementById("create-form").addEventListener("submit", (event) => {
  event.preventDefault();
  onCreate();
});

// Script is loaded with `defer`, so the DOM is already parsed here.
const tokenInput = document.getElementById("grafana-token");
if (tokenInput && grafanaToken) {
  tokenInput.value = grafanaToken;
}
loadTokens();
