(function (global) {
  'use strict';

  function wsURL(index) {
    var proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    return proto + '//' + location.host + '/doors/ws?n=' + index;
  }

  function connect(index) {
    var term = document.getElementById('door-term');
    if (!term) return;
    var ws = new WebSocket(wsURL(index));
    ws.binaryType = 'arraybuffer';

    ws.onmessage = function (ev) {
      var text = typeof ev.data === 'string' ? ev.data : decodeBuffer(ev.data);
      term.textContent += text;
      term.scrollTop = term.scrollHeight;
    };
    ws.onclose = function () {
      term.textContent += '\n[session ended]\n';
    };
    ws.onerror = function () {
      term.textContent += '\n[connection error]\n';
    };

    document.addEventListener('keydown', function (e) {
      if (!ws || ws.readyState !== WebSocket.OPEN) return;
      if (e.target && (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA' || e.target.tagName === 'SELECT')) return;
      var ch = keyToChar(e);
      if (ch !== null) {
        e.preventDefault();
        ws.send(ch);
      }
    });
  }

  function decodeBuffer(buf) {
    try {
      return new TextDecoder('utf-8').decode(buf);
    } catch (e) {
      return String.fromCharCode.apply(null, new Uint8Array(buf));
    }
  }

  function keyToChar(e) {
    if (e.key === 'Enter') return '\r';
    if (e.key === 'Backspace') return '\x7f';
    if (e.key === 'Tab') return '\t';
    if (e.key === 'Escape') return '\x1b';
    if (e.key.length === 1) return e.key;
    return null;
  }

  global.VirtBBSDoor = { connect: connect };
})(window);
