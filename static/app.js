'use strict';

let daysOffset  = 0;

// Month (calendar) view
let monthOffset = 0; // 0 = current month, -1 = previous month, 1 = next month, ...

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
    loadMonth(),
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

// Month (calendar)

// Fetches the full calendar grid + label for the currently selected
// monthOffset and swaps it in, updating the Prev/Next button state.
async function loadMonth() {
  const res  = await fetch('/tiles/month?offset=' + monthOffset);
  const data = await res.json();
  document.getElementById('month-label').textContent = data.label;
  document.getElementById('month-grid').innerHTML     = data.grid;
  document.getElementById('month-next-btn').disabled  = data.next_disabled;
}

async function shiftMonth(dir) {
  monthOffset += dir;
  await loadMonth();
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
