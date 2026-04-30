// SmartClaw - Skill Marketplace
(function() {
  'use strict';

  var state = {
    mode: 'installed',
    query: '',
    category: '',
    page: 1,
    pageSize: 20,
    categories: [],
    featured: [],
    results: null,
    loading: false,
    installedNames: {},
    searchTimer: null,
    initialized: false
  };

  function fetchJSON(url) {
    return fetch(url).then(function(r) {
      if (!r.ok) throw new Error('HTTP ' + r.status);
      return r.json();
    });
  }

  function postJSON(url, data) {
    return fetch(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data)
    }).then(function(r) {
      if (!r.ok) throw new Error('HTTP ' + r.status);
      return r.json();
    });
  }

  function starRating(rating) {
    var val = parseFloat(rating) || 0;
    var full = Math.floor(val);
    var hasHalf = (val - full) >= 0.5;
    var html = '<div class="marketplace-rating">';
    for (var i = 0; i < full; i++) {
      html += '<span class="star full">\u2605</span>';
    }
    if (hasHalf) {
      html += '<span class="star half">\u2605</span>';
    }
    var empty = 5 - full - (hasHalf ? 1 : 0);
    for (var j = 0; j < empty; j++) {
      html += '<span class="star empty">\u2606</span>';
    }
    html += '<span class="rating-num">' + val.toFixed(1) + '</span></div>';
    return html;
  }

  function sourceIcon(source) {
    switch (source) {
      case 'bundled': return '\u25CF';
      case 'local': return '\u25C6';
      case 'marketplace': return '\u2605';
      default: return '\u25CB';
    }
  }

  function sourceLabel(source) {
    switch (source) {
      case 'bundled': return 'Bundled';
      case 'local': return 'Local';
      case 'marketplace': return 'Published';
      default: return source || 'Unknown';
    }
  }

  function formatDownloads(n) {
    if (!n && n !== 0) return '0';
    if (n >= 1000000) return (n / 1000000).toFixed(1) + 'M';
    if (n >= 1000) return (n / 1000).toFixed(1) + 'K';
    return String(n);
  }

  function categoryColor(cat) {
    var colors = {
      'Code Review': '#5290e0',
      'Deployment': '#3ec96e',
      'Debugging': '#e05252',
      'Security': '#e0a230',
      'SRE': '#8b5cf6',
      'Testing': '#a855f7',
      'Automation': '#14b8a6'
    };
    return colors[cat] || 'var(--accent)';
  }

  function buildInstalledMap() {
    state.installedNames = {};
    if (SC.state && SC.state.skills) {
      SC.state.skills.forEach(function(s) {
        state.installedNames[s.name] = true;
      });
    }
  }

  function isSkillInstalled(name) {
    return !!state.installedNames[name];
  }

  function initMarketplace() {
    var section = SC.$('#section-skills') || SC.$('#view-skills');
    if (!section) return;

    var existing = SC.$('#marketplace-container');
    if (existing) return;

    buildInstalledMap();

    var container = document.createElement('div');
    container.id = 'marketplace-container';
    container.className = 'marketplace-container';

    container.innerHTML =
      '<div class="marketplace-tabs">' +
        '<button class="marketplace-tab active" data-mode="installed">My Skills</button>' +
        '<button class="marketplace-tab" data-mode="marketplace">Marketplace</button>' +
      '</div>' +
      '<div class="marketplace-search-row" id="marketplace-search-row" style="display:none">' +
        '<div class="marketplace-search-wrap">' +
          '<span class="marketplace-search-icon">\uD83D\uDD0D</span>' +
          '<input type="text" class="marketplace-search" id="marketplace-search-input" placeholder="Search skills..." autocomplete="off">' +
        '</div>' +
        '<select class="marketplace-category-filter" id="marketplace-category-filter">' +
          '<option value="">All Categories</option>' +
        '</select>' +
      '</div>' +
      '<div class="marketplace-featured" id="marketplace-featured" style="display:none"></div>' +
      '<div class="marketplace-category-pills" id="marketplace-category-pills" style="display:none"></div>' +
      '<div class="marketplace-results" id="marketplace-results" style="display:none"></div>';

    var skillList = SC.$('#skill-list') || SC.$('#skill-list-view');
    var skillSearch = SC.$('#skill-search') || SC.$('#skill-search-view');
    var insertBefore = skillSearch || skillList;
    if (insertBefore) {
      section.insertBefore(container, insertBefore);
    } else {
      section.appendChild(container);
    }

    container.querySelectorAll('.marketplace-tab').forEach(function(tab) {
      tab.addEventListener('click', function() {
        switchMode(tab.getAttribute('data-mode'));
      });
    });

    var searchInput = SC.$('#marketplace-search-input');
    if (searchInput) {
      searchInput.addEventListener('input', function() {
        clearTimeout(state.searchTimer);
        state.searchTimer = setTimeout(function() {
          state.query = searchInput.value.trim();
          state.page = 1;
          searchMarketplace();
        }, 300);
      });
    }

    var categoryFilter = SC.$('#marketplace-category-filter');
    if (categoryFilter) {
      categoryFilter.addEventListener('change', function() {
        state.category = categoryFilter.value;
        state.page = 1;
        renderCategoryPills();
        searchMarketplace();
      });
    }

    loadCategories();
    loadFeatured();

    state.initialized = true;
  }

  function switchMode(mode) {
    state.mode = mode;

    document.querySelectorAll('.marketplace-tab').forEach(function(tab) {
      tab.classList.toggle('active', tab.getAttribute('data-mode') === mode);
    });

    var searchRow = SC.$('#marketplace-search-row');
    var featuredEl = SC.$('#marketplace-featured');
    var pillsEl = SC.$('#marketplace-category-pills');
    var resultsEl = SC.$('#marketplace-results');
    var skillSearch = SC.$('#skill-search') || SC.$('#skill-search-view');
    var skillList = SC.$('#skill-list') || SC.$('#skill-list-view');

    if (mode === 'marketplace') {
      if (searchRow) searchRow.style.display = 'flex';
      if (featuredEl) featuredEl.style.display = 'block';
      if (pillsEl) pillsEl.style.display = 'flex';
      if (resultsEl) resultsEl.style.display = 'flex';
      if (skillSearch) skillSearch.style.display = 'none';
      if (skillList) skillList.style.display = 'none';
      buildInstalledMap();
      searchMarketplace();
    } else {
      if (searchRow) searchRow.style.display = 'none';
      if (featuredEl) featuredEl.style.display = 'none';
      if (pillsEl) pillsEl.style.display = 'none';
      if (resultsEl) resultsEl.style.display = 'none';
      if (skillSearch) skillSearch.style.display = '';
      if (skillList) skillList.style.display = '';
      if (typeof SC.renderSkillList === 'function') SC.renderSkillList();
    }
  }

  function loadCategories() {
    fetchJSON('/api/skills/marketplace/categories').then(function(cats) {
      state.categories = Array.isArray(cats) ? cats : [];
      var filter = SC.$('#marketplace-category-filter');
      if (filter) {
        while (filter.options.length > 1) {
          filter.remove(1);
        }
        state.categories.forEach(function(cat) {
          var opt = document.createElement('option');
          opt.value = cat;
          opt.textContent = cat;
          filter.appendChild(opt);
        });
      }
      renderCategoryPills();
    }).catch(function() {
      state.categories = ['Code Review', 'Deployment', 'Debugging', 'Security', 'SRE', 'Testing', 'Automation'];
      renderCategoryPills();
    });
  }

  function renderCategoryPills() {
    var container = SC.$('#marketplace-category-pills');
    if (!container) return;
    container.innerHTML = '';

    var allPill = document.createElement('span');
    allPill.className = 'marketplace-category-pill' + (state.category === '' ? ' active' : '');
    allPill.textContent = 'All';
    allPill.addEventListener('click', function() {
      state.category = '';
      state.page = 1;
      var filter = SC.$('#marketplace-category-filter');
      if (filter) filter.value = '';
      renderCategoryPills();
      searchMarketplace();
    });
    container.appendChild(allPill);

    state.categories.forEach(function(cat) {
      var pill = document.createElement('span');
      pill.className = 'marketplace-category-pill' + (state.category === cat ? ' active' : '');
      pill.textContent = cat;
      pill.addEventListener('click', function() {
        state.category = cat;
        state.page = 1;
        var filter = SC.$('#marketplace-category-filter');
        if (filter) filter.value = cat;
        renderCategoryPills();
        searchMarketplace();
      });
      container.appendChild(pill);
    });
  }

  function loadFeatured() {
    fetchJSON('/api/skills/marketplace/featured').then(function(skills) {
      state.featured = Array.isArray(skills) ? skills : [];
      renderFeatured();
    }).catch(function() {
      state.featured = [];
      renderFeatured();
    });
  }

  function renderFeatured() {
    var container = SC.$('#marketplace-featured');
    if (!container) return;
    container.innerHTML = '';

    if (state.featured.length === 0) return;

    var header = document.createElement('div');
    header.className = 'marketplace-featured-header';
    header.textContent = 'FEATURED';
    container.appendChild(header);

    var wrap = document.createElement('div');
    wrap.className = 'marketplace-featured-wrap';

    var leftArrow = document.createElement('button');
    leftArrow.className = 'marketplace-featured-arrow left';
    leftArrow.innerHTML = '\u2039';
    leftArrow.setAttribute('aria-label', 'Scroll left');

    var carousel = document.createElement('div');
    carousel.className = 'marketplace-featured-carousel';

    state.featured.forEach(function(skill) {
      var card = buildFeaturedCard(skill);
      carousel.appendChild(card);
    });

    var rightArrow = document.createElement('button');
    rightArrow.className = 'marketplace-featured-arrow right';
    rightArrow.innerHTML = '\u203A';
    rightArrow.setAttribute('aria-label', 'Scroll right');

    leftArrow.addEventListener('click', function() {
      carousel.scrollBy({ left: -200, behavior: 'smooth' });
    });
    rightArrow.addEventListener('click', function() {
      carousel.scrollBy({ left: 200, behavior: 'smooth' });
    });

    wrap.appendChild(leftArrow);
    wrap.appendChild(carousel);
    wrap.appendChild(rightArrow);
    container.appendChild(wrap);
  }

  function buildFeaturedCard(skill) {
    var card = document.createElement('div');
    card.className = 'marketplace-card marketplace-featured-card';

    var installed = isSkillInstalled(skill.name);
    var btnClass = installed ? 'marketplace-install-btn installed' : 'marketplace-install-btn';
    var btnText = installed ? 'Installed \u2713' : 'Install';

    card.innerHTML =
      '<div class="marketplace-card-header">' +
        '<span class="marketplace-card-icon">' + sourceIcon(skill.source) + '</span>' +
        '<span class="marketplace-card-name">' + SC.escapeHtml(skill.name) + '</span>' +
        (skill.source === 'marketplace' ? '<span class="marketplace-published-badge">Published</span>' : '') +
      '</div>' +
      '<div class="marketplace-card-desc">' + SC.escapeHtml(skill.description || 'No description') + '</div>' +
      (skill.category ? '<div class="marketplace-card-category-row"><span class="marketplace-category-badge" style="--cat-color:' + categoryColor(skill.category) + '">' + SC.escapeHtml(skill.category) + '</span></div>' : '') +
      '<div class="marketplace-card-footer">' +
        starRating(skill.rating) +
        '<span class="marketplace-downloads">' + formatDownloads(skill.downloads) + '</span>' +
        '<button class="' + btnClass + '" data-name="' + SC.escapeHtml(skill.name) + '">' + btnText + '</button>' +
      '</div>';

    card.querySelector('.marketplace-install-btn').addEventListener('click', function(e) {
      e.stopPropagation();
      var btn = e.currentTarget;
      if (btn.classList.contains('installed')) return;
      installSkill(skill.name, btn);
    });

    card.addEventListener('click', function() {
      showSkillPreview(skill);
    });

    return card;
  }

  function searchMarketplace() {
    if (state.mode !== 'marketplace') return;

    var resultsEl = SC.$('#marketplace-results');
    if (!resultsEl) return;

    state.loading = true;
    resultsEl.innerHTML = '<div class="marketplace-loading"><span class="marketplace-spinner"></span> Searching...</div>';

    var url = '/api/skills/marketplace/search?page=' + state.page + '&pageSize=' + state.pageSize;
    if (state.query) url += '&q=' + encodeURIComponent(state.query);
    if (state.category) url += '&category=' + encodeURIComponent(state.category);

    fetchJSON(url).then(function(result) {
      state.results = result || {};
      state.loading = false;
      renderResults();
    }).catch(function() {
      state.loading = false;
      resultsEl.innerHTML = '<div class="marketplace-error">Failed to load skills. Please try again.</div>';
    });
  }

  function renderResults() {
    var resultsEl = SC.$('#marketplace-results');
    if (!resultsEl) return;
    resultsEl.innerHTML = '';

    var skills = (state.results && state.results.skills) || [];
    if (skills.length === 0) {
      resultsEl.innerHTML = '<div class="marketplace-empty">No skills found</div>';
      return;
    }

    var grid = document.createElement('div');
    grid.className = 'marketplace-results-grid';

    skills.forEach(function(skill) {
      var card = buildResultCard(skill);
      grid.appendChild(card);
    });

    resultsEl.appendChild(grid);

    var total = state.results.total || 0;
    var currentPage = state.results.page || state.page;
    var pageSize = state.results.page_size || state.pageSize;

    if ((currentPage * pageSize) < total) {
      var loadMore = document.createElement('button');
      loadMore.className = 'marketplace-load-more';
      loadMore.textContent = 'Load More';
      loadMore.addEventListener('click', function() {
        state.page++;
        appendResults();
      });
      resultsEl.appendChild(loadMore);
    }
  }

  function appendResults() {
    var resultsEl = SC.$('#marketplace-results');
    if (!resultsEl) return;

    var loadMoreBtn = resultsEl.querySelector('.marketplace-load-more');
    if (loadMoreBtn) {
      loadMoreBtn.textContent = 'Loading...';
      loadMoreBtn.disabled = true;
    }

    var url = '/api/skills/marketplace/search?page=' + state.page + '&pageSize=' + state.pageSize;
    if (state.query) url += '&q=' + encodeURIComponent(state.query);
    if (state.category) url += '&category=' + encodeURIComponent(state.category);

    fetchJSON(url).then(function(result) {
      if (!result || !result.skills || result.skills.length === 0) {
        if (loadMoreBtn) loadMoreBtn.remove();
        return;
      }

      var grid = resultsEl.querySelector('.marketplace-results-grid');
      if (!grid) {
        renderResults();
        return;
      }

      result.skills.forEach(function(skill) {
        grid.appendChild(buildResultCard(skill));
      });

      var total = result.total || 0;
      var currentPage = result.page || state.page;
      var pageSize = result.page_size || state.pageSize;

      if (loadMoreBtn) {
        if ((currentPage * pageSize) >= total) {
          loadMoreBtn.remove();
        } else {
          loadMoreBtn.textContent = 'Load More';
          loadMoreBtn.disabled = false;
        }
      }

      if (typeof SC.applyListStagger === 'function') {
        SC.applyListStagger(grid, '.marketplace-card');
      }
    }).catch(function() {
      if (loadMoreBtn) {
        loadMoreBtn.textContent = 'Load More';
        loadMoreBtn.disabled = false;
      }
    });
  }

  function buildResultCard(skill) {
    var card = document.createElement('div');
    card.className = 'marketplace-card';

    var installed = isSkillInstalled(skill.name);
    var btnClass = installed ? 'marketplace-install-btn installed' : 'marketplace-install-btn';
    var btnText = installed ? 'Installed \u2713' : 'Install';

    var desc = skill.description || 'No description';
    var truncated = desc.length > 90 ? desc.slice(0, 90) + '...' : desc;

    var html =
      '<div class="marketplace-card-header">' +
        '<span class="marketplace-card-icon">' + sourceIcon(skill.source) + '</span>' +
        '<span class="marketplace-card-name">' + SC.escapeHtml(skill.name) + '</span>' +
        (skill.source === 'marketplace' ? '<span class="marketplace-published-badge">Published</span>' : '') +
        (skill.category ? '<span class="marketplace-card-category" style="--cat-color:' + categoryColor(skill.category) + '">' + SC.escapeHtml(skill.category) + '</span>' : '') +
      '</div>' +
      '<div class="marketplace-card-desc">' + SC.escapeHtml(truncated) + '</div>' +
      '<div class="marketplace-card-meta">' +
        '<span class="marketplace-card-author">by ' + SC.escapeHtml(skill.author || 'unknown') + '</span>' +
        (skill.version ? '<span class="marketplace-card-version">v' + SC.escapeHtml(skill.version) + '</span>' : '') +
      '</div>' +
      '<div class="marketplace-card-footer">' +
        starRating(skill.rating) +
        '<span class="marketplace-downloads">' + formatDownloads(skill.downloads) + '</span>' +
        '<button class="' + btnClass + '" data-name="' + SC.escapeHtml(skill.name) + '">' + btnText + '</button>' +
      '</div>';

    if (skill.tags && skill.tags.length > 0) {
      html += '<div class="marketplace-card-tags">';
      skill.tags.forEach(function(tag) {
        html += '<span class="marketplace-card-tag">' + SC.escapeHtml(tag) + '</span>';
      });
      html += '</div>';
    }

    card.innerHTML = html;

    card.querySelector('.marketplace-install-btn').addEventListener('click', function(e) {
      e.stopPropagation();
      var btn = e.currentTarget;
      if (btn.classList.contains('installed')) return;
      installSkill(skill.name, btn);
    });

    card.addEventListener('click', function() {
      showSkillPreview(skill);
    });

    return card;
  }

  function installSkill(name, btn) {
    btn.textContent = 'Installing...';
    btn.disabled = true;
    btn.classList.add('installing');

    postJSON('/api/skills/marketplace/install', { name: name }).then(function(result) {
      if (result && result.success) {
        btn.textContent = 'Installed \u2713';
        btn.classList.remove('installing');
        btn.classList.add('installed');
        btn.disabled = false;
        state.installedNames[name] = true;
        if (typeof SC.toast === 'function') {
          SC.toast('Skill installed: ' + name, 'success');
        }
        if (typeof SC.wsSend === 'function') {
          SC.wsSend('skill_list', {});
        }
      } else {
        btn.textContent = 'Install';
        btn.classList.remove('installing');
        btn.disabled = false;
        if (typeof SC.toast === 'function') {
          SC.toast((result && result.error) || 'Install failed', 'error');
        }
      }
    }).catch(function(err) {
      btn.textContent = 'Install';
      btn.classList.remove('installing');
      btn.disabled = false;
      if (typeof SC.toast === 'function') {
        SC.toast('Install failed: ' + (err.message || 'Network error'), 'error');
      }
    });
  }

  function showSkillPreview(skill) {
    var existing = SC.$('.marketplace-preview-overlay');
    if (existing) existing.remove();

    var overlay = document.createElement('div');
    overlay.className = 'marketplace-preview-overlay';

    var installed = isSkillInstalled(skill.name);

    var tagsHtml = '';
    if (skill.tags && skill.tags.length > 0) {
      tagsHtml = '<div class="marketplace-card-tags">';
      skill.tags.forEach(function(t) {
        tagsHtml += '<span class="marketplace-card-tag">' + SC.escapeHtml(t) + '</span>';
      });
      tagsHtml += '</div>';
    }

    var triggersHtml = '';
    if (skill.triggers && skill.triggers.length > 0) {
      triggersHtml = '<div class="marketplace-preview-triggers"><strong>Triggers:</strong> ' +
        skill.triggers.map(function(t) { return '<code>' + SC.escapeHtml(t) + '</code>'; }).join(' ') +
        '</div>';
    }

    var toolsHtml = '';
    if (skill.tools && skill.tools.length > 0) {
      toolsHtml = '<div class="marketplace-preview-tools"><strong>Tools:</strong> ' +
        skill.tools.map(function(t) { return '<code>' + SC.escapeHtml(t) + '</code>'; }).join(' ') +
        '</div>';
    }

    var contentHtml = '';
    if (skill.content) {
      var preview = skill.content.length > 500 ? skill.content.slice(0, 500) + '...' : skill.content;
      contentHtml = '<div class="marketplace-preview-content"><strong>Content Preview</strong><pre>' + SC.escapeHtml(preview) + '</pre></div>';
    }

    overlay.innerHTML =
      '<div class="marketplace-preview">' +
        '<div class="marketplace-preview-header">' +
          '<h3>' + SC.escapeHtml(skill.name) + '</h3>' +
          '<button class="marketplace-preview-close" aria-label="Close">&times;</button>' +
        '</div>' +
        '<div class="marketplace-preview-body">' +
          '<div class="marketplace-preview-meta">' +
            '<span class="marketplace-preview-author">by ' + SC.escapeHtml(skill.author || 'unknown') + '</span>' +
            (skill.version ? '<span class="marketplace-preview-version">v' + SC.escapeHtml(skill.version) + '</span>' : '') +
            (skill.category ? '<span class="marketplace-preview-category" style="--cat-color:' + categoryColor(skill.category) + '">' + SC.escapeHtml(skill.category) + '</span>' : '') +
            '<span class="marketplace-preview-source">' + sourceLabel(skill.source) + '</span>' +
          '</div>' +
          '<p>' + SC.escapeHtml(skill.description || 'No description available') + '</p>' +
          tagsHtml +
          triggersHtml +
          toolsHtml +
          contentHtml +
          '<div class="marketplace-preview-stats">' +
            starRating(skill.rating) +
            '<span>' + formatDownloads(skill.downloads) + ' downloads</span>' +
          '</div>' +
        '</div>' +
        '<div class="marketplace-preview-actions">' +
          (installed
            ? '<button class="marketplace-install-btn installed" disabled>Installed</button>'
            : '<button class="marketplace-install-btn" data-name="' + SC.escapeHtml(skill.name) + '">Install</button>') +
          (!installed && skill.source === 'local'
            ? '<button class="marketplace-publish-btn" data-name="' + SC.escapeHtml(skill.name) + '">Publish</button>'
            : '') +
        '</div>' +
      '</div>';

    document.body.appendChild(overlay);
    requestAnimationFrame(function() { overlay.classList.add('visible'); });

    overlay.querySelector('.marketplace-preview-close').addEventListener('click', function() {
      closePreview(overlay);
    });
    overlay.addEventListener('click', function(e) {
      if (e.target === overlay) closePreview(overlay);
    });
    document.addEventListener('keydown', function escHandler(e) {
      if (e.key === 'Escape') {
        closePreview(overlay);
        document.removeEventListener('keydown', escHandler);
      }
    });

    var installBtn = overlay.querySelector('.marketplace-install-btn:not(.installed)');
    if (installBtn) {
      installBtn.addEventListener('click', function() {
        var name = installBtn.getAttribute('data-name');
        installSkill(name, installBtn);
      });
    }

    var publishBtn = overlay.querySelector('.marketplace-publish-btn');
    if (publishBtn) {
      publishBtn.addEventListener('click', function() {
        var name = publishBtn.getAttribute('data-name');
        publishBtn.textContent = 'Publishing...';
        publishBtn.disabled = true;
        postJSON('/api/skills/marketplace/publish', { name: name }).then(function(result) {
          if (result && result.success) {
            publishBtn.textContent = 'Published';
            publishBtn.classList.add('published');
            if (typeof SC.toast === 'function') SC.toast('Published skill: ' + name, 'success');
          } else {
            publishBtn.textContent = 'Publish';
            publishBtn.disabled = false;
            if (typeof SC.toast === 'function') SC.toast((result && result.error) || 'Publish failed', 'error');
          }
        }).catch(function() {
          publishBtn.textContent = 'Publish';
          publishBtn.disabled = false;
        });
      });
    }
  }

  function closePreview(overlay) {
    overlay.classList.remove('visible');
    setTimeout(function() { overlay.remove(); }, 200);
  }

  SC.initMarketplace = initMarketplace;
  SC.switchMarketplaceMode = switchMode;
  SC.refreshMarketplace = function() {
    buildInstalledMap();
    if (state.mode === 'marketplace') {
      searchMarketplace();
      loadFeatured();
    }
  };
})();
