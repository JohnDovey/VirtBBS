(function () {
  'use strict';

  function initDebugPoll() {
    var modalEl = document.getElementById('debug-poll-modal');
    if (!modalEl || typeof bootstrap === 'undefined') {
      return;
    }

    var networkSelect = document.querySelector('select[name="network"]');
    var networkLabel = document.getElementById('debug-poll-network');
    var statusEl = document.getElementById('debug-poll-status');
    var outputEl = document.getElementById('debug-poll-output');
    var logPathEl = document.getElementById('debug-poll-log-path');
    var startBtn = document.getElementById('debug-poll-start');
    var closeBtn = document.getElementById('debug-poll-close');
    var activeSource = null;
    var running = false;

    var labels = {
      idle: modalEl.getAttribute('data-status-idle') || 'Ready',
      starting: modalEl.getAttribute('data-status-starting') || 'Starting…',
      polling: modalEl.getAttribute('data-status-polling') || 'Polling uplink…',
      complete: modalEl.getAttribute('data-status-complete') || 'Complete',
      failed: modalEl.getAttribute('data-status-failed') || 'Failed'
    };

    function setStatus(key) {
      if (statusEl) {
        statusEl.textContent = labels[key] || key;
        statusEl.className = 'badge rounded-pill debug-poll-status debug-poll-status-' + key;
      }
    }

    function appendLine(line) {
      if (!outputEl) {
        return;
      }
      outputEl.textContent += line + '\n';
      outputEl.scrollTop = outputEl.scrollHeight;
    }

    function resetModal() {
      if (outputEl) {
        outputEl.textContent = '';
      }
      if (logPathEl) {
        logPathEl.textContent = '';
      }
      setStatus('idle');
      if (startBtn) {
        startBtn.disabled = false;
      }
      if (closeBtn) {
        closeBtn.disabled = false;
      }
    }

    function stopStream() {
      if (activeSource) {
        activeSource.close();
        activeSource = null;
      }
      running = false;
      if (startBtn) {
        startBtn.disabled = false;
      }
      if (closeBtn) {
        closeBtn.disabled = false;
      }
    }

    function syncNetwork() {
      var network = networkSelect ? networkSelect.value : '';
      if (networkLabel) {
        networkLabel.textContent = network || '—';
      }
      return network;
    }

    modalEl.addEventListener('show.bs.modal', function () {
      stopStream();
      resetModal();
      syncNetwork();
    });

    modalEl.addEventListener('hidden.bs.modal', function () {
      stopStream();
    });

    if (!startBtn) {
      return;
    }

    startBtn.addEventListener('click', function () {
      if (running) {
        return;
      }
      var network = syncNetwork();
      if (!network) {
        setStatus('failed');
        appendLine('--- no network selected');
        return;
      }
      stopStream();
      if (outputEl) {
        outputEl.textContent = '';
      }
      if (logPathEl) {
        logPathEl.textContent = '';
      }
      running = true;
      startBtn.disabled = true;
      if (closeBtn) {
        closeBtn.disabled = true;
      }
      setStatus('starting');

      var url = '/admin/fido/debug_poll?network=' + encodeURIComponent(network);
      var es = new EventSource(url);
      activeSource = es;

      es.addEventListener('status', function (ev) {
        try {
          var phase = JSON.parse(ev.data);
          if (phase && labels[phase]) {
            setStatus(phase);
          }
        } catch (e) { /* ignore */ }
      });

      es.addEventListener('meta', function (ev) {
        try {
          var meta = JSON.parse(ev.data);
          if (logPathEl && meta.logPath) {
            logPathEl.textContent = meta.logPath;
          }
        } catch (e) { /* ignore */ }
      });

      es.addEventListener('line', function (ev) {
        try {
          appendLine(JSON.parse(ev.data));
        } catch (e) {
          appendLine(ev.data);
        }
      });

      es.addEventListener('done', function (ev) {
        try {
          var done = JSON.parse(ev.data);
          if (done.logPath && logPathEl) {
            logPathEl.textContent = done.logPath;
          }
          if (done.error) {
            appendLine('--- ' + done.error);
          } else {
            appendLine('--- sent: ' + done.sent + ', received: ' + done.received + ', tossed: ' + (done.tossed || 0));
          }
        } catch (e) { /* ignore */ }
        stopStream();
      });

      es.onerror = function () {
        if (running) {
          setStatus('failed');
          appendLine('--- connection lost');
        }
        stopStream();
      };
    });
  }

  if (document.readyState === 'complete') {
    initDebugPoll();
  } else {
    window.addEventListener('load', initDebugPoll);
  }
})();
