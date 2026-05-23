const TEL_AVIV_LAT = 32.0853;
const TEL_AVIV_LON = 34.7818;
const START_ALT = 400;

const planeData = {
  cessna:   { icon: 'ðŸ›©ï¸', name: 'Cessna 172 Skyhawk', desc: 'Stable, reliable single-engine aircraft. Perfect for learning.' },
  piper:    { icon: 'âœˆï¸', name: 'Piper PA-28 Cherokee', desc: 'Nimble general-aviation workhorse.' },
  cirrus:   { icon: 'âœˆï¸', name: 'Cirrus SR22', desc: 'High-performance with CAPS emergency parachute.' },
  bonanza:  { icon: 'âœˆï¸', name: 'Beechcraft Bonanza', desc: 'Comfortable and capable mid-range cruiser.' },
  baron:    { icon: 'ðŸ›©ï¸', name: 'Beechcraft Baron', desc: 'Twin-engine power for serious missions.' },
  fighter:  { icon: 'ðŸŽ–ï¸', name: 'F-16 Fighting Falcon', desc: 'Supersonic military fighter. Extreme performance.' },
};

let isPaused = false, lastState = null, windEnabled = false, simStarted = false, selectedPlane = 'cessna';
let map = null, planeMarker = null, sse = null, sessionID = null;
let waypoints = [];  // Track waypoints
let planeCrashed = false;  // Track if plane has crashed

// Initialize session ID from localStorage (persists across page reloads for same browser)
function initSessionID() {
  sessionID = localStorage.getItem('flight_simulator_session_id');
  if (!sessionID) {
    sessionID = 'session_' + Math.random().toString(36).substr(2, 9);
    localStorage.setItem('flight_simulator_session_id', sessionID);
  }
}

function updatePlanePreview() {
  const sel = document.getElementById('plane-select');
  selectedPlane = sel.value;
  const p = planeData[selectedPlane];
  document.getElementById('plane-icon-preview').textContent = p.icon;
  document.getElementById('plane-name').textContent = p.name;
  document.getElementById('plane-desc').textContent = p.desc;
}

function startSimulation() {
  initSessionID();
  document.getElementById('menu-screen').classList.add('hidden');
  document.getElementById('app').classList.remove('hidden');
  simStarted = true;
  waypoints = [];  // Initialize waypoints
  planeCrashed = false;  // Reset crash state
  updateWaypointsList();  // Clear waypoints display
  initMap();
  connectSSE();
  loadInitialState();
}

async function loadInitialState() {
  try {
    let url = '/state';
    const params = [];
    if (sessionID) params.push(`sid=${encodeURIComponent(sessionID)}`);
    if (selectedPlane) params.push(`planeType=${encodeURIComponent(selectedPlane)}`);
    if (params.length) url += '?' + params.join('&');
    
    const r = await fetch(url);
    if (r.ok) {
      const st = await r.json();
      lastState = st;
      updateStats(st);
      updateMap(st);
      updateModeBadge(st);
    }
  } catch (_) {
    // Silently fail; SSE will update state when it connects
  }
}

function initMap() {
  // Destroy old map if it exists
  if (map) {
    map.remove();
    map = null;
    planeMarker = null;
  }
  map = L.map('map', { zoomControl: true, attributionControl: true });
  L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
    attribution: 'Â© OpenStreetMap',
    maxZoom: 18,
  }).addTo(map);
  map.setView([TEL_AVIV_LAT, TEL_AVIV_LON], 11);
  let planeIcon = L.divIcon({
    html: '<span class="plane-icon" id="plane-icon-el">âœˆ</span>',
    className: '',
    iconSize:   [28, 28],
    iconAnchor: [14, 14],
  });
  planeMarker = L.marker([TEL_AVIV_LAT, TEL_AVIV_LON], { icon: planeIcon, zIndexOffset: 1000 }).addTo(map);
}

function rotatePlane(headingDeg) {
  const el = document.getElementById('plane-icon-el');
  if (el) el.style.transform = `rotate(${headingDeg - 90}deg)`;
}

function connectSSE() {
  if (sse) { sse.close(); }
  if (!simStarted || !sessionID) return;
  const params = [`sid=${encodeURIComponent(sessionID)}`];
  if (selectedPlane) params.push(`planeType=${encodeURIComponent(selectedPlane)}`);
  sse = new EventSource(`/stream?${params.join('&')}`);
  sse.onopen = () => { setStatus(true); log('SSE connected', 'trace'); };
  sse.onmessage = (e) => {
    try {
      const st = JSON.parse(e.data);
      lastState = st;
      updateStats(st);
      updateMap(st);
      updateModeBadge(st);
      // Check for crash condition: altitude 0 and all velocities 0
      if (!planeCrashed && st.alt <= 0 && st.vLat === 0 && st.vLon === 0 && st.vAlt === 0) {
        planeCrashed = true;
        log('CRASH: Aircraft altitude reached 0 - plane crashed!', 'error');
      }
      // Check if current waypoint is reached and remove it
      checkAndRemoveWaypoint(st);
    } catch (_) {}
  };
  // Handle log events from backend
  sse.addEventListener('log', (e) => {
    try {
      const logData = JSON.parse(e.data);
      log(logData.message, logData.level);
    } catch (_) {}
  });
  sse.onerror = () => {
    setStatus(false);
    log('SSE disconnected â€” reconnectingâ€¦', 'warn');
    setTimeout(connectSSE, 3000);
  };
}

function updateStats(st) {
  setText('s-lat',     fmt6(st.lat));
  setText('s-lon',     fmt6(st.lon));
  setText('s-alt',     fmt1(st.alt));
  setText('s-vlat',    fmtE(st.vLat));
  setText('s-vlon',    fmtE(st.vLon));
  setText('s-valt',    fmt1(st.vAlt));
  setText('s-heading', fmt1(st.heading));
  setText('s-seq',     st.seq);
  if (st.simTime) {
    const d = new Date(st.simTime);
    setText('s-simtime', d.toISOString().replace('T',' ').substring(0,19) + ' UTC');
  }
}

function updateMap(st) {
  if (!map || !planeMarker) return;
  const latlng = [st.lat, st.lon];
  planeMarker.setLatLng(latlng);
  map.setView(latlng, map.getZoom(), { animate: true, duration: 0.3 });
  rotatePlane(st.heading || 0);
}

function updateModeBadge(st) {}

async function refreshWindDisplay() {
  try {
    const wlat = parseFloat(document.getElementById('w-lat').value) || 0;
    const wlon = parseFloat(document.getElementById('w-lon').value) || 0;
    const walt = parseFloat(document.getElementById('w-alt').value) || 0;
    const on = document.getElementById('wind-toggle').checked;
    setText('s-wlat', on ? fmtE(wlat) : '0');
    setText('s-wlon', on ? fmtE(wlon) : '0');
    setText('s-walt', on ? fmt1(walt)  : '0');
  } catch(_) {}
}

async function post(path, body) {
  try {
    let url = path;
    const params = [];
    if (sessionID) params.push(`sid=${encodeURIComponent(sessionID)}`);
    if (selectedPlane) params.push(`planeType=${encodeURIComponent(selectedPlane)}`);
    if (params.length) url += '?' + params.join('&');
    
    const r = await fetch(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    const text = await r.text();
    let data;
    try { data = JSON.parse(text); } catch(_) { data = text; }
    if (r.ok) {
      log(`${path} â†’ ${JSON.stringify(data)}`, 'trace');
    } else {
      log(`${path} ERROR ${r.status}: ${text}`, 'error');
    }
    return data;
  } catch(e) {
    log(`${path} fetch error: ${e.message}`, 'error');
  }
}

async function togglePause() {
  isPaused = !isPaused;
  const btn = document.getElementById('pause-btn');
  if (isPaused) {
    await post('/sim/pause');
    btn.textContent = 'â¸ Paused';
    btn.className = 'btn paused';
  } else {
    await post('/sim/resume');
    btn.textContent = 'â–¶ Running';
    btn.className = 'btn success';
  }
}

async function applyHz() {
  const hz = parseInt(document.getElementById('hz-slider').value, 10);
  await post('/sim/hz', { hz });
}

async function applyWind() {
  const enabled = document.getElementById('wind-toggle').checked;
  const vLat = parseFloat(document.getElementById('w-lat').value) || 0;
  const vLon = parseFloat(document.getElementById('w-lon').value) || 0;
  const vAlt = parseFloat(document.getElementById('w-alt').value) || 0;
  await post('/wind', { enabled, vLat, vLon, vAlt });
  await refreshWindDisplay();
}

async function skip(by) {
  await post('/sim/skip', { by });
  showModeBadgeFF();
}

async function skipCustom() {
  const by = document.getElementById('skip-custom').value.trim();
  if (!by) { log('Enter a duration first (e.g. 30s)', 'warn'); return; }
  await post('/sim/skip', { by });
  showModeBadgeFF();
}

async function applyTrace() {
  const enabled = document.getElementById('trace-toggle').checked;
  if (enabled) {
    await post('/command/trace/start', {});
  }
}

function addWaypoint() {
  const latInput = document.getElementById('goto-lat').value.trim();
  const lonInput = document.getElementById('goto-lon').value.trim();
  const altInput = document.getElementById('goto-alt').value.trim();

  // Use current position as defaults if fields are empty
  const lat = latInput === '' ? (lastState?.lat ?? TEL_AVIV_LAT) : parseFloat(latInput);
  const lon = lonInput === '' ? (lastState?.lon ?? TEL_AVIV_LON) : parseFloat(lonInput);
  const alt = altInput === '' ? (lastState?.alt ?? START_ALT) : parseFloat(altInput);

  if (isNaN(lat) || isNaN(lon) || isNaN(alt)) {
    log('Invalid waypoint parameters', 'error');
    return;
  }

  // Add to waypoints list
  waypoints.push({ lat, lon, alt });
  
  // Clear inputs
  document.getElementById('goto-lat').value = '';
  document.getElementById('goto-lon').value = '';
  document.getElementById('goto-alt').value = '';
  
  // Update display
  updateWaypointsList();
  
  // Send trajectory command
  sendTrajectory();
  
  log(`Waypoint added: (${lat.toFixed(4)}, ${lon.toFixed(4)}) at ${alt.toFixed(1)}m. Total waypoints: ${waypoints.length}`, 'info');
}

function removeWaypoint(index) {
  waypoints.splice(index, 1);
  updateWaypointsList();
  sendTrajectory();
  log(`Waypoint removed. Remaining: ${waypoints.length}`, 'info');
}

function updateWaypointsList() {
  const listDiv = document.getElementById('waypoints-list');
  
  if (waypoints.length === 0) {
    listDiv.innerHTML = '<div style="padding:6px; color:#666; text-align:center">No waypoints</div>';
    return;
  }
  
  let html = '';
  waypoints.forEach((wp, idx) => {
    html += `
      <div style="padding:6px; border-bottom:1px solid #222; display:flex; justify-content:space-between; align-items:center">
        <span style="flex:1">${idx + 1}. (${wp.lat.toFixed(4)}, ${wp.lon.toFixed(4)}) ${wp.alt}m</span>
        <button style="background:#d32f2f; color:white; border:none; border-radius:2px; padding:2px 6px; cursor:pointer; font-size:10px" onclick="removeWaypoint(${idx})">Ã—</button>
      </div>
    `;
  });
  listDiv.innerHTML = html;
}

function checkAndRemoveWaypoint(st) {
  // Check if the first waypoint is reached (aircraft at waypoint within tolerance)
  if (waypoints.length === 0) return;
  
  const wp = waypoints[0];
  const arrivalToleranceDeg = 0.0005; // ~55 m (matches backend)
  const arrivalToleranceAlt = 1.0; // Same as backend
  
  // Calculate horizontal distance
  const dLat = st.lat - wp.lat;
  const dLon = st.lon - wp.lon;
  const horizDist = Math.sqrt(dLat * dLat + dLon * dLon);
  
  // Calculate vertical distance
  const dAlt = Math.abs(st.alt - wp.alt);
  
  // Check if aircraft is at waypoint and velocity is near zero (reached destination)
  if (horizDist < arrivalToleranceDeg && dAlt < arrivalToleranceAlt && 
      Math.abs(st.vLat) < 0.00001 && Math.abs(st.vLon) < 0.00001 && Math.abs(st.vAlt) < 0.1) {
    waypoints.shift(); // Remove first waypoint
    updateWaypointsList();
    if (waypoints.length > 0) {
      log(`Waypoint reached! Continuing to next waypoint (${waypoints.length} remaining).`, 'info');
    } else {
      log('All waypoints reached! Heading north.', 'info');
    }
  }
}

async function sendTrajectory() {
  if (waypoints.length === 0) {
    // No waypoints - plane defaults to heading north naturally
    log('No waypoints. Plane heading north by default.', 'info');
    return;
  }

  await post('/command/trajectory', { waypoints });
  log(`Trajectory sent with ${waypoints.length} waypoint(s)`, 'info');
}

async function sendStop() {
  if (sse) { sse.close(); }
  await post('/command/stop', {});
  // Clear waypoints
  waypoints = [];
  updateWaypointsList();
  // Clear logs
  document.getElementById('log-lines').innerHTML = '';
  // Reset state and UI
  lastState = null;
  // Destroy map
  if (map) {
    map.remove();
    map = null;
    planeMarker = null;
  }
  // Reset all displays to dashes
  setText('s-lat', 'â€”');
  setText('s-lon', 'â€”');
  setText('s-alt', 'â€”');
  setText('s-vlat', 'â€”');
  setText('s-vlon', 'â€”');
  setText('s-valt', 'â€”');
  setText('s-heading', 'â€”');
  setText('s-seq', 'â€”');
  setText('s-simtime', 'â€”');
  // Return to main menu
  document.getElementById('menu-screen').classList.remove('hidden');
  document.getElementById('app').classList.add('hidden');
  simStarted = false;
  log('Returned to main menu', 'info');
}

async function toggleInfoLogs() {
  const enabled = document.getElementById('info-toggle').checked;
  await post('/log/info', { enabled });
  log(`Info logs ${enabled ? 'enabled' : 'disabled'}`, 'info');
}

async function toggleTraceLogs() {
  const enabled = document.getElementById('trace-log-toggle').checked;
  await post('/log/trace', { enabled });
  log(`Trace logs ${enabled ? 'enabled' : 'disabled'}`, 'info');
}

function setStatus(connected) {
  const dot = document.getElementById('status-dot');
  const txt = document.getElementById('status-txt');
  dot.className = connected ? 'connected' : '';
  txt.textContent = connected ? 'live' : 'disconnected';
}

function showModeBadgeFF() {
  const b = document.getElementById('mode-badge');
  b.textContent = 'FAST-FWD';
  b.className = 'ff';
  setTimeout(() => {
    b.textContent = 'REAL-TIME';
    b.className = '';
  }, 3500);
}

function setText(id, val) {
  const el = document.getElementById(id);
  if (el) el.textContent = val;
}

function fmt6(v) { return typeof v === 'number' ? v.toFixed(6) : 'â€”'; }
function fmt1(v) { return typeof v === 'number' ? v.toFixed(1) : 'â€”'; }
function fmtE(v) {
  if (typeof v !== 'number') return 'â€”';
  return (v * 111320).toFixed(2);
}

const MAX_LOGS = 200;
function log(msg, level = 'info') {
  // Filter based on toggle states
  if (level === 'trace' && !document.getElementById('trace-log-toggle')?.checked) {
    return; // Don't display trace logs when trace is disabled
  }
  if (level === 'info' && !document.getElementById('info-toggle')?.checked) {
    return; // Don't display info logs when info is disabled
  }
  
  const lines = document.getElementById('log-lines');
  const d = document.createElement('div');
  d.className = `log-line ${level}`;
  const ts = new Date().toISOString().substring(11, 23);
  d.textContent = `[${ts}] ${msg}`;
  lines.appendChild(d);
  while (lines.children.length > MAX_LOGS) {
    lines.removeChild(lines.firstChild);
  }
  const panel = document.getElementById('log-panel');
  panel.scrollTop = panel.scrollHeight;
}
