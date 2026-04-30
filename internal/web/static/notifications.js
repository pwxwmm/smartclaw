// SmartClaw - Notifications
(function() {
  'use strict';

  var notifIdCounter = 0;
  var desktopPermissionRequested = false;
  var expireInterval = null;

  var levelIcons = {
    info:    '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="var(--info)" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="12" y1="16" x2="12" y2="12"/><line x1="12" y1="8" x2="12.01" y2="8"/></svg>',
    success: '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="var(--ok)" stroke-width="2"><path d="M22 11.08V12a10 10 0 11-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>',
    warning: '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="var(--warn)" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>',
    error:   '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="var(--err)" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg>'
  };

  var expiryMs = {
    info: 5 * 60 * 1000,
    success: 30 * 60 * 1000,
    warning: 30 * 60 * 1000,
    error: 0
  };

  SC.Notification = function(level, title, message) {
    this.id = 'notif-' + (++notifIdCounter);
    this.level = level || 'info';
    this.title = title || '';
    this.message = message || '';
    this.timestamp = new Date();
    this.read = false;
  };

  SC.notifications = [];

  SC.addNotification = function(level, title, message) {
    var notif = new SC.Notification(level, title, message);
    SC.notifications.unshift(notif);
    updateBadge();
    maybeExpireNotifications();
    maybeDesktopNotify(notif);
    return notif;
  };

  SC.getNotifications = function() {
    return SC.notifications;
  };

  SC.markAllRead = function() {
    SC.notifications.forEach(function(n) { n.read = true; });
    updateBadge();
    renderNotifDropdown();
  };

  SC.clearNotifications = function() {
    SC.notifications = [];
    updateBadge();
    renderNotifDropdown();
  };

  function getUnreadCount() {
    return SC.notifications.filter(function(n) { return !n.read; }).length;
  }

  function updateBadge() {
    var badge = SC.$('#notification-badge');
    if (!badge) return;
    var count = getUnreadCount();
    if (count > 0) {
      badge.textContent = count > 99 ? '99+' : String(count);
      badge.classList.remove('hidden');
    } else {
      badge.classList.add('hidden');
    }
  }

  function timeAgo(date) {
    var now = new Date();
    var diff = now - date;
    var secs = Math.floor(diff / 1000);
    if (secs < 60) return 'just now';
    var mins = Math.floor(secs / 60);
    if (mins < 60) return mins + 'm ago';
    var hrs = Math.floor(mins / 60);
    if (hrs < 24) return hrs + 'h ago';
    var days = Math.floor(hrs / 24);
    return days + 'd ago';
  }

  function renderNotifDropdown() {
    var list = SC.$('#notif-list');
    if (!list) return;
    if (SC.notifications.length === 0) {
      list.innerHTML = '<div class="notif-empty">No notifications</div>';
      return;
    }
    list.innerHTML = '';
    SC.notifications.forEach(function(notif) {
      var item = document.createElement('div');
      item.className = 'notif-item' + (notif.read ? ' read' : '');
      item.setAttribute('role', 'menuitem');
      item.innerHTML =
        '<div class="notif-icon">' + (levelIcons[notif.level] || levelIcons.info) + '</div>' +
        '<div class="notif-body">' +
          '<div class="notif-title">' + SC.escapeHtml(notif.title) + '</div>' +
          (notif.message ? '<div class="notif-msg">' + SC.escapeHtml(notif.message) + '</div>' : '') +
        '</div>' +
        '<div class="notif-time">' + timeAgo(notif.timestamp) + '</div>';
      item.addEventListener('click', function() {
        notif.read = true;
        updateBadge();
        renderNotifDropdown();
      });
      list.appendChild(item);
    });
  }

  function maybeDesktopNotify(notif) {
    if (document.hasFocus && document.hasFocus()) return;
    if (notif.level !== 'error' && notif.level !== 'warning' && notif.level !== 'success') return;
    if (!desktopPermissionRequested) {
      try {
        if (Notification.permission === 'default') {
          Notification.requestPermission();
        }
        desktopPermissionRequested = true;
      } catch(e) {}
    }
    try {
      if (Notification.permission === 'granted') {
        new Notification(notif.title, {
          body: notif.message,
          tag: notif.id
        });
      }
    } catch(e) {}
  }

  function maybeExpireNotifications() {
    var now = new Date();
    SC.notifications = SC.notifications.filter(function(n) {
      var expiry = expiryMs[n.level];
      if (expiry === 0) return true;
      return (now - n.timestamp) < expiry;
    });
    updateBadge();
  }

  var originalToast = SC.toast;
  SC.toast = function(msg, type) {
    if (originalToast) originalToast(msg, type);
    var level = type === 'success' ? 'success'
              : type === 'error' ? 'error'
              : type === 'warning' ? 'warning'
              : 'info';
    SC.addNotification(level, msg, '');
  };

  SC.initNotifications = function() {
    var btn = SC.$('#btn-notifications');
    var dropdown = SC.$('#notification-dropdown');
    if (!btn || !dropdown) return;

    btn.addEventListener('click', function(e) {
      e.stopPropagation();
      var isHidden = dropdown.classList.contains('hidden');
      dropdown.classList.toggle('hidden', !isHidden);
      if (isHidden) {
        renderNotifDropdown();
      }
    });

    document.addEventListener('click', function(e) {
      if (!dropdown.contains(e.target) && e.target !== btn && !btn.contains(e.target)) {
        dropdown.classList.add('hidden');
      }
    });

    SC.$('#notif-mark-read')?.addEventListener('click', function(e) {
      e.stopPropagation();
      SC.markAllRead();
    });

    SC.$('#notif-clear-all')?.addEventListener('click', function(e) {
      e.stopPropagation();
      SC.clearNotifications();
    });

    expireInterval = setInterval(maybeExpireNotifications, 60000);
    updateBadge();
  };

  SC.destroyNotifications = function() {
    if (expireInterval) {
      clearInterval(expireInterval);
      expireInterval = null;
    }
  };
})();
