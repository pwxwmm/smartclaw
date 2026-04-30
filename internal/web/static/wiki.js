// SmartClaw - Wiki
(function() {
  'use strict';

  function renderWikiResults(data) {
    SC.renderToBoth('wiki-pages', 'wiki-pages-view', '');
    if (!data) return;
    const results = data.results || [];
    if (results.length === 0) {
      SC.renderToBoth('wiki-pages', 'wiki-pages-view', '<div class="loading-placeholder" style="color:var(--tx-2)">No wiki results</div>');
      return;
    }
    SC.renderToBoth('wiki-pages', 'wiki-pages-view', function(el) {
      results.forEach(page => { el.appendChild(createWikiPageEl(page)); });
    });
  }

  function createWikiPageEl(page) {
    const el = document.createElement('div');
    el.className = 'wiki-page';
    el.dataset.pageId = page.id || '';
    const title = page.title || page.name || 'Untitled';
    const meta = page.path || page.url || '';
    el.innerHTML = `
      <div class="wiki-page-title">${SC.escapeHtml(title)}</div>
      ${meta ? `<div class="wiki-page-meta">${SC.escapeHtml(meta)}</div>` : ''}
    `;
    el.addEventListener('click', () => {
      if (page.id) SC.wsSend('wiki_page_content', { page_id: page.id });
    });
    return el;
  }

  function renderWikiPages() {
    SC.renderToBoth('wiki-pages', 'wiki-pages-view', '');

    var statusText = '';
    var statusClass = 'wiki-status';
    if (SC.state.wikiEnabled) {
      statusText = 'Connected';
      statusClass = 'wiki-status connected';
    } else {
      statusText = 'Not configured';
    }
    var statusEl = SC.$('#wiki-status');
    if (statusEl) { statusEl.textContent = statusText; statusEl.className = statusClass; }
    var viewStatusEl = SC.$('#wiki-status-view');
    if (viewStatusEl) { viewStatusEl.textContent = statusText; viewStatusEl.className = statusClass; }

    if (!SC.state.wikiEnabled) {
      SC.renderToBoth('wiki-pages', 'wiki-pages-view', '<div class="wiki-not-configured">Wiki is not configured. Enable it in your project settings.</div>');
      return;
    }
    if (!SC.state.wikiPages || SC.state.wikiPages.length === 0) {
      SC.renderToBoth('wiki-pages', 'wiki-pages-view', '<div class="loading-placeholder" style="color:var(--tx-2)">No wiki pages</div>');
      return;
    }
    SC.renderToBoth('wiki-pages', 'wiki-pages-view', function(el) {
      SC.state.wikiPages.forEach(page => { el.appendChild(createWikiPageEl(page)); });
      if (typeof SC.applyListStagger === 'function') SC.applyListStagger(el, '.wiki-page');
    });
  }

  function renderWikiPageContent(data) {
    const containers = [SC.$('#wiki-pages'), SC.$('#wiki-pages-view')];
    if (!data) return;
    if (data.enabled === false) {
      SC.toast('Wiki not configured', 'warning');
      return;
    }
    const page = data.page;
    if (!page) {
      SC.toast('Page not found', 'error');
      return;
    }
    const tags = (page.tags || []).map(t =>
      `<span class="wiki-page-tag">${SC.escapeHtml(t)}</span>`
    ).join('');
    const contentHtml = SC.renderMarkdown(page.content || '');
    const pageDetailHtml = `
      <div class="wiki-page-detail">
        <div class="wiki-page-back wiki-back-btn">
          <span class="wiki-back-arrow">←</span> Back to pages
        </div>
        <div class="wiki-page-detail-title">${SC.escapeHtml(page.title || 'Untitled')}</div>
        ${tags ? `<div class="wiki-page-tags">${tags}</div>` : ''}
        ${page.updated_at ? `<div class="wiki-page-date">Updated: ${SC.escapeHtml(page.updated_at)}</div>` : ''}
        <div class="wiki-page-content markdown-body">${contentHtml}</div>
      </div>
    `;

    containers.forEach(function(container) {
      if (!container) return;
      container.innerHTML = pageDetailHtml;
      var backBtn = container.querySelector('.wiki-back-btn');
      if (backBtn) {
        backBtn.addEventListener('click', () => {
          renderWikiPages();
        });
      }
      var contentEl = container.querySelector('.wiki-page-content');
      if (contentEl) SC.postRenderMarkdown(contentEl);
    });
  }

  SC.renderWikiResults = renderWikiResults;
  SC.renderWikiPages = renderWikiPages;
  SC.renderWikiPageContent = renderWikiPageContent;
})();
