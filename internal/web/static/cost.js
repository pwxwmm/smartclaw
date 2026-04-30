// SmartClaw - Cost Intelligence Dashboard
(function() {
  'use strict';

  var refreshTimer = null;
  var currentPeriod = '7d';

  function init() {
    loadCostDashboard();
    bindPeriodTabs();
    startAutoRefresh();
  }

  function startAutoRefresh() {
    if (refreshTimer) clearInterval(refreshTimer);
    refreshTimer = setInterval(loadCostDashboard, 60000);
  }

  function bindPeriodTabs() {
    [{ id: 'cost-period-tabs' }, { id: 'cost-period-tabs-view' }].forEach(function(target) {
      var container = SC.$('#' + target.id);
      if (!container) return;
      container.querySelectorAll('.cost-period-tab').forEach(function(tab) {
        tab.addEventListener('click', function() {
          currentPeriod = tab.dataset.period || '7d';
          container.querySelectorAll('.cost-period-tab').forEach(function(t) { t.classList.remove('active'); });
          tab.classList.add('active');
          if (currentPeriod === 'forecast') {
            loadCostForecast();
          } else {
            loadCostHistory();
          }
        });
      });
    });
  }

  function loadCostDashboard() {
    fetch('/api/cost/dashboard', { credentials: 'same-origin' })
      .then(function(r) { return r.json(); })
      .then(function(data) {
        renderStatCards(data);
        renderCostTrendChart(data);
        renderModelBreakdown(data);
        renderOptimizationTips(data);
        loadCostHistory();
        loadProjectCosts();
      })
      .catch(function(err) {
        console.error('[Cost Dashboard] load error:', err);
      });
  }

  function loadCostHistory() {
    fetch('/api/cost/history?period=' + currentPeriod, { credentials: 'same-origin' })
      .then(function(r) { return r.json(); })
      .then(function(data) {
        renderCostTrendFromHistory(data);
      })
      .catch(function(err) {
        console.error('[Cost History] load error:', err);
      });
  }

  function loadCostForecast() {
    fetch('/api/cost/forecast', { credentials: 'same-origin' })
      .then(function(r) { return r.json(); })
      .then(function(data) {
        renderForecastView(data);
      })
      .catch(function(err) {
        console.error('[Cost Forecast] load error:', err);
      });
  }

  function loadProjectCosts() {
    fetch('/api/cost/projects?days=30', { credentials: 'same-origin' })
      .then(function(r) { return r.json(); })
      .then(function(data) {
        renderProjectCosts(data);
      })
      .catch(function(err) {
        console.error('[Cost Projects] load error:', err);
      });
  }

  function loadWeeklyReport() {
    fetch('/api/cost/report', { credentials: 'same-origin' })
      .then(function(r) { return r.json(); })
      .then(function(data) {
        renderWeeklyReport(data);
      })
      .catch(function(err) {
        console.error('[Cost Report] load error:', err);
      });
  }

  function renderStatCards(data) {
    var cards = [
      { id: 'cost-today', label: 'Today', value: data.today_total, budget: data.budget_fraction },
      { id: 'cost-week', label: 'This Week', value: data.week_total },
      { id: 'cost-month', label: 'This Month', value: data.month_total },
      { id: 'cost-projected', label: 'Projected Monthly', value: data.projected_monthly }
    ];

    SC.state.costDashboard = data;

    cards.forEach(function(card) {
      [card.id, card.id + '-view'].forEach(function(id) {
        var el = SC.$('#' + id);
        if (!el) return;
        var amount = el.querySelector('.cost-stat-amount');
        var label = el.querySelector('.cost-stat-label');
        if (amount) amount.textContent = '$' + (card.value || 0).toFixed(2);
        if (label) label.textContent = card.label;

        var statusClass = 'cost-stat-ok';
        if (card.budget !== undefined) {
          if (card.budget > 0.9) statusClass = 'cost-stat-danger';
          else if (card.budget > 0.7) statusClass = 'cost-stat-warn';
        }
        el.className = 'cost-stat-card ' + statusClass;
      });
    });

    ['cost-budget-bar-fill', 'cost-budget-bar-fill-view'].forEach(function(id) {
      var budgetBar = SC.$('#' + id);
      if (budgetBar) {
        var pct = Math.min((data.budget_fraction || 0) * 100, 100);
        budgetBar.style.width = pct + '%';
        if (pct > 90) budgetBar.className = 'cost-budget-fill cost-budget-danger';
        else if (pct > 70) budgetBar.className = 'cost-budget-fill cost-budget-warn';
        else budgetBar.className = 'cost-budget-fill cost-budget-ok';
      }
    });

    ['cost-budget-text', 'cost-budget-text-view'].forEach(function(id) {
      var budgetText = SC.$('#' + id);
      if (budgetText) {
        budgetText.textContent = 'Budget: $' + (data.budget_remaining || 0).toFixed(2) + ' remaining (' + Math.round((data.budget_fraction || 0) * 100) + '% used)';
      }
    });
  }

  function renderCostTrendChart(data) {
    var history = data.history || [];
    var emptyHtml = '<div class="cost-chart-empty">No cost data yet. Start using the AI to see trends.</div>';
    if (history.length === 0) {
      SC.renderToBoth('cost-trend-chart', 'cost-trend-chart-view', emptyHtml);
    } else {
      SC.renderToBoth('cost-trend-chart', 'cost-trend-chart-view', function(el) {
        renderCostTrendFromHistoryInto(el, history);
      });
    }
  }

  function renderCostTrendFromHistoryInto(chartEl, history) {
    if (!history || history.length === 0) {
      chartEl.innerHTML = '<div class="cost-chart-empty">No cost data for this period.</div>';
      return;
    }

    var dailyMap = {};
    history.forEach(function(h) {
      if (!dailyMap[h.date]) dailyMap[h.date] = 0;
      dailyMap[h.date] += h.cost_usd || 0;
    });

    var dates = Object.keys(dailyMap).sort();
    var maxCost = 0;
    dates.forEach(function(d) { if (dailyMap[d] > maxCost) maxCost = dailyMap[d]; });
    if (maxCost === 0) maxCost = 0.01;

    var budgetLine = null;
    var budgetFraction = SC.state && SC.state.costDashboard ? SC.state.costDashboard.budget_fraction : 0;
    var dailyLimit = SC.state && SC.state.costDashboard && SC.state.costDashboard.budget_remaining
      ? (SC.state.costDashboard.today_total / Math.max(budgetFraction, 0.01))
      : 0;
    if (dailyLimit > 0) budgetLine = dailyLimit;

    var html = '<div class="cost-bar-chart">';
    dates.forEach(function(date, i) {
      var cost = dailyMap[date];
      var heightPct = (cost / maxCost) * 100;
      var shortDate = date.slice(5);
      var barColor = 'var(--accent)';
      if (budgetLine && cost > budgetLine) barColor = 'var(--err)';
      else if (budgetLine && cost > budgetLine * 0.7) barColor = 'var(--warn)';

      html += '<div class="cost-bar-col" style="animation-delay:' + (i * 20) + 'ms">';
      html += '<div class="cost-bar-tooltip">$' + cost.toFixed(3) + '</div>';
      html += '<div class="cost-bar" style="height:' + heightPct + '%;background:' + barColor + '"></div>';
      html += '<div class="cost-bar-label">' + SC.escapeHtml(shortDate) + '</div>';
      html += '</div>';
    });

    if (budgetLine && budgetLine <= maxCost * 1.2) {
      var budgetPct = (budgetLine / maxCost) * 100;
      html += '<div class="cost-budget-line" style="bottom:' + budgetPct + '%"></div>';
    }

    html += '</div>';
    chartEl.innerHTML = html;
  }

  function renderCostTrendFromHistory(history) {
    SC.renderToBoth('cost-trend-chart', 'cost-trend-chart-view', function(el) {
      renderCostTrendFromHistoryInto(el, history);
    });
  }

  function renderForecastView(forecast) {
    var chartEl = SC.$('#cost-trend-chart');
    if (!chartEl) return;

    var dailyForecasts = forecast.daily_forecasts || [];
    if (dailyForecasts.length === 0) {
      chartEl.innerHTML = '<div class="cost-chart-empty">No forecast data available.</div>';
      return;
    }

    var maxCost = 0;
    dailyForecasts.forEach(function(f) {
      if (f.upper_bound > maxCost) maxCost = f.upper_bound;
      if (f.predicted_cost > maxCost) maxCost = f.predicted_cost;
    });
    if (maxCost === 0) maxCost = 0.01;

    var trendArrow = '&#8594;';
    var trendColor = 'var(--accent)';
    if (forecast.trend_direction === 'increasing') { trendArrow = '&#8593;'; trendColor = 'var(--err)'; }
    else if (forecast.trend_direction === 'decreasing') { trendArrow = '&#8595;'; trendColor = '#4caf50'; }

    var riskColors = { low: '#4caf50', moderate: 'var(--warn)', high: '#ff9800', critical: 'var(--err)' };
    var riskColor = riskColors[forecast.risk_level] || 'var(--accent)';

    var html = '<div class="cost-forecast-header">';
    html += '<div class="cost-forecast-risk" style="color:' + riskColor + ';border:1px solid ' + riskColor + ';border-radius:12px;padding:2px 10px;font-size:12px;font-weight:600">' + SC.escapeHtml(forecast.risk_level.toUpperCase()) + '</div>';
    html += '<div class="cost-forecast-trend" style="color:' + trendColor + ';font-size:18px">' + trendArrow + ' ' + SC.escapeHtml(forecast.trend_direction) + '</div>';
    html += '<div class="cost-forecast-sustain" style="font-size:12px;color:var(--muted)">' + SC.escapeHtml(forecast.budget_sustain_days) + ' days budget left</div>';
    html += '</div>';

    html += '<div class="cost-bar-chart cost-forecast-chart">';
    dailyForecasts.forEach(function(f, i) {
      var heightPct = (f.predicted_cost / maxCost) * 100;
      var bandTopPct = (f.upper_bound / maxCost) * 100;
      var bandBotPct = (f.lower_bound / maxCost) * 100;
      var shortDate = f.date.slice(5);
      var opacity = Math.max(0.3, f.confidence);

      html += '<div class="cost-bar-col" style="animation-delay:' + (i * 30) + 'ms">';
      html += '<div class="cost-bar-tooltip">$' + f.predicted_cost.toFixed(3) + ' (&plusmn;' + ((f.upper_bound - f.lower_bound) / 2).toFixed(3) + ')</div>';
      html += '<div class="cost-forecast-band" style="bottom:' + bandBotPct + '%;height:' + (bandTopPct - bandBotPct) + '%;opacity:0.2;background:var(--accent)"></div>';
      html += '<div class="cost-bar" style="height:' + heightPct + '%;background:var(--accent);opacity:' + opacity + '"></div>';
      html += '<div class="cost-bar-label">' + SC.escapeHtml(shortDate) + '</div>';
      html += '</div>';
    });
    html += '</div>';

    html += '<div class="cost-forecast-summary">';
    html += '<div class="cost-forecast-metric"><span>Today Remaining</span><strong>$' + (forecast.today_remaining || 0).toFixed(2) + '</strong></div>';
    html += '<div class="cost-forecast-metric"><span>Week Projected</span><strong>$' + (forecast.week_projected || 0).toFixed(2) + '</strong></div>';
    html += '<div class="cost-forecast-metric"><span>Month Projected</span><strong>$' + (forecast.month_projected || 0).toFixed(2) + '</strong></div>';
    html += '</div>';

    if (forecast.recommendations && forecast.recommendations.length > 0) {
      html += '<div class="cost-forecast-recs">';
      forecast.recommendations.forEach(function(rec) {
        var prioColors = { high: 'var(--err)', medium: 'var(--warn)', low: 'var(--accent)' };
        var prioColor = prioColors[rec.priority] || 'var(--muted)';
        html += '<div class="cost-forecast-rec">';
        html += '<span class="cost-forecast-rec-prio" style="color:' + prioColor + '">' + SC.escapeHtml(rec.priority) + '</span>';
        html += '<div class="cost-forecast-rec-body">';
        html += '<div class="cost-forecast-rec-title">' + SC.escapeHtml(rec.title) + '</div>';
        html += '<div class="cost-forecast-rec-desc">' + SC.escapeHtml(rec.description) + '</div>';
        html += '</div>';
        if (rec.impact > 0) {
          html += '<div class="cost-forecast-rec-impact">~$' + rec.impact.toFixed(2) + '</div>';
        }
        html += '</div>';
      });
      html += '</div>';
    }

    chartEl.innerHTML = html;
  }

  function renderModelBreakdown(data) {
    var breakdown = data.model_breakdown || [];
    if (breakdown.length === 0) {
      SC.renderToBoth('cost-model-breakdown', 'cost-model-breakdown-view', '<div class="cost-chart-empty">No model usage data yet.</div>');
      return;
    }

    var maxCost = 0;
    breakdown.forEach(function(b) { if (b.total_cost > maxCost) maxCost = b.total_cost; });
    if (maxCost === 0) maxCost = 0.01;

    var modelTiers = {
      'claude-3-5-haiku-20241022': 'fast',
      'gpt-4o-mini': 'fast',
      'gemini-2.5-flash': 'fast',
      'claude-sonnet-4-20250514': 'default',
      'claude-sonnet-4-5': 'default',
      'claude-3-5-sonnet-20241022': 'default',
      'gpt-4o': 'default',
      'gemini-2.5-pro': 'default',
      'glm-4-plus': 'default',
      'claude-opus-4-20250514': 'heavy'
    };

    var shortNames = {
      'claude-opus-4-20250514': 'Opus 4',
      'claude-sonnet-4-20250514': 'Sonnet 4',
      'claude-sonnet-4-5': 'Sonnet 4.5',
      'claude-3-5-sonnet-20241022': 'Sonnet 3.5',
      'claude-3-5-haiku-20241022': 'Haiku 3.5',
      'gpt-4o': 'GPT-4o',
      'gpt-4o-mini': 'GPT-4o Mini',
      'gemini-2.5-pro': 'Gemini Pro',
      'gemini-2.5-flash': 'Gemini Flash',
      'glm-4-plus': 'GLM-4 Plus',
      'sre-model': 'SRE Model'
    };

    var html = '';
    breakdown.forEach(function(b) {
      var widthPct = (b.total_cost / maxCost) * 100;
      var tier = modelTiers[b.model] || 'default';
      var name = shortNames[b.model] || b.model;
      var tierClass = 'cost-model-tier-' + tier;

      html += '<div class="cost-model-row">';
      html += '<div class="cost-model-info">';
      html += '<span class="cost-model-name ' + tierClass + '">' + SC.escapeHtml(name) + '</span>';
      html += '<span class="cost-model-queries">' + b.query_count + ' queries</span>';
      html += '</div>';
      html += '<div class="cost-model-bar-wrap">';
      html += '<div class="cost-model-bar ' + tierClass + '" style="width:' + widthPct + '%"></div>';
      html += '</div>';
      html += '<div class="cost-model-meta">';
      html += '<span class="cost-model-cost">$' + b.total_cost.toFixed(3) + '</span>';
      html += '<span class="cost-model-pct">' + b.percentage.toFixed(1) + '%</span>';
      html += '</div>';
      html += '</div>';
    });

    SC.renderToBoth('cost-model-breakdown', 'cost-model-breakdown-view', html);
  }

  function renderProjectCosts(projects) {
    var el = SC.$('#cost-projects');
    if (!el) return;

    if (!projects || projects.length === 0) {
      el.innerHTML = '<div class="cost-chart-empty">No project cost data yet.</div>';
      return;
    }

    var maxCost = 0;
    projects.forEach(function(p) { if (p.total_cost > maxCost) maxCost = p.total_cost; });
    if (maxCost === 0) maxCost = 0.01;

    var html = '<div class="cost-projects-grid">';
    projects.slice(0, 6).forEach(function(p) {
      var widthPct = (p.total_cost / maxCost) * 100;
      html += '<div class="cost-project-card">';
      html += '<div class="cost-project-name">' + SC.escapeHtml(p.project_name) + '</div>';
      html += '<div class="cost-project-cost">$' + p.total_cost.toFixed(3) + '</div>';
      html += '<div class="cost-project-bar-wrap">';
      html += '<div class="cost-project-bar" style="width:' + widthPct + '%"></div>';
      html += '</div>';
      html += '<div class="cost-project-meta">';
      html += '<span>' + p.session_count + ' sessions</span>';
      html += '<span>$' + p.avg_per_session.toFixed(3) + '/session</span>';
      html += '</div>';

      if (p.model_breakdown && p.model_breakdown.length > 0) {
        html += '<div class="cost-project-models">';
        p.model_breakdown.slice(0, 3).forEach(function(m) {
          html += '<span class="cost-project-model-tag">' + SC.escapeHtml(m.model.split('-').slice(0, 2).join('-')) + ' $' + m.total_cost.toFixed(2) + '</span>';
        });
        html += '</div>';
      }

      html += '</div>';
    });
    html += '</div>';

    el.innerHTML = html;
  }

  function renderOptimizationTips(data) {
    var tips = data.optimization_tips || [];
    if (tips.length === 0) {
      SC.renderToBoth('cost-tips', 'cost-tips-view', '<div class="cost-chart-empty">No optimization suggestions yet. Keep using the AI to get personalized tips.</div>');
      return;
    }

    var icons = {
      'model_switch': '&#8635;',
      'cache_usage': '&#9889;',
      'budget_alert': '&#9888;'
    };

    var html = '';
    tips.forEach(function(tip) {
      var icon = icons[tip.type] || '&#9733;';
      html += '<div class="cost-tip">';
      html += '<div class="cost-tip-icon">' + icon + '</div>';
      html += '<div class="cost-tip-body">';
      html += '<div class="cost-tip-title">' + SC.escapeHtml(tip.title) + '</div>';
      html += '<div class="cost-tip-desc">' + SC.escapeHtml(tip.description) + '</div>';
      html += '</div>';
      if (tip.savings_usd > 0) {
        html += '<div class="cost-tip-savings">-$' + tip.savings_usd.toFixed(2) + '/mo</div>';
      }
      html += '</div>';
    });

    html += '<button class="cost-weekly-report-btn" onclick="SC.showWeeklyReport()">&#128196; Weekly Report</button>';

    SC.renderToBoth('cost-tips', 'cost-tips-view', html);
  }

  function renderWeeklyReport(report) {
    var modal = SC.$('#cost-weekly-report-modal');
    if (!modal) {
      modal = document.createElement('div');
      modal.id = 'cost-weekly-report-modal';
      modal.className = 'cost-modal-overlay';
      document.body.appendChild(modal);
    }

    var vsArrow = report.vs_previous_week >= 0 ? '&#8593;' : '&#8595;';
    var vsColor = report.vs_previous_week >= 0 ? 'var(--err)' : '#4caf50';
    var topModelName = report.top_model ? report.top_model.model : 'N/A';
    var topProjectName = report.top_project ? report.top_project.project_name : 'N/A';
    var topProjectCost = report.top_project ? report.top_project.total_cost.toFixed(2) : '0.00';

    var html = '<div class="cost-modal-content">';
    html += '<div class="cost-modal-header">';
    html += '<h3>Weekly Cost Report</h3>';
    html += '<button class="cost-modal-close" onclick="SC.closeWeeklyReport()">&times;</button>';
    html += '</div>';

    html += '<div class="cost-modal-body">';
    html += '<div class="cost-report-period">' + SC.escapeHtml(report.period) + '</div>';

    html += '<div class="cost-report-metrics">';
    html += '<div class="cost-report-metric"><label>Total Cost</label><span>$' + report.total_cost.toFixed(2) + '</span></div>';
    html += '<div class="cost-report-metric"><label>vs Previous Week</label><span style="color:' + vsColor + '">' + vsArrow + ' ' + Math.abs(report.vs_previous_week).toFixed(1) + '%</span></div>';
    html += '<div class="cost-report-metric"><label>Top Model</label><span>' + SC.escapeHtml(topModelName) + '</span></div>';
    html += '<div class="cost-report-metric"><label>Top Project</label><span>' + SC.escapeHtml(topProjectName) + ' ($' + topProjectCost + ')</span></div>';
    html += '<div class="cost-report-metric"><label>Savings from Downgrade</label><span style="color:#4caf50">-$' + report.savings_realized.toFixed(2) + '</span></div>';
    html += '</div>';

    if (report.tips && report.tips.length > 0) {
      html += '<div class="cost-report-tips">';
      html += '<h4>Optimization Tips</h4>';
      report.tips.forEach(function(tip) {
        html += '<div class="cost-report-tip">';
        html += '<strong>' + SC.escapeHtml(tip.title) + '</strong>';
        html += '<p>' + SC.escapeHtml(tip.description) + '</p>';
        html += '</div>';
      });
      html += '</div>';
    }

    if (report.forecast.daily_forecasts && report.forecast.daily_forecasts.length > 0) {
      html += '<div class="cost-report-forecast">';
      html += '<h4>Forecast: ' + SC.escapeHtml(report.forecast.trend_direction) + ' (risk: ' + SC.escapeHtml(report.forecast.risk_level) + ')</h4>';
      html += '<div class="cost-report-forecast-metrics">';
      html += '<span>Week: $' + report.forecast.week_projected.toFixed(2) + '</span>';
      html += '<span>Month: $' + report.forecast.month_projected.toFixed(2) + '</span>';
      html += '</div>';
      html += '</div>';
    }

    html += '</div></div>';
    modal.innerHTML = html;
    modal.style.display = 'flex';
  }

  SC.initCostDashboard = init;
  SC.showWeeklyReport = function() { loadWeeklyReport(); };
  SC.closeWeeklyReport = function() {
    var modal = SC.$('#cost-weekly-report-modal');
    if (modal) modal.style.display = 'none';
  };
})();
