(function (global) {
  'use strict';

  var FG = {
    '30': 'ansi-fg-black',
    '31': 'ansi-fg-red',
    '32': 'ansi-fg-green',
    '33': 'ansi-fg-yellow',
    '34': 'ansi-fg-blue',
    '35': 'ansi-fg-magenta',
    '36': 'ansi-fg-cyan',
    '37': 'ansi-fg-white',
    '90': 'ansi-fg-bright-black',
    '91': 'ansi-fg-bright-red',
    '92': 'ansi-fg-bright-green',
    '93': 'ansi-fg-bright-yellow',
    '94': 'ansi-fg-bright-blue',
    '95': 'ansi-fg-bright-magenta',
    '96': 'ansi-fg-bright-cyan',
    '97': 'ansi-fg-bright-white'
  };

  var BG = {
    '40': 'ansi-bg-black',
    '41': 'ansi-bg-red',
    '42': 'ansi-bg-green',
    '43': 'ansi-bg-yellow',
    '44': 'ansi-bg-blue',
    '45': 'ansi-bg-magenta',
    '46': 'ansi-bg-cyan',
    '47': 'ansi-bg-white'
  };

  function escapeHTML(s) {
    return s
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;');
  }

  // Convert a screen buffer (SGR codes only; clears already applied) to HTML.
  // Supports named colors and truecolor 38;2 / 48;2 sequences.
  function ansiToHTML(raw) {
    var s = raw.replace(/\r\n/g, '\n').replace(/\r/g, '\n');
    var bold = false;
    var fg = '';
    var bg = '';
    var fgRGB = '';
    var bgRGB = '';
    var out = '';

    function flush(text) {
      if (!text) return;
      var escaped = escapeHTML(text).replace(/\n/g, '<br>');
      var classes = [];
      if (bold) classes.push('ansi-bold');
      if (fg && !fgRGB) classes.push(fg);
      if (bg && !bgRGB) classes.push(bg);
      var styles = [];
      if (fgRGB) styles.push('color:rgb(' + fgRGB + ')');
      if (bgRGB) styles.push('background-color:rgb(' + bgRGB + ')');
      var attr = '';
      if (classes.length) attr += ' class="' + classes.join(' ') + '"';
      if (styles.length) attr += ' style="' + styles.join(';') + '"';
      if (attr) {
        out += '<span' + attr + '>' + escaped + '</span>';
      } else {
        out += escaped;
      }
    }

    var i = 0;
    while (i < s.length) {
      if (s.charCodeAt(i) === 0x1b && s.charAt(i + 1) === '[') {
        var j = i + 2;
        while (j < s.length && /[0-9;]/.test(s.charAt(j))) j++;
        if (j >= s.length || s.charAt(j) !== 'm') {
          i++;
          continue;
        }
        var code = s.slice(i + 2, j);
        i = j + 1;
        if (!code || code === '0') {
          bold = false;
          fg = '';
          bg = '';
          fgRGB = '';
          bgRGB = '';
          continue;
        }
        var parts = code.split(';');
        for (var p = 0; p < parts.length; p++) {
          var part = parts[p];
          if (part === '1') bold = true;
          else if (part === '22') bold = false;
          else if (part === '39') {
            fg = '';
            fgRGB = '';
          } else if (part === '49') {
            bg = '';
            bgRGB = '';
          } else if (part === '38' && parts[p + 1] === '2' && p + 4 < parts.length) {
            fgRGB = parts[p + 2] + ',' + parts[p + 3] + ',' + parts[p + 4];
            fg = '';
            p += 4;
          } else if (part === '48' && parts[p + 1] === '2' && p + 4 < parts.length) {
            bgRGB = parts[p + 2] + ',' + parts[p + 3] + ',' + parts[p + 4];
            bg = '';
            p += 4;
          } else if (FG[part]) {
            fg = FG[part];
            fgRGB = '';
          } else if (BG[part]) {
            bg = BG[part];
            bgRGB = '';
          }
        }
        continue;
      }
      var next = s.indexOf('\x1b', i);
      if (next < 0) {
        flush(s.slice(i));
        break;
      }
      flush(s.slice(i, next));
      i = next;
    }
    return out;
  }

  function Terminal(el) {
    this.el = el;
    this.screen = '';
    this.hold = '';
  }

  Terminal.prototype.feed = function (chunk) {
    var s = this.hold + chunk;
    this.hold = '';
    var i = 0;
    while (i < s.length) {
      if (s.charCodeAt(i) !== 0x1b) {
        var nextEsc = s.indexOf('\x1b', i);
        if (nextEsc < 0) {
          this.screen += s.slice(i);
          i = s.length;
          break;
        }
        this.screen += s.slice(i, nextEsc);
        i = nextEsc;
        continue;
      }
      // Incomplete escape at end of chunk — hold it.
      if (i === s.length - 1) {
        this.hold = '\x1b';
        break;
      }
      if (s.charAt(i + 1) !== '[') {
        i += 1;
        continue;
      }
      var j = i + 2;
      while (j < s.length && /[0-9;]/.test(s.charAt(j))) j++;
      if (j >= s.length) {
        this.hold = s.slice(i);
        break;
      }
      var final = s.charAt(j);
      if (final === 'm') {
        this.screen += s.slice(i, j + 1);
      } else if (final === 'J' || final === 'H') {
        // Clear / cursor home → start a fresh screen (door full redraws).
        this.screen = '';
      }
      i = j + 1;
    }
    this.el.innerHTML = ansiToHTML(this.screen);
    this.el.scrollTop = this.el.scrollHeight;
  };

  Terminal.prototype.appendPlain = function (text) {
    this.screen += text;
    this.el.innerHTML = ansiToHTML(this.screen);
    this.el.scrollTop = this.el.scrollHeight;
  };

  function wsURL(index) {
    var proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    return proto + '//' + location.host + '/doors/ws?n=' + index;
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
    if (e.key === 'ArrowUp') return '\x1b[A';
    if (e.key === 'ArrowDown') return '\x1b[B';
    if (e.key === 'ArrowRight') return '\x1b[C';
    if (e.key === 'ArrowLeft') return '\x1b[D';
    if (e.key.length === 1) return e.key;
    return null;
  }

  function connect(index) {
    var termEl = document.getElementById('door-term');
    if (!termEl) return;
    termEl.classList.add('ansi-screen', 'door-terminal');
    var term = new Terminal(termEl);
    var ws = new WebSocket(wsURL(index));
    ws.binaryType = 'arraybuffer';

    ws.onmessage = function (ev) {
      var text = typeof ev.data === 'string' ? ev.data : decodeBuffer(ev.data);
      term.feed(text);
    };
    ws.onclose = function () {
      term.appendPlain('\n[session ended]\n');
    };
    ws.onerror = function () {
      term.appendPlain('\n[connection error]\n');
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

  global.VirtBBSDoor = { connect: connect, ansiToHTML: ansiToHTML };
})(window);
