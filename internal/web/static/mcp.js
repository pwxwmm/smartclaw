(function() {
  'use strict';

  var MCP_CATEGORY_COLORS = {
    'Development': '#3b82f6',
    'Data': '#10b981',
    'AI': '#8b5cf6',
    'Operations': '#f59e0b',
    'Communication': '#ec4899',
    'Productivity': '#06b6d4'
  };

  function renderMCPServers() {
    var panel = SC.$('#mcp-servers-panel');
    if (!panel) return;
    var servers = SC.state.mcpServers || [];

    if (servers.length === 0) {
      panel.innerHTML = '<div class="empty-state"><span class="empty-desc">No MCP servers configured</span></div>';
      return;
    }

    var html = '';
    servers.forEach(function(s) {
      var isRunning = s._running || false;
      var statusDot = isRunning ? '<span class="mcp-status-dot running"></span>' : '<span class="mcp-status-dot stopped"></span>';
      var statusText = isRunning ? 'Running' : 'Stopped';
      var transport = s.type || 'stdio';
      var cmdDisplay = transport === 'sse' ? (s.url || '') : (s.command || '') + ' ' + (s.args || []).join(' ');

      html += '<div class="mcp-server-card' + (isRunning ? ' running' : '') + '" data-name="' + SC.escapeHtml(s.name) + '">';
      html += '<div class="mcp-server-header">';
      html += statusDot;
      html += '<span class="mcp-server-name">' + SC.escapeHtml(s.name) + '</span>';
      html += '<span class="mcp-server-status ' + (isRunning ? 'running' : 'stopped') + '">' + statusText + '</span>';
      html += '</div>';
      if (s.description) {
        html += '<div class="mcp-server-desc">' + SC.escapeHtml(s.description) + '</div>';
      }
      html += '<div class="mcp-server-meta">';
      html += '<span class="mcp-server-transport">' + SC.escapeHtml(transport) + '</span>';
      html += '<span class="mcp-server-cmd">' + SC.escapeHtml(cmdDisplay.trim()) + '</span>';
      if (s.auto_start) html += '<span class="mcp-server-autostart">Auto-start ✓</span>';
      html += '</div>';
      html += '<div class="mcp-server-actions">';
      if (isRunning) {
        html += '<button class="btn-ghost sm mcp-stop-btn" data-name="' + SC.escapeHtml(s.name) + '">Stop</button>';
      } else {
        html += '<button class="btn-ghost sm mcp-start-btn" data-name="' + SC.escapeHtml(s.name) + '">Start</button>';
      }
      html += '<button class="btn-ghost sm mcp-config-btn" data-name="' + SC.escapeHtml(s.name) + '">Config</button>';
      html += '<button class="btn-ghost sm mcp-remove-btn" data-name="' + SC.escapeHtml(s.name) + '">Remove</button>';
      html += '</div>';
      html += '<div class="mcp-server-config hidden" data-name="' + SC.escapeHtml(s.name) + '"></div>';
      html += '</div>';
    });
    panel.innerHTML = html;

    wireMCPServerActions();
  }

  function wireMCPServerActions() {
    SC.$$('.mcp-start-btn').forEach(function(btn) {
      btn.addEventListener('click', function() {
        SC.wsSend('mcp_start', { name: this.dataset.name });
        SC.toast('Starting ' + this.dataset.name + '...', 'info');
      });
    });

    SC.$$('.mcp-stop-btn').forEach(function(btn) {
      btn.addEventListener('click', function() {
        SC.wsSend('mcp_stop', { name: this.dataset.name });
        SC.toast('Stopping ' + this.dataset.name + '...', 'info');
      });
    });

    SC.$$('.mcp-config-btn').forEach(function(btn) {
      btn.addEventListener('click', function() {
        var configEl = SC.$('.mcp-server-config[data-name="' + this.dataset.name + '"]');
        if (configEl) {
          configEl.classList.toggle('hidden');
          if (!configEl.classList.contains('hidden') && !configEl.innerHTML) {
            renderServerConfig(configEl, this.dataset.name);
          }
        }
      });
    });

    SC.$$('.mcp-remove-btn').forEach(function(btn) {
      btn.addEventListener('click', function() {
        var name = this.dataset.name;
        if (confirm('Remove MCP server "' + name + '"?')) {
          SC.wsSend('mcp_remove', { name: name });
        }
      });
    });
  }

  function renderServerConfig(container, name) {
    var servers = SC.state.mcpServers || [];
    var server = servers.find(function(s) { return s.name === name; });
    if (!server) return;

    var html = '<div class="mcp-config-fields">';
    html += '<div class="mcp-config-row"><label>Name</label><input type="text" class="form-input sm" value="' + SC.escapeHtml(server.name) + '" disabled></div>';
    html += '<div class="mcp-config-row"><label>Transport</label><input type="text" class="form-input sm" value="' + SC.escapeHtml(server.type || 'stdio') + '" disabled></div>';
    if (server.type === 'sse') {
      html += '<div class="mcp-config-row"><label>URL</label><input type="text" class="form-input sm" value="' + SC.escapeHtml(server.url || '') + '" disabled></div>';
    } else {
      html += '<div class="mcp-config-row"><label>Command</label><input type="text" class="form-input sm" value="' + SC.escapeHtml(server.command || '') + '" disabled></div>';
      html += '<div class="mcp-config-row"><label>Args</label><input type="text" class="form-input sm" value="' + SC.escapeHtml((server.args || []).join(', ')) + '" disabled></div>';
    }
    if (server.env && Object.keys(server.env).length) {
      html += '<div class="mcp-config-row"><label>Env</label>';
      Object.keys(server.env).forEach(function(k) {
        html += '<div class="mcp-config-env"><span>' + SC.escapeHtml(k) + '</span>=<span>' + SC.escapeHtml(server.env[k]) + '</span></div>';
      });
      html += '</div>';
    }
    html += '<div class="mcp-config-row"><label>Auto-start</label><span>' + (server.auto_start ? 'Yes' : 'No') + '</span></div>';
    html += '</div>';
    container.innerHTML = html;
  }

  function renderMCPCatalogView() {
    var catalog = SC.state.mcpCatalog || [];
    var servers = SC.state.mcpServers || [];
    var installedNames = {};
    servers.forEach(function(s) { installedNames[s.name] = true; });

    var searchEl = SC.$('#mcp-catalog-search-view');
    var query = searchEl ? searchEl.value.toLowerCase().trim() : '';
    var chipsEl = SC.$('#mcp-category-chips-view');
    var gridEl = SC.$('#mcp-catalog-grid-view');
    if (!gridEl) return;

    var categories = ['All'];
    catalog.forEach(function(item) {
      if (categories.indexOf(item.category) < 0) categories.push(item.category);
    });

    var activeCategory = SC.state.mcpCatalogCategory || 'All';

    if (chipsEl) {
      var chipsHtml = '';
      categories.forEach(function(cat) {
        chipsHtml += '<button class="mcp-chip' + (cat === activeCategory ? ' active' : '') + '" data-category="' + SC.escapeHtml(cat) + '">' + SC.escapeHtml(cat) + '</button>';
      });
      chipsEl.innerHTML = chipsHtml;
      chipsEl.querySelectorAll('.mcp-chip').forEach(function(chip) {
        chip.addEventListener('click', function() {
          SC.state.mcpCatalogCategory = this.dataset.category;
          renderMCPCatalogView();
        });
      });
    }

    var filtered = catalog.filter(function(item) {
      var matchCat = activeCategory === 'All' || item.category === activeCategory;
      var matchQuery = !query || item.name.toLowerCase().indexOf(query) >= 0 || item.description.toLowerCase().indexOf(query) >= 0;
      return matchCat && matchQuery;
    });

    var html = '';
    filtered.forEach(function(item) {
      var isInstalled = !!installedNames[item.name];
      var catColor = MCP_CATEGORY_COLORS[item.category] || '#6a6a72';
      html += '<div class="mcp-card' + (isInstalled ? ' installed' : '') + '">';
      html += '<div class="mcp-card-name">' + SC.escapeHtml(item.name) + '</div>';
      html += '<div class="mcp-card-desc">' + SC.escapeHtml(item.description) + '</div>';
      html += '<div class="mcp-card-footer">';
      html += '<span class="mcp-card-category" style="background:' + catColor + '">' + SC.escapeHtml(item.category) + '</span>';
      if (isInstalled) {
        html += '<span class="mcp-card-install installed-label">✓ Installed</span>';
      } else {
        html += '<button class="mcp-card-install" data-name="' + SC.escapeHtml(item.name) + '" data-type="' + SC.escapeHtml(item.type) + '" data-command="' + SC.escapeHtml(item.command) + '" data-args="' + SC.escapeHtml(JSON.stringify(item.args || [])) + '">Install</button>';
      }
      html += '</div></div>';
    });

    if (filtered.length === 0) {
      html = '<div class="empty-state"><span class="empty-desc">No servers found</span></div>';
    }

    gridEl.innerHTML = html;

    gridEl.querySelectorAll('.mcp-card-install[data-name]').forEach(function(btn) {
      btn.addEventListener('click', function() {
        var name = this.dataset.name;
        var type = this.dataset.type;
        var command = this.dataset.command;
        var args;
        try { args = JSON.parse(this.dataset.args); } catch(e) { args = []; }
        SC.wsSend('mcp_add', { name: name, type: type, command: command, args: args });
        SC.toast('Installing ' + name + '...', 'success');
      });
    });

    if (searchEl && !searchEl._mcpBound) {
      searchEl._mcpBound = true;
      searchEl.addEventListener('input', function() { renderMCPCatalogView(); });
    }
  }

  function renderMCPToolsView() {
    var select = SC.$('#mcp-tools-server-select');
    var toolsList = SC.$('#mcp-tools-list');
    var resourcesList = SC.$('#mcp-resources-list');
    if (!select) return;

    var servers = SC.state.mcpServers || [];
    var currentVal = select.value;
    select.innerHTML = '<option value="">Select a server...</option>';
    servers.forEach(function(s) {
      if (s._running) {
        select.innerHTML += '<option value="' + SC.escapeHtml(s.name) + '">' + SC.escapeHtml(s.name) + '</option>';
      }
    });
    select.value = currentVal;

    if (!select.value) {
      if (toolsList) toolsList.innerHTML = '<div class="empty-state"><span class="empty-desc">Select a running server to view tools and resources</span></div>';
      if (resourcesList) resourcesList.innerHTML = '';
      return;
    }
  }

  function renderMCPToolsData(data) {
    var toolsList = SC.$('#mcp-tools-list');
    if (!toolsList) return;

    var tools = data.tools || [];
    if (data.error) {
      toolsList.innerHTML = '<div class="mcp-tools-error">' + SC.escapeHtml(data.error) + '</div>';
      return;
    }

    if (tools.length === 0) {
      toolsList.innerHTML = '<div class="empty-state"><span class="empty-desc">No tools available</span></div>';
      return;
    }

    var html = '<div class="mcp-tools-header-label">🔧 Tools (' + tools.length + ')</div>';
    tools.forEach(function(tool) {
      html += '<div class="mcp-tool-card">';
      html += '<div class="mcp-tool-name">' + SC.escapeHtml(tool.name || tool.Name || '') + '</div>';
      html += '<div class="mcp-tool-desc">' + SC.escapeHtml(tool.description || tool.Description || '') + '</div>';
      var schema = tool.inputSchema || tool.InputSchema || tool.input_schema;
      if (schema && schema.properties) {
        html += '<div class="mcp-tool-schema">';
        Object.keys(schema.properties).forEach(function(key) {
          var prop = schema.properties[key];
          var required = schema.required && schema.required.indexOf(key) >= 0;
          html += '<span class="mcp-tool-param' + (required ? ' required' : '') + '">' + SC.escapeHtml(key) + '<span class="mcp-param-type">:' + SC.escapeHtml(prop.type || 'any') + '</span>' + (required ? '*' : '') + '</span>';
        });
        html += '</div>';
      }
      html += '</div>';
    });
    toolsList.innerHTML = html;
  }

  function renderMCPResourcesData(data) {
    var resourcesList = SC.$('#mcp-resources-list');
    if (!resourcesList) return;

    var resources = data.resources || [];
    if (resources.length === 0) {
      resourcesList.innerHTML = '';
      return;
    }

    var html = '<div class="mcp-tools-header-label">📄 Resources (' + resources.length + ')</div>';
    resources.forEach(function(res) {
      var uri = res.uri || res.URI || '';
      var name = res.name || res.Name || uri;
      var mimeType = res.mimeType || res.MimeType || '';
      html += '<div class="mcp-resource-card">';
      html += '<span class="mcp-resource-name">' + SC.escapeHtml(name) + '</span>';
      if (uri && uri !== name) {
        html += '<span class="mcp-resource-uri">' + SC.escapeHtml(uri) + '</span>';
      }
      if (mimeType) {
        html += '<span class="mcp-resource-mime">' + SC.escapeHtml(mimeType) + '</span>';
      }
      html += '</div>';
    });
    resourcesList.innerHTML = html;
  }

  function initMCPAddModal() {
    var modal = SC.$('#mcp-add-modal');
    var typeSelect = SC.$('#mcp-add-type');
    var stdioFields = SC.$('#mcp-add-stdio-fields');
    var sseFields = SC.$('#mcp-add-sse-fields');
    var cancelBtn = SC.$('#mcp-add-cancel');
    var closeBtn = SC.$('#mcp-add-modal-close');
    var submitBtn = SC.$('#mcp-add-submit');
    var addBtn = SC.$('#mcp-add-btn');

    if (!modal) return;

    if (typeSelect) {
      typeSelect.addEventListener('change', function() {
        if (this.value === 'sse') {
          stdioFields.classList.add('hidden');
          sseFields.classList.remove('hidden');
        } else {
          stdioFields.classList.remove('hidden');
          sseFields.classList.add('hidden');
        }
      });
    }

    function closeModal() {
      modal.classList.add('hidden');
      SC.$('#mcp-add-name').value = '';
      SC.$('#mcp-add-command').value = '';
      SC.$('#mcp-add-args').value = '';
      SC.$('#mcp-add-url').value = '';
      SC.$('#mcp-add-env').value = '';
      SC.$('#mcp-add-desc').value = '';
      SC.$('#mcp-add-autostart').checked = true;
      typeSelect.value = 'stdio';
      stdioFields.classList.remove('hidden');
      sseFields.classList.add('hidden');
    }

    if (addBtn) addBtn.addEventListener('click', function() { modal.classList.remove('hidden'); SC.$('#mcp-add-name').focus(); });
    if (cancelBtn) cancelBtn.addEventListener('click', closeModal);
    if (closeBtn) closeBtn.addEventListener('click', closeModal);
    modal.addEventListener('click', function(e) { if (e.target === modal) closeModal(); });

    if (submitBtn) {
      submitBtn.addEventListener('click', function() {
        var name = SC.$('#mcp-add-name').value.trim();
        var type = SC.$('#mcp-add-type').value;
        var command = SC.$('#mcp-add-command').value.trim();
        var argsStr = SC.$('#mcp-add-args').value.trim();
        var url = SC.$('#mcp-add-url').value.trim();
        var envStr = SC.$('#mcp-add-env').value.trim();
        var desc = SC.$('#mcp-add-desc').value.trim();
        var autoStart = SC.$('#mcp-add-autostart').checked;

        if (!name) { SC.toast('Server name is required', 'error'); return; }
        if (type === 'stdio' && !command) { SC.toast('Command is required for stdio transport', 'error'); return; }
        if (type === 'sse' && !url) { SC.toast('URL is required for SSE transport', 'error'); return; }

        var args = [];
        if (argsStr) {
          args = argsStr.split(',').map(function(a) { return a.trim(); }).filter(function(a) { return a; });
        }

        var env = {};
        if (envStr) {
          envStr.split('\n').forEach(function(line) {
            var eq = line.indexOf('=');
            if (eq > 0) {
              env[line.slice(0, eq).trim()] = line.slice(eq + 1).trim();
            }
          });
        }

        SC.wsSend('mcp_add', {
          name: name,
          type: type,
          command: command,
          args: args,
          url: url,
          env: env,
          auto_start: autoStart,
          description: desc
        });

        closeModal();
      });
    }
  }

  function initMCPTabs() {
    var tabs = SC.$$('.mcp-view-tab');
    var panels = {
      servers: SC.$('#mcp-servers-panel'),
      catalog: SC.$('#mcp-catalog-panel'),
      tools: SC.$('#mcp-tools-panel')
    };

    tabs.forEach(function(tab) {
      tab.addEventListener('click', function() {
        tabs.forEach(function(t) { t.classList.remove('active'); });
        this.classList.add('active');
        var view = this.dataset.view;
        Object.keys(panels).forEach(function(key) {
          if (panels[key]) {
            panels[key].classList.toggle('hidden', key !== view);
          }
        });
      });
    });
  }

  function initMCPToolsSelect() {
    var select = SC.$('#mcp-tools-server-select');
    var refreshBtn = SC.$('#mcp-tools-refresh');

    if (select) {
      select.addEventListener('change', function() {
        var name = this.value;
        if (name) {
          SC.wsSend('mcp_tools', { name: name });
          SC.wsSend('mcp_resources', { name: name });
        } else {
          var toolsList = SC.$('#mcp-tools-list');
          var resourcesList = SC.$('#mcp-resources-list');
          if (toolsList) toolsList.innerHTML = '<div class="empty-state"><span class="empty-desc">Select a running server</span></div>';
          if (resourcesList) resourcesList.innerHTML = '';
        }
      });
    }

    if (refreshBtn) {
      refreshBtn.addEventListener('click', function() {
        var name = select ? select.value : '';
        if (name) {
          SC.wsSend('mcp_tools', { name: name });
          SC.wsSend('mcp_resources', { name: name });
        }
      });
    }
  }

  function renderMCPInstalled() {
    var container = SC.$('#mcp-installed-list');
    if (!container) return;    var servers = SC.state.mcpServers || [];

    if (servers.length === 0) {
      container.innerHTML = '<div class="mcp-server-item" style="justify-content:center;color:var(--tx-2);font-size:12px">No servers</div>';
      return;
    }

    var html = '';
    servers.forEach(function(s) {
      var isRunning = s._running || false;
      var statusClass = isRunning ? 'running' : 'stopped';
      var tools = SC.state._mcpTools[s.name] || [];
      var resources = SC.state._mcpResources[s.name] || [];
      var toolCount = tools.length;
      var resCount = resources.length;

      html += '<div class="mcp-server-item" data-name="' + SC.escapeHtml(s.name) + '">';
      html += '<span class="mcp-server-status ' + statusClass + '"></span>';
      html += '<span class="mcp-server-item-name">' + SC.escapeHtml(s.name) + '</span>';
      if (isRunning && (toolCount || resCount)) {
        html += '<span class="mcp-server-item-meta">' + toolCount + 'T ' + resCount + 'R</span>';
      }
      html += '<div class="mcp-server-actions">';
      if (isRunning) {
        html += '<button class="mcp-sidebar-stop" data-name="' + SC.escapeHtml(s.name) + '" title="Stop">&#9632;</button>';
      } else {
        html += '<button class="mcp-sidebar-start" data-name="' + SC.escapeHtml(s.name) + '" title="Start">&#9654;</button>';
      }
      html += '<button class="mcp-sidebar-remove" data-name="' + SC.escapeHtml(s.name) + '" title="Remove">&times;</button>';
      html += '</div>';
      html += '</div>';

      if (isRunning && (toolCount || resCount)) {
        html += '<div class="mcp-server-tools" data-name="' + SC.escapeHtml(s.name) + '">';
        tools.forEach(function(t) {
          var tName = t.name || t.Name || '';
          html += '<div class="mcp-tool-item">' + SC.escapeHtml(tName) + '</div>';
        });
        resources.forEach(function(r) {
          var rName = r.name || r.Name || r.uri || '';
          html += '<div class="mcp-resource-item">' + SC.escapeHtml(rName) + '</div>';
        });
        html += '</div>';
      }
    });
    container.innerHTML = html;
    wireMCPInstalledActions();
  }

  function renderMCPSidebarCatalog() {
    var container = SC.$('#mcp-catalog-list');
    if (!container) return;    var catalog = SC.state.mcpCatalog || [];
    var servers = SC.state.mcpServers || [];
    var installedNames = {};
    servers.forEach(function(s) { installedNames[s.name] = true; });

    var available = catalog.filter(function(item) { return !installedNames[item.name]; });
    if (available.length === 0) {
      container.innerHTML = '<div class="mcp-catalog-item" style="justify-content:center;color:var(--tx-2);font-size:12px">All installed</div>';
      return;
    }

    var html = '';
    available.forEach(function(item) {
      var catColor = MCP_CATEGORY_COLORS[item.category] || '#6a6a72';
      html += '<div class="mcp-catalog-item">';
      html += '<span class="mcp-catalog-badge" style="background:' + catColor + '">' + SC.escapeHtml(item.category || '') + '</span>';
      html += '<span class="mcp-catalog-item-name">' + SC.escapeHtml(item.name) + '</span>';
      html += '<button class="mcp-catalog-install" data-name="' + SC.escapeHtml(item.name) + '" data-type="' + SC.escapeHtml(item.type || 'stdio') + '" data-command="' + SC.escapeHtml(item.command || '') + '" data-args="' + SC.escapeHtml(JSON.stringify(item.args || [])) + '" title="Install">+</button>';
      html += '</div>';
    });
    container.innerHTML = html;

    container.querySelectorAll('.mcp-catalog-install').forEach(function(btn) {
      btn.addEventListener('click', function() {
        var name = this.dataset.name;
        var type = this.dataset.type;
        var command = this.dataset.command;
        var args;
        try { args = JSON.parse(this.dataset.args); } catch(e) { args = []; }
        SC.wsSend('mcp_add', { name: name, type: type, command: command, args: args });
        SC.toast('Installing ' + name + '...', 'info');
      });
    });
  }

  function wireMCPInstalledActions() {
    var container = SC.$('#mcp-installed-list');
    if (!container) return;

    container.querySelectorAll('.mcp-sidebar-start').forEach(function(btn) {
      btn.addEventListener('click', function() {
        SC.wsSend('mcp_start', { name: this.dataset.name });
        SC.toast('Starting ' + this.dataset.name + '...', 'info');
      });
    });

    container.querySelectorAll('.mcp-sidebar-stop').forEach(function(btn) {
      btn.addEventListener('click', function() {
        SC.wsSend('mcp_stop', { name: this.dataset.name });
      });
    });

    container.querySelectorAll('.mcp-sidebar-remove').forEach(function(btn) {
      btn.addEventListener('click', function() {
        var name = this.dataset.name;
        if (confirm('Remove "' + name + '"?')) {
          SC.wsSend('mcp_remove', { name: name });
        }
      });
    });

    container.querySelectorAll('.mcp-server-item-name').forEach(function(el) {
      el.addEventListener('click', function() {
        var name = this.textContent;
        var toolsDiv = container.querySelector('.mcp-server-tools[data-name="' + name + '"]');
        if (toolsDiv) {
          toolsDiv.classList.toggle('hidden');
          if (!toolsDiv.classList.contains('hidden')) {
            SC.wsSend('mcp_tools', { name: name });
            SC.wsSend('mcp_resources', { name: name });
          }
        }
      });
    });
  }

  function resetAddForm() {
    var nameEl = SC.$('#mcp-sidebar-name');
    var typeEl = SC.$('#mcp-sidebar-type');
    var cmdEl = SC.$('#mcp-sidebar-command');
    var argsEl = SC.$('#mcp-sidebar-args');
    var urlEl = SC.$('#mcp-sidebar-url');
    var descEl = SC.$('#mcp-sidebar-desc');
    var autoEl = SC.$('#mcp-sidebar-autostart');
    if (nameEl) nameEl.value = '';
    if (typeEl) typeEl.value = 'stdio';
    if (cmdEl) cmdEl.value = '';
    if (argsEl) argsEl.value = '';
    if (urlEl) urlEl.value = '';
    if (descEl) descEl.value = '';
    if (autoEl) autoEl.checked = true;
    var stdioFields = SC.$('#mcp-sidebar-stdio-fields');
    var sseFields = SC.$('#mcp-sidebar-sse-fields');
    if (stdioFields) stdioFields.classList.remove('hidden');
    if (sseFields) sseFields.classList.add('hidden');
  }

  function initMCPAddForm() {
    var addBtn = SC.$('#btn-add-mcp');
    var form = SC.$('#mcp-add-form');
    if (!form) return;
    var typeEl = SC.$('#mcp-sidebar-type');
    var cancelBtn = SC.$('#mcp-sidebar-cancel');
    var submitBtn = SC.$('#mcp-sidebar-submit');

    if (addBtn && form) {
      addBtn.addEventListener('click', function() {
        form.classList.toggle('hidden');
        if (!form.classList.contains('hidden')) {
          var nameEl = SC.$('#mcp-sidebar-name');
          if (nameEl) nameEl.focus();
        }
      });
    }

    if (typeEl) {
      typeEl.addEventListener('change', function() {
        var stdioFields = SC.$('#mcp-sidebar-stdio-fields');
        var sseFields = SC.$('#mcp-sidebar-sse-fields');
        if (this.value === 'sse') {
          if (stdioFields) stdioFields.classList.add('hidden');
          if (sseFields) sseFields.classList.remove('hidden');
        } else {
          if (stdioFields) stdioFields.classList.remove('hidden');
          if (sseFields) sseFields.classList.add('hidden');
        }
      });
    }

    if (cancelBtn) {
      cancelBtn.addEventListener('click', function() {
        if (form) form.classList.add('hidden');
        resetAddForm();
      });
    }

    if (submitBtn) {
      submitBtn.addEventListener('click', function() {
        var name = (SC.$('#mcp-sidebar-name') || {}).value || '';
        var type = (SC.$('#mcp-sidebar-type') || {}).value || 'stdio';
        var command = (SC.$('#mcp-sidebar-command') || {}).value || '';
        var argsStr = (SC.$('#mcp-sidebar-args') || {}).value || '';
        var url = (SC.$('#mcp-sidebar-url') || {}).value || '';
        var desc = (SC.$('#mcp-sidebar-desc') || {}).value || '';
        var autoStart = SC.$('#mcp-sidebar-autostart') ? SC.$('#mcp-sidebar-autostart').checked : true;

        name = name.trim();
        command = command.trim();
        url = url.trim();

        if (!name) { SC.toast('Name required', 'error'); return; }
        if (type === 'stdio' && !command) { SC.toast('Command required', 'error'); return; }
        if (type === 'sse' && !url) { SC.toast('URL required', 'error'); return; }

        var args = [];
        if (argsStr) {
          args = argsStr.split(',').map(function(a) { return a.trim(); }).filter(function(a) { return a; });
        }

        SC.wsSend('mcp_add', {
          name: name,
          type: type,
          command: command,
          args: args,
          url: url,
          auto_start: autoStart,
          description: desc.trim()
        });

        if (form) form.classList.add('hidden');
        resetAddForm();
      });
    }
  }

  function initMCP() {
    if (!SC.state._mcpTools) SC.state._mcpTools = {};
    if (!SC.state._mcpResources) SC.state._mcpResources = {};
    initMCPTabs();
    initMCPAddModal();
    initMCPToolsSelect();
    initMCPAddForm();
    SC.wsSend('mcp_list', {});
    SC.wsSend('mcp_catalog', {});
  }

  SC.renderMCPServers = renderMCPServers;
  SC.renderMCPCatalogView = renderMCPCatalogView;
  SC.renderMCPToolsView = renderMCPToolsView;
  SC.renderMCPInstalled = renderMCPInstalled;
  SC.renderMCPSidebarCatalog = renderMCPSidebarCatalog;
  SC.renderMCPToolsData = renderMCPToolsData;
  SC.renderMCPResourcesData = renderMCPResourcesData;
  SC.initMCP = initMCP;
})();
