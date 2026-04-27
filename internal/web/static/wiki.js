// SmartClaw - Wiki
(function() {
  'use strict';

  function renderWikiResults(data) {
    const container = SC.$('#wiki-pages');
    container.innerHTML = '';
    if (!data) return;
    const results = data.results || [];
    if (results.length === 0) {
      container.innerHTML = '<div class="loading-placeholder" style="color:var(--tx-2)">No wiki results</div>';
      return;
    }
    results.forEach(page => {
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
      container.appendChild(el);
    });
  }

  function renderWikiPages() {
    const container = SC.$('#wiki-pages');
    const statusEl = SC.$('#wiki-status');
    container.innerHTML = '';
    if (statusEl) {
      if (SC.state.wikiEnabled) {
        statusEl.textContent = 'Connected';
        statusEl.className = 'wiki-status connected';
      } else {
        statusEl.textContent = 'Not configured';
        statusEl.className = 'wiki-status';
      }
    }
    if (!SC.state.wikiEnabled) {
      container.innerHTML = '<div class="wiki-not-configured">Wiki is not configured. Enable it in your project settings.</div>';
      return;
    }
    if (!SC.state.wikiPages || SC.state.wikiPages.length === 0) {
      container.innerHTML = '<div class="loading-placeholder" style="color:var(--tx-2)">No wiki pages</div>';
      return;
    }
    SC.state.wikiPages.forEach(page => {
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
      container.appendChild(el);
    });
  }

  function renderWikiPageContent(data) {
    const container = SC.$('#wiki-pages');
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
    container.innerHTML = `
      <div class="wiki-page-detail">
        <div class="wiki-page-back" id="wiki-back-btn">
          <span class="wiki-back-arrow">←</span> Back to pages
        </div>
        <div class="wiki-page-detail-title">${SC.escapeHtml(page.title || 'Untitled')}</div>
        ${tags ? `<div class="wiki-page-tags">${tags}</div>` : ''}
        ${page.updated_at ? `<div class="wiki-page-date">Updated: ${SC.escapeHtml(page.updated_at)}</div>` : ''}
        <div class="wiki-page-content markdown-body">${contentHtml}</div>
      </div>
    `;
    const backBtn = SC.$('#wiki-back-btn');
    if (backBtn) {
      backBtn.addEventListener('click', () => {
        renderWikiPages();
      });
    }
    const contentEl = container.querySelector('.wiki-page-content');
    if (contentEl) SC.postRenderMarkdown(contentEl);
  }

  SC.renderWikiResults = renderWikiResults;
  SC.renderWikiPages = renderWikiPages;
  SC.renderWikiPageContent = renderWikiPageContent;
})();
