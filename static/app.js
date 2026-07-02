'use strict';

let daysOffset  = 0;
let weekOffset  = 0;
let monthOffset = 0;

// Labels

function updateDaysLabel() {
  const el  = document.getElementById('days-offset-label');
  const fwd = document.getElementById('days-forward-btn');
  el.textContent = daysOffset === 0 ? 'Today ±2' : daysOffset + 'd back';
  fwd.disabled   = daysOffset <= 0;
}

function updateWeekLabel() {
  const el  = document.getElementById('week-offset-label');
  const fwd = document.getElementById('week-forward-btn');
  if      (weekOffset === 0) el.textContent = 'This week';
  else if (weekOffset === 1) el.textContent = 'Last week';
  else                       el.textContent = weekOffset + ' weeks ago';
  fwd.disabled = weekOffset <= 0;
}

function updateMonthLabel() {
  const el  = document.getElementById('month-offset-label');
  const fwd = document.getElementById('month-forward-btn');
  if      (monthOffset === 0) el.textContent = 'This month';
  else if (monthOffset === 1) el.textContent = 'Last month';
  else                        el.textContent = monthOffset + ' months ago';
  fwd.disabled = monthOffset <= 0;
}

// Tiles

async function fetchHTML(url, containerId) {
  const res  = await fetch(url);
  const html = await res.text();
  document.getElementById(containerId).innerHTML = html;
}

async function refreshAllTiles() {
  await Promise.all([
    fetchHTML('/tiles/days?offset='  + daysOffset,  'days-row'),
    fetchHTML('/tiles/week?offset='  + weekOffset,  'week-row'),
    fetchHTML('/tiles/month?offset=' + monthOffset, 'month-grid'),
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

async function shiftWeek(dir) {
  const next = weekOffset + dir;
  if (next < 0) return;
  weekOffset = next;
  updateWeekLabel();
  await fetchHTML('/tiles/week?offset=' + weekOffset, 'week-row');
  document.getElementById('week-label').textContent =
    await (await fetch('/label/week?offset=' + weekOffset)).text();
}

async function shiftMonth(dir) {
  const next = monthOffset + dir;
  if (next < 0) return;
  monthOffset = next;
  updateMonthLabel();
  await fetchHTML('/tiles/month?offset=' + monthOffset, 'month-grid');
  document.getElementById('month-label').textContent =
    await (await fetch('/label/month?offset=' + monthOffset)).text();
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
updateWeekLabel();
updateMonthLabel();
