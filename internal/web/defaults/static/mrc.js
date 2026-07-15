(function () {
  'use strict';

  var cfg = window.VirtBBSMRC || {};
  var log = document.getElementById('mrc-log');
  var input = document.getElementById('mrc-input');
  var btnConnect = document.getElementById('mrc-connect');
  var btnDisconnect = document.getElementById('mrc-disconnect');
  var btnSend = document.getElementById('mrc-send');
  var handleEl = document.getElementById('mrc-handle');
  var roomEl = document.getElementById('mrc-room');
  var badge = document.getElementById('mrc-status-badge');
  var roomLabel = document.getElementById('mrc-room-label');
  var topicLabel = document.getElementById('mrc-topic-label');
  var nicksEl = document.getElementById('mrc-nicks');
  var labels = cfg.labels || {};
  var ws = null;
  var nicks = [];

  function setConnected(on) {
    if (handleEl) handleEl.disabled = on;
    if (roomEl) roomEl.disabled = on;
    if (btnConnect) btnConnect.disabled = on;
    if (btnDisconnect) btnDisconnect.disabled = !on;
    if (input) input.disabled = !on;
    if (btnSend) btnSend.disabled = !on;
    if (badge) {
      badge.textContent = on ? (labels.connected || 'Connected') : (labels.disconnected || 'Disconnected');
      badge.className = 'badge ' + (on ? 'text-bg-success' : 'text-bg-secondary');
    }
  }

  function appendLine(msg) {
    if (!log) return;
    var div = document.createElement('div');
    div.className = 'mrc-line ' + (msg.kind || '');
    var ts = document.createElement('span');
    ts.className = 'ts';
    ts.textContent = msg.ts || '';
    div.appendChild(ts);
    if (msg.from) {
      var who = document.createElement('span');
      who.className = 'who';
      who.textContent = '<' + msg.from + (msg.site ? '@' + msg.site : '') + '> ';
      div.appendChild(who);
    }
    var body = document.createElement('span');
    if (msg.html) {
      body.innerHTML = msg.html;
    } else {
      body.textContent = msg.body || msg.message || '';
    }
    div.appendChild(body);
    log.appendChild(div);
    log.scrollTop = log.scrollHeight;
  }

  function sendJSON(obj) {
    if (!ws || ws.readyState !== 1) return;
    ws.send(JSON.stringify(obj));
  }

  function connect() {
    if (ws) return;
    var handle = (handleEl && handleEl.value.trim()) || cfg.handle || '';
    var room = (roomEl && roomEl.value.trim()) || cfg.room || 'lobby';
    if (badge) {
      badge.textContent = labels.connecting || 'Connecting…';
      badge.className = 'badge text-bg-warning';
    }
    var proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    ws = new WebSocket(proto + '//' + location.host + '/mrc/ws');
    ws.binaryType = 'arraybuffer';
    ws.onopen = function () {
      sendJSON({ type: 'join', handle: handle, room: room });
      setConnected(true);
      if (input) input.focus();
    };
    ws.onmessage = function (ev) {
      var raw = typeof ev.data === 'string' ? ev.data : new TextDecoder().decode(ev.data);
      var msg;
      try {
        msg = JSON.parse(raw);
      } catch (e) {
        return;
      }
      if (msg.type === 'event') {
        appendLine(msg);
      } else if (msg.type === 'status') {
        if (roomLabel) roomLabel.textContent = '#' + (msg.room || '—');
        if (topicLabel) topicLabel.textContent = msg.topic || labels.topicNone || '';
        if (roomEl && msg.room) roomEl.value = msg.room;
      } else if (msg.type === 'nicks') {
        nicks = msg.names || [];
        if (nicksEl) {
          nicksEl.textContent = (labels.nicks || 'In room') + ': ' + (nicks.join(', ') || '—');
        }
      } else if (msg.type === 'error') {
        appendLine({ kind: 'notice', body: msg.message, ts: msg.ts });
      } else if (msg.type === 'quit') {
        disconnect();
      }
    };
    ws.onclose = function () {
      ws = null;
      setConnected(false);
      appendLine({ kind: 'notice', body: labels.disconnected || 'Disconnected', ts: '' });
    };
    ws.onerror = function () {
      appendLine({ kind: 'notice', body: 'WebSocket error', ts: '' });
    };
  }

  function disconnect() {
    if (!ws) return;
    try {
      sendJSON({ type: 'quit' });
      ws.close();
    } catch (e) {}
    ws = null;
    setConnected(false);
  }

  function sendChat() {
    if (!input) return;
    var text = input.value.trim();
    if (!text) return;
    sendJSON({ type: 'chat', text: text });
    input.value = '';
    input.focus();
  }

  if (btnConnect) btnConnect.addEventListener('click', connect);
  if (btnDisconnect) btnDisconnect.addEventListener('click', disconnect);
  if (btnSend) btnSend.addEventListener('click', sendChat);
  if (input) {
    input.addEventListener('keydown', function (e) {
      if (e.key === 'Enter') {
        e.preventDefault();
        sendChat();
      }
    });
  }
  setConnected(false);
})();
