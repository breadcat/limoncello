'use strict';

let daysOffset  = 0;

// Month (scrollable weeks) view
const MONTH_BATCH = 6;               // must match monthBatchSize in main.go
let monthWeeksLoaded  = MONTH_BATCH; // weeks already present server-side on load
let monthScrollReady  = false;       // becomes true once the panel is first opened
let monthLoadingMore  = false;

// Labels

function updateDaysLabel() {
  const el  = document.getElementById('days-offset-label');
  const fwd = document.getElementById('days-forward-btn');
  el.textContent = daysOffset === 0 ? 'Today ±2' : daysOffset + 'd back';
  fwd.disabled   = daysOffset <= 0;
}

// Tiles

async function fetchHTML(url, containerId) {
  const res  = await fetch(url);
  const html = await res.text();
  document.getElementById(containerId).innerHTML = html;
}

async function refreshAllTiles() {
  await Promise.all([
    fetchHTML('/tiles/days?offset=' + daysOffset, 'days-row'),
    refreshMonthScroll(),
  ]);
}

// Navigation

async function shiftDays(dir) {
  const next = daysOffset + dir;
  if (next < 0) return;
  daysOffset = next;
  updateDaysLabel();
  await fetchHTML('/tiles/days?offset=' + daysOffset, 'days-row');
}

// Month (scrollable weeks)

// Called once, the first time the "Month view" <details> panel is opened.
// Waits until the panel is actually visible (so scrollHeight is meaningful),
// then jumps the scroll position to the bottom so the current week is the
// last row in view, and wires up infinite-scroll-upward loading.
function handleMonthDetailsToggle(details) {
  if (!details.open || monthScrollReady) return;
  monthScrollReady = true;
  const container = document.getElementById('month-scroll');
  if (!container) return;
  container.scrollTop = container.scrollHeight;
  container.addEventListener('scroll', onMonthScroll);
}

async function onMonthScroll(e) {
  const container = e.target;
  if (container.scrollTop > 60 || monthLoadingMore) return;
  monthLoadingMore = true;
  const prevHeight = container.scrollHeight;
  const html = await (await fetch(
    '/tiles/monthweeks?start=' + monthWeeksLoaded + '&count=' + MONTH_BATCH
  )).text();
  if (html.trim()) {
    container.insertAdjacentHTML('afterbegin', html);
    monthWeeksLoaded += MONTH_BATCH;
    // Keep the same rows in view instead of jumping after prepending.
    container.scrollTop = container.scrollHeight - prevHeight + container.scrollTop;
  }
  monthLoadingMore = false;
}

// Re-fetches the same number of weeks already loaded (e.g. after a drink is
// added/removed) and swaps them in, preserving the current scroll position.
async function refreshMonthScroll() {
  const container = document.getElementById('month-scroll');
  if (!container) return;
  const prevScrollTop = container.scrollTop;
  const html = await (await fetch(
    '/tiles/monthweeks?start=0&count=' + monthWeeksLoaded
  )).text();
  container.innerHTML = html;
  container.scrollTop = prevScrollTop;
}

// Days

function openDay(date) {
  fetch('/modal?date=' + date)
    .then(r => r.text())
    .then(html => {
      document.body.insertAdjacentHTML('beforeend', html);
      updatePreview();
      document.getElementById('drink-volume').addEventListener('change', updatePreview);
      document.getElementById('drink-abv').addEventListener('input', updatePreview);
    });
}

function closeModal(e) {
  if (e.target.id === 'day-modal') {
    document.getElementById('day-modal').remove();
  }
}

function refreshModal(date) {
  const modal = document.getElementById('day-modal');
  if (!modal) return;
  fetch('/modal?date=' + date)
    .then(r => r.text())
    .then(html => {
      modal.remove();
      document.body.insertAdjacentHTML('beforeend', html);
      updatePreview();
      document.getElementById('drink-volume').addEventListener('change', updatePreview);
      document.getElementById('drink-abv').addEventListener('input', updatePreview);
    });
}

function updatePreview() {
  const volEl = document.getElementById('drink-volume');
  const abvEl = document.getElementById('drink-abv');
  const pre   = document.getElementById('abv-preview');
  if (!volEl || !abvEl || !pre) return;
  const ml  = parseInt(volEl.value, 10);
  const abv = parseFloat(abvEl.value);
  if (ml > 0 && abv > 0) {
    const u = (ml * abv / 1000).toFixed(2);
    pre.textContent = `= ${u} unit${u === '1.00' ? '' : 's'}`;
  } else {
    pre.textContent = '';
  }
}

// Drinks

function post(url, params) {
  return fetch(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    body: new URLSearchParams(params).toString(),
  });
}

async function addDrink(date) {
  const ml  = document.getElementById('drink-volume').value;
  const abv = document.getElementById('drink-abv').value;
  if (!ml || !abv || parseFloat(abv) <= 0) return;
  await post('/drink/add', { date, volume_ml: ml, abv });
  refreshModal(date);
  refreshAllTiles();
}

async function removeDrink(date, key) {
  await post('/drink/remove', { date, key });
  refreshModal(date);
  refreshAllTiles();
}

async function adjustDrink(date, key, delta) {
  await post('/drink/adjust', { date, key, delta });
  refreshModal(date);
  refreshAllTiles();
}

// Init
updateDaysLabel();
