(function () {
  'use strict';

  var booted = false;

  function boot() {
    if (booted) return;

    var cfgEl = document.getElementById('bbslist-page-config');
    if (!cfgEl) return;

    var cfg, i18n, pageSize, currentSearch;
    try {
      cfg = JSON.parse(cfgEl.textContent);
      i18n = cfg.i18n || {};
      pageSize = cfg.pageSize || 15;
    } catch (e) {
      return;
    }

    if (typeof window.bootstrap === 'undefined' || !window.bootstrap.Modal) {
      return;
    }
    booted = true;

    function t(key, fallback) {
      return i18n[key] || fallback || key;
    }

    function esc(s) {
      if (s === null || s === undefined) return '';
      var d = document.createElement('div');
      d.textContent = String(s);
      return d.innerHTML;
    }

    function attrEsc(s) {
      return String(s == null ? '' : s)
        .replace(/&/g, '&amp;')
        .replace(/"/g, '&quot;')
        .replace(/</g, '&lt;');
    }

    function formatDate(iso) {
      if (!iso) return '—';
      try {
        return new Date(iso).toLocaleString();
      } catch (e) {
        return iso;
      }
    }

    function getModal() {
      var modalEl = document.getElementById('bbslist-detail-modal');
      if (!modalEl) return null;
      return window.bootstrap.Modal.getOrCreateInstance(modalEl);
    }

    function nodeTable(nodes) {
      if (!nodes || !nodes.length) {
        return '<p class="meta small">' + esc(t('empty', 'No systems yet.')) + '</p>';
      }
      var html = '<div class="table-responsive"><table class="table table-dark table-striped table-hover align-middle mb-0 table-sm">' +
        '<thead><tr>' +
        '<th>' + esc(t('col_address', 'Address')) + '</th>' +
        '<th>' + esc(t('col_name', 'Name')) + '</th>' +
        '<th>' + esc(t('col_location', 'Location')) + '</th>' +
        '<th>' + esc(t('col_echomail', 'Echo')) + '</th>' +
        '<th>' + esc(t('col_netmail', 'Netmail')) + '</th>' +
        '<th>' + esc(t('col_last_seen', 'Last seen')) + '</th>' +
        '<th></th></tr></thead><tbody>';
      nodes.forEach(function (n) {
        html += '<tr>' +
          '<td>' + esc(n.node_addr) + '</td>' +
          '<td>' + esc(n.name || '—') + '</td>' +
          '<td>' + esc(n.location || '—') + '</td>' +
          '<td>' + esc(n.echomail_count) + '</td>' +
          '<td>' + esc(n.netmail_count) + '</td>' +
          '<td class="small">' + esc(formatDate(n.last_seen)) + '</td>' +
          '<td class="text-end"><button type="button" class="btn btn-sm btn-outline-secondary bbslist-view-btn" ' +
          'data-network="' + attrEsc(n.network) + '" data-addr="' + attrEsc(n.node_addr) + '">' +
          esc(t('view', 'View')) + '</button></td></tr>';
      });
      html += '</tbody></table></div>';
      return html;
    }

    function renderPager(el, page, pages, onPage) {
      if (!el) return;
      if (pages <= 1) {
        el.hidden = true;
        el.innerHTML = '';
        return;
      }
      el.hidden = false;
      var html = esc(t('page', 'Page')) + ' ' + page + ' ' + esc(t('of', 'of')) + ' ' + pages + ' ';
      if (page > 1) {
        html += '<button type="button" class="btn btn-link btn-sm p-0 bbslist-page-btn" data-page="' + (page - 1) + '">' +
          esc(t('previous', 'Previous')) + '</button> ';
      }
      if (page < pages) {
        html += '<button type="button" class="btn btn-link btn-sm p-0 bbslist-page-btn" data-page="' + (page + 1) + '">' +
          esc(t('next', 'Next')) + '</button>';
      }
      el.innerHTML = html;
      el.querySelectorAll('.bbslist-page-btn').forEach(function (btn) {
        btn.addEventListener('click', function () {
          onPage(parseInt(btn.getAttribute('data-page'), 10));
        });
      });
    }

    function loadSection(section, wrapEl, pagerEl, network, page) {
      var url = '/api/bbslist?section=' + encodeURIComponent(section) + '&page=' + page;
      if (network) url += '&network=' + encodeURIComponent(network);
      if (currentSearch) url += '&q=' + encodeURIComponent(currentSearch);
      wrapEl.innerHTML = '<p class="meta small">' + esc(t('loading', 'Loading…')) + '</p>';
      fetch(url, { credentials: 'same-origin' })
        .then(function (r) { return r.json(); })
        .then(function (data) {
          wrapEl.innerHTML = nodeTable(data.nodes);
          renderPager(pagerEl, data.page, data.pages, function (p) {
            loadSection(section, wrapEl, pagerEl, network, p);
          });
        })
        .catch(function () {
          wrapEl.innerHTML = '<p class="meta text-danger">Error</p>';
        });
    }

    function reloadAllSections() {
      if (echoWrap) loadSection('echomail', echoWrap, echoPager, '', 1);
      if (netWrap) loadSection('netmail', netWrap, netPager, '', 1);
      document.querySelectorAll('.bbslist-network-wrap').forEach(function (wrap) {
        var network = wrap.getAttribute('data-network');
        var pager = document.querySelector('.bbslist-network-pager[data-network="' + network + '"]');
        loadSection('network', wrap, pager, network, 1);
      });
    }

    var modalBody = document.getElementById('bbslist-detail-body');
    var modalTitle = document.getElementById('bbslist-detail-title');

    function detailRow(label, value) {
      return '<div class="nodelist-detail-row"><span class="nodelist-detail-label">' + esc(label) +
        '</span><span class="nodelist-detail-value">' + (value ? esc(value) : '—') + '</span></div>';
    }

    function renderUsers(users) {
      if (!users || !users.length) {
        return '<p class="meta small">—</p>';
      }
      var html = '<div class="table-responsive"><table class="table table-dark table-striped table-sm mb-0">' +
        '<thead><tr><th>' + esc(t('col_user', 'User')) + '</th>' +
        '<th>' + esc(t('col_user_addr', 'Address')) + '</th>' +
        '<th>' + esc(t('col_echomail', 'Echo')) + '</th>' +
        '<th>' + esc(t('col_netmail', 'Netmail')) + '</th></tr></thead><tbody>';
      users.forEach(function (u) {
        html += '<tr><td>' + esc(u.user_name) + '</td>' +
          '<td><button type="button" class="btn btn-link btn-sm p-0 bbslist-add-contact" data-name="' +
          attrEsc(u.user_name) + '" data-addr="' + attrEsc(u.user_addr) + '">' + esc(u.user_addr) + '</button></td>' +
          '<td>' + esc(u.echomail_count) + '</td><td>' + esc(u.netmail_count) + '</td></tr>';
      });
      html += '</tbody></table></div>';
      return html;
    }

    function openNode(network, addr) {
      var modal = getModal();
      if (!modal || !modalBody) return;
      modalBody.innerHTML = '<p class="meta">' + esc(t('loading', 'Loading…')) + '</p>';
      modal.show();
      var url = '/api/bbslist/node?network=' + encodeURIComponent(network) + '&addr=' + encodeURIComponent(addr);
      fetch(url, { credentials: 'same-origin' })
        .then(function (r) { return r.json(); })
        .then(function (data) {
          var n = data.node || {};
          modalTitle.textContent = (n.name || n.address || addr) + ' (' + network + ')';
          var html = '';
          html += detailRow(t('network', 'Network'), n.network);
          html += detailRow(t('col_address', 'Address'), n.address);
          if (n.aka) html += detailRow('AKA', n.aka);
          html += detailRow(t('col_name', 'Name'), n.name);
          html += detailRow(t('col_location', 'Location'), n.location);
          html += detailRow(t('col_sysop', 'Sysop'), n.sysop);
          html += detailRow(t('col_echomail', 'Echo'), String(data.stats ? data.stats.echomail_count : 0));
          html += detailRow(t('col_netmail', 'Netmail'), String(data.stats ? data.stats.netmail_count : 0));
          if (!n.name && !n.sysop) {
            html += '<p class="meta small">' + esc(t('no_nodelist', 'Not found in nodelist.')) + '</p>';
          }
          html += '<h4 class="h6 section-head mt-3">' + esc(t('users_heading', 'Users seen')) + '</h4>';
          html += renderUsers(data.users);
          modalBody.innerHTML = html;
          modalBody.querySelectorAll('.bbslist-add-contact').forEach(function (btn) {
            btn.addEventListener('click', function () {
              fetch('/api/addressbook', {
                method: 'POST',
                credentials: 'same-origin',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                  name: btn.getAttribute('data-name'),
                  fido_addr: btn.getAttribute('data-addr')
                })
              })
                .then(function (r) { return r.json(); })
                .then(function (res) {
                  if (res && res.ok) {
                    btn.textContent = btn.getAttribute('data-addr') + ' ✓';
                    btn.disabled = true;
                  }
                });
            });
          });
        })
        .catch(function () {
          modalBody.innerHTML = '<p class="meta text-danger">Error</p>';
        });
    }

    document.addEventListener('click', function (e) {
      var btn = e.target.closest('.bbslist-view-btn');
      if (!btn) return;
      openNode(btn.getAttribute('data-network'), btn.getAttribute('data-addr'));
    });

    var echoWrap = document.getElementById('bbslist-echomail-wrap');
    var echoPager = document.getElementById('bbslist-echomail-pager');
    var netWrap = document.getElementById('bbslist-netmail-wrap');
    var netPager = document.getElementById('bbslist-netmail-pager');
    var searchInput = document.getElementById('bbslist-search-input');
    var searchBtn = document.getElementById('bbslist-search-btn');
    var searchClear = document.getElementById('bbslist-search-clear');

    currentSearch = '';
    reloadAllSections();

    function applySearch() {
      currentSearch = searchInput ? searchInput.value.trim() : '';
      reloadAllSections();
    }

    if (searchBtn) {
      searchBtn.addEventListener('click', applySearch);
    }
    if (searchClear) {
      searchClear.addEventListener('click', function () {
        if (searchInput) searchInput.value = '';
        currentSearch = '';
        reloadAllSections();
      });
    }
    if (searchInput) {
      searchInput.addEventListener('keydown', function (e) {
        if (e.key === 'Enter') {
          e.preventDefault();
          applySearch();
        }
      });
    }

    function loadCharts() {
      if (typeof Chart === 'undefined') return;
      fetch('/api/bbslist/charts', { credentials: 'same-origin' })
        .then(function (r) { return r.json(); })
        .then(function (data) {
          var gridColor = 'rgba(255,255,255,0.08)';
          var tickColor = 'rgba(255,255,255,0.65)';
          function lineChart(id, values, label, color) {
            var el = document.getElementById(id);
            if (!el) return;
            if (!data.has_data) {
              el.parentElement.innerHTML = '<div class="stats-chart-empty" role="status"><p class="stats-chart-empty-note">' +
                esc(t('chart_insufficient', 'Not enough data yet.')) + '</p></div>';
              return;
            }
            new Chart(el, {
              type: 'line',
              data: {
                labels: data.labels,
                datasets: [{
                  label: label,
                  data: values,
                  borderColor: color,
                  backgroundColor: color.replace('1)', '0.15)'),
                  fill: true,
                  tension: 0.25,
                  pointRadius: 2
                }]
              },
              options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: { legend: { labels: { color: tickColor } } },
                scales: {
                  x: { ticks: { color: tickColor, maxTicksLimit: 8 }, grid: { color: gridColor } },
                  y: { beginAtZero: true, ticks: { color: tickColor, precision: 0 }, grid: { color: gridColor } }
                }
              }
            });
          }
          lineChart('bbslist-chart-echo', data.echomail, t('chart_echomail', 'Echomail'), 'rgba(129, 230, 217, 1)');
          lineChart('bbslist-chart-netmail', data.netmail, t('chart_netmail', 'Netmail'), 'rgba(99, 179, 237, 1)');
        });
    }

    loadCharts();
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', boot);
  } else {
    boot();
  }
})();
