(function() {
  'use strict';

  var AGENT_META = {
    network:   { label: 'Network',   icon: '🌐', color: '#3b82f6' },
    database:  { label: 'Database',  icon: '🗄️', color: '#8b5cf6' },
    infra:     { label: 'Infra',     icon: '🏗️', color: '#f59e0b' },
    app:       { label: 'App',       icon: '📱', color: '#10b981' },
    security:  { label: 'Security',  icon: '🔒', color: '#ef4444' },
    reasoning: { label: 'Reasoning', icon: '🧠', color: '#ec4899' },
    training:  { label: 'Training',  icon: '🎯', color: '#f97316' },
    inference: { label: 'Inference', icon: '⚡', color: '#06b6d4' }
  };

  var state = {
    sessions: [],
    activeSessionId: null,
    agentStatuses: {},
    findings: [],
    timeline: [],
    blackboard: { entries: [], hypotheses: [], sharedFacts: [] },
    handoffs: []
  };

  function agentMeta(type) {
    return AGENT_META[type] || { label: type, icon: '🔍', color: '#6b7280' };
  }

  function statusLabel(s) {
    var map = { spawning: '启动中', running: '排查中', waiting: '等待中', complete: '已完成', failed: '失败', active: '进行中', resolved: '已解决', closed: '已关闭', paused: '暂停' };
    return map[s] || s;
  }

  function phaseLabel(evt) {
    var map = {
      dispatch_plan: '📋 调度计划',
      phase1_complete: '✅ Phase 1 并行排查完成',
      phase2_started: '🧠 Phase 2 推理启动',
      phase2_complete: '✅ Phase 2 推理完成',
      phase3_started: '🔍 Phase 3 扩展排查启动',
      phase3_complete: '✅ Phase 3 扩展排查完成',
      phase4_started: '🎯 Phase 4 终极分析启动',
      phase4_complete: '✅ Phase 4 终极分析完成',
      agent_running: '排查中',
      agent_complete: '已完成',
      agent_failed: '排查失败',
      agent_spawning: '启动中',
      phase3_agent_started: 'Phase 3 专家启动',
      war_room_started: '🚀 War Room 启动',
      session_created: '会话创建',
      session_closed: '会话关闭',
      finding_submitted: '发现提交',
      task_assigned: '任务分配',
      handoff_request: '🤝 请求协助',
      handoff_response: '🤝 协助回复',
      confidence_evolved: '📊 置信度变化',
      blackboard_entry_added: '📝 共享观察',
      hypothesis_added: '💡 假设提出',
      shared_fact_added: '✓ 共享事实'
    };
    return map[evt] || evt;
  }

  function statusClass(s) {
    if (s === 'running' || s === 'active') return 'wr-status-running';
    if (s === 'complete' || s === 'resolved' || s === 'closed') return 'wr-status-done';
    if (s === 'failed') return 'wr-status-failed';
    return 'wr-status-idle';
  }

  function timeAgo(ts) {
    if (!ts) return '';
    var d = new Date(ts);
    var sec = Math.floor((Date.now() - d.getTime()) / 1000);
    if (sec < 60) return sec + 's ago';
    if (sec < 3600) return Math.floor(sec / 60) + 'm ago';
    return Math.floor(sec / 3600) + 'h ago';
  }

  function renderPhaseIndicator(session) {
    var timeline = session.timeline || [];
    var hasPhase1Complete = false;
    var hasPhase2Start = false;
    var hasPhase2Complete = false;
    var hasPhase3Start = false;
    var hasPhase3Complete = false;
    var hasPhase4Start = false;
    var hasPhase4Complete = false;
    for (var i = 0; i < timeline.length; i++) {
      if (timeline[i].event === 'phase1_complete') hasPhase1Complete = true;
      if (timeline[i].event === 'phase2_started') hasPhase2Start = true;
      if (timeline[i].event === 'phase2_complete') hasPhase2Complete = true;
      if (timeline[i].event === 'phase3_started') hasPhase3Start = true;
      if (timeline[i].event === 'phase3_complete') hasPhase3Complete = true;
      if (timeline[i].event === 'phase4_started') hasPhase4Start = true;
      if (timeline[i].event === 'phase4_complete') hasPhase4Complete = true;
    }

    var p1DotCls = 'wr-phase-step-dot' + (hasPhase1Complete ? ' wr-phase-dot-done' : ' wr-phase-dot-active');
    var p1LabelCls = 'wr-phase-step-label' + (hasPhase1Complete ? ' wr-phase-label-done' : ' wr-phase-label-active');
    var p1Label = hasPhase1Complete ? 'Phase 1 ✓' : 'Phase 1';

    var line1Cls = 'wr-phase-step-line' + (hasPhase1Complete ? ' wr-phase-line-done' : '');

    var p2DotCls = 'wr-phase-step-dot';
    var p2LabelCls = 'wr-phase-step-label';
    var p2Label = 'Phase 2';
    if (hasPhase2Complete) {
      p2DotCls += ' wr-phase-dot-done';
      p2LabelCls += ' wr-phase-label-done';
      p2Label = 'Phase 2 ✓';
    } else if (hasPhase2Start) {
      p2DotCls += ' wr-phase-dot-active';
      p2LabelCls += ' wr-phase-label-active';
    }

    var line2Cls = 'wr-phase-step-line' + (hasPhase2Complete ? ' wr-phase-line-done' : '');

    var p3DotCls = 'wr-phase-step-dot';
    var p3LabelCls = 'wr-phase-step-label';
    var p3Label = 'Phase 3';
    if (hasPhase3Complete) {
      p3DotCls += ' wr-phase-dot-done';
      p3LabelCls += ' wr-phase-label-done';
      p3Label = 'Phase 3 ✓';
    } else if (hasPhase3Start) {
      p3DotCls += ' wr-phase-dot-active';
      p3LabelCls += ' wr-phase-label-active';
    }

    var line3Cls = 'wr-phase-step-line' + (hasPhase3Complete ? ' wr-phase-line-done' : '');

    var p4DotCls = 'wr-phase-step-dot';
    var p4LabelCls = 'wr-phase-step-label';
    var p4Label = 'Phase 4';
    if (hasPhase4Complete) {
      p4DotCls += ' wr-phase-dot-done';
      p4LabelCls += ' wr-phase-label-done';
      p4Label = 'Phase 4 ✓';
    } else if (hasPhase4Start) {
      p4DotCls += ' wr-phase-dot-active';
      p4LabelCls += ' wr-phase-label-active';
    }

    return '<div class="wr-phase-stepper">' +
      '<div class="wr-phase-step"><div class="' + p1DotCls + '"></div><span class="' + p1LabelCls + '">' + p1Label + '</span></div>' +
      '<div class="' + line1Cls + '"></div>' +
      '<div class="wr-phase-step"><div class="' + p2DotCls + '"></div><span class="' + p2LabelCls + '">' + p2Label + '</span></div>' +
      '<div class="' + line2Cls + '"></div>' +
      '<div class="wr-phase-step"><div class="' + p3DotCls + '"></div><span class="' + p3LabelCls + '">' + p3Label + '</span></div>' +
      '<div class="' + line3Cls + '"></div>' +
      '<div class="wr-phase-step"><div class="' + p4DotCls + '"></div><span class="' + p4LabelCls + '">' + p4Label + '</span></div>' +
    '</div>';
  }

  function renderWarRoomView() {
    var container = SC.$('#view-warroom');
    if (!container) return;

    if (state.activeSessionId) {
      renderActiveSession(container);
    } else {
      renderSessionList(container);
    }
  }

  function renderSessionList(container) {
    var sessionsHtml = '';
    if (state.sessions.length === 0) {
      sessionsHtml = '<div class="wr-empty"><svg width="56" height="56" viewBox="0 0 24 24" fill="none" stroke="var(--tx-2)" stroke-width="1"><circle cx="12" cy="12" r="10" opacity=".3"/><path d="M12 6v6l4 2" stroke-width="1.5" stroke-linecap="round"/><circle cx="12" cy="12" r="3" fill="var(--accent)" opacity=".3"/><circle cx="12" cy="12" r="1.5" fill="var(--accent)" opacity=".6"/></svg><p>暂无 War Room 会话</p><p class="wr-empty-sub">点击上方「新建」启动多专家协同排查</p></div>';
    } else {
      state.sessions.forEach(function(s) {
        var sc = statusClass(s.status);
        var agentDots = '';
        if (s.agents && s.agents.length > 0) {
          agentDots = '<span class="wr-agent-dots">';
          s.agents.forEach(function(a) {
            var m = agentMeta(a.agent_type);
            agentDots += '<span class="wr-agent-dot" style="background:' + m.color + '" title="' + SC.escapeHtml(m.label) + '"></span>';
          });
          agentDots += '</span>';
        }
        sessionsHtml += '<div class="wr-session-card liquid-glass" data-session-id="' + SC.escapeHtml(s.id) + '">' +
          '<div class="wr-session-card-head">' +
            '<span class="wr-session-title">' + SC.escapeHtml(s.title) + '</span>' +
            (s.context && s.context.auto_triggered ? '<span class="wr-auto-badge">🤖 Auto</span>' : '') +
            '<span class="wr-status-badge ' + sc + '">' + statusLabel(s.status) + '</span>' +
          '</div>' +
          '<div class="wr-session-card-desc">' + SC.escapeHtml(s.description || '') + '</div>' +
          '<div class="wr-session-card-meta">' +
            agentDots +
            '<span>' + (s.findings ? s.findings.length : 0) + '条发现</span>' +
            '<span>' + timeAgo(s.created_at) + '</span>' +
          '</div>' +
        '</div>';
      });
    }

    container.innerHTML =
      '<div class="wr-header">' +
        '<div class="wr-header-left">' +
          '<h2 class="wr-title">War Room</h2>' +
          '<span class="wr-subtitle">多专家协同故障排查</span>' +
        '</div>' +
        '<div class="wr-header-actions">' +
          '<button class="wr-btn wr-btn-primary" id="wr-new-btn">+ 新建</button>' +
        '</div>' +
      '</div>' +
      '<div class="wr-session-list" id="wr-session-list">' + sessionsHtml + '</div>' +
      '<div class="wr-new-form" id="wr-new-form" style="display:none"></div>';
  }

  function renderNewForm() {
    var form = SC.$('#wr-new-form');
    var list = SC.$('#wr-session-list');
    if (!form || !list) return;

    list.style.display = 'none';
    form.style.display = 'block';

    var agentsCheckboxes = '';
    var agentTypes = Object.keys(AGENT_META);
    agentTypes.forEach(function(type) {
      var m = agentMeta(type);
      agentsCheckboxes += '<label class="wr-agent-check">' +
        '<input type="checkbox" name="wr-agent-type" value="' + type + '" checked>' +
        '<span class="wr-agent-check-dot" style="background:' + m.color + '"></span>' +
        '<span class="wr-agent-check-label">' + m.icon + ' ' + m.label + '</span>' +
      '</label>';
    });

    form.innerHTML =
      '<div class="wr-form-card liquid-glass">' +
        '<div class="wr-form-head">' +
          '<h3>新建 War Room</h3>' +
          '<button class="wr-btn wr-btn-ghost" id="wr-form-cancel">取消</button>' +
        '</div>' +
        '<div class="wr-form-body">' +
          '<div class="wr-form-group">' +
            '<label>标题</label>' +
            '<input type="text" id="wr-form-title" placeholder="例: node3 训练任务 OOM 排查" class="wr-input">' +
          '</div>' +
          '<div class="wr-form-group">' +
            '<label>描述</label>' +
            '<textarea id="wr-form-desc" placeholder="描述故障现象、影响范围..." class="wr-input" rows="3"></textarea>' +
          '</div>' +
          '<div class="wr-form-group">' +
            '<label>关联事件 ID（可选）</label>' +
            '<input type="text" id="wr-form-incident" placeholder="INC-xxxx" class="wr-input">' +
          '</div>' +
          '<div class="wr-form-group">' +
            '<label>选择专家</label>' +
            '<div class="wr-agent-checks">' + agentsCheckboxes + '</div>' +
          '</div>' +
          '<button class="wr-btn wr-btn-primary wr-btn-block" id="wr-form-submit">启动 War Room</button>' +
        '</div>' +
      '</div>';

    SC.$('#wr-form-cancel').addEventListener('click', function() {
      form.style.display = 'none';
      list.style.display = '';
    });

    SC.$('#wr-form-submit').addEventListener('click', function() {
      var title = SC.$('#wr-form-title').value.trim();
      var desc = SC.$('#wr-form-desc').value.trim();
      var incident = SC.$('#wr-form-incident').value.trim();
      if (!title || !desc) return;

      var selectedAgents = [];
      SC.$$('.wr-agent-check input:checked').forEach(function(cb) {
        selectedAgents.push(cb.value);
      });
      if (selectedAgents.length === 0) return;

      SC.wsSend('warroom_start', {
        title: title,
        description: desc,
        incident_id: incident,
        agent_types: selectedAgents
      });
    });
  }

  function renderActiveSession(container) {
    var session = null;
    for (var i = 0; i < state.sessions.length; i++) {
      if (state.sessions[i].id === state.activeSessionId) {
        session = state.sessions[i];
        break;
      }
    }
    if (!session) {
      state.activeSessionId = null;
      renderSessionList(container);
      return;
    }

    var bb = session.blackboard || state.blackboard;

    var agentsHtml = '';
    var agentTypes = session.agents || [];
    agentTypes.forEach(function(a) {
      var m = agentMeta(a.agent_type);
      var sc = statusClass(state.agentStatuses[a.agent_type] || a.status);
      var findingsCount = a.findings ? a.findings.length : 0;
      var handoffHtml = '';
      for (var hi = 0; hi < state.handoffs.length; hi++) {
        var ho = state.handoffs[hi];
        if (ho.from_agent === a.agent_type) {
          var toM = agentMeta(ho.to_agent);
          handoffHtml = '<span class="wr-agent-handoff" title="请求 ' + toM.label + ' 协助"><span class="wr-handoff-arrow">→</span>' + toM.icon + '</span>';
        } else if (ho.to_agent === a.agent_type) {
          var fromM = agentMeta(ho.from_agent);
          handoffHtml = '<span class="wr-agent-handoff" title="协助 ' + fromM.label + '"><span class="wr-handoff-arrow">←</span>' + fromM.icon + '</span>';
        }
      }
      agentsHtml += '<div class="wr-agent-card liquid-glass-light ' + sc + '" data-agent-type="' + SC.escapeHtml(a.agent_type) + '">' +
        '<div class="wr-agent-icon" style="background:' + m.color + '20;color:' + m.color + '">' + m.icon + '</div>' +
        '<div class="wr-agent-info">' +
          '<div class="wr-agent-name">' + SC.escapeHtml(m.label) + '</div>' +
          '<div class="wr-agent-status ' + sc + '">' + statusLabel(state.agentStatuses[a.agent_type] || a.status) + '</div>' +
        '</div>' +
        '<div class="wr-agent-findings-count">' + findingsCount + '</div>' +
        handoffHtml +
      '</div>';
    });

    var findingsHtml = '';
    var allFindings = state.findings.length > 0 ? state.findings : (session.findings || []);
    allFindings.forEach(function(f) {
      var m = agentMeta(f.agent_type);
      var confidencePct = Math.round((f.confidence || 0) * 100);
      var isRootCause = f.category === 'root_cause';
      var findingCls = 'wr-finding liquid-glass-light' + (isRootCause ? ' wr-finding-root-cause' : '');
      var barColor = confidencePct >= 80 ? '#22c55e' : (confidencePct >= 50 ? '#f59e0b' : '#ef4444');
      var crossRefHtml = '';
      if (f.cross_references && f.cross_references.length > 0) {
        crossRefHtml = '<div class="wr-finding-crossrefs">';
        f.cross_references.forEach(function(cr) {
          var crMeta = agentMeta(cr.referenced_by);
          var crIcon = cr.agrees ? '✅' : '❌';
          var crCls = cr.agrees ? 'wr-crossref-agree' : 'wr-crossref-disagree';
          crossRefHtml += '<span class="wr-crossref ' + crCls + '">' + crIcon + ' ' + crMeta.icon + ' ' + SC.escapeHtml(crMeta.label) + '</span>';
        });
        crossRefHtml += '</div>';
      }
      findingsHtml += '<div class="' + findingCls + '">' +
        '<div class="wr-finding-head">' +
          '<span class="wr-finding-agent" style="color:' + m.color + '">' + m.icon + ' ' + SC.escapeHtml(m.label) + '</span>' +
          '<span class="wr-finding-category">' + SC.escapeHtml(f.category || '') + '</span>' +
          '<span class="wr-finding-confidence">' + confidencePct + '%</span>' +
          '<span class="wr-finding-confidence-bar"><span class="wr-finding-confidence-bar-fill" style="width:' + confidencePct + '%;background:' + barColor + '"></span></span>' +
        '</div>' +
        '<div class="wr-finding-title">' + SC.escapeHtml(f.title || '') + '</div>' +
        '<div class="wr-finding-desc">' + SC.escapeHtml(f.description || '') + '</div>' +
        crossRefHtml +
      '</div>';
    });

    var timelineHtml = '';
    var allTimeline = state.timeline.length > 0 ? state.timeline : (session.timeline || []);
    allTimeline.slice().reverse().forEach(function(e) {
      var m = agentMeta(e.agent_type);
      timelineHtml += '<div class="wr-timeline-entry">' +
        '<div class="wr-timeline-dot" style="background:' + (e.agent_type ? m.color : (e.event && e.event.startsWith('phase') ? '#8b5cf6' : 'var(--tx-2)')) + '"></div>' +
        '<div class="wr-timeline-content">' +
          '<span class="wr-timeline-event">' + SC.escapeHtml(phaseLabel(e.event || e.event)) + '</span>' +
          (e.details ? ' <span class="wr-timeline-details">' + SC.escapeHtml(e.details) + '</span>' : '') +
          '<span class="wr-timeline-time">' + timeAgo(e.timestamp) + '</span>' +
        '</div>' +
      '</div>';
    });

    var blackboardHtml = renderBlackboardContent(bb);

    container.innerHTML =
      '<div class="wr-header">' +
        '<div class="wr-header-left">' +
          '<button class="wr-btn wr-btn-ghost" id="wr-back-btn">← 返回</button>' +
          '<h2 class="wr-title">' + SC.escapeHtml(session.title) + '</h2>' +
          '<span class="wr-status-badge ' + statusClass(session.status) + '">' + statusLabel(session.status) + '</span>' +
          renderPhaseIndicator(session) +
        '</div>' +
        '<div class="wr-header-actions">' +
          (session.status === 'active' ? '<button class="wr-btn wr-btn-danger" id="wr-stop-btn">结束排查</button>' : '') +
        '</div>' +
      '</div>' +
      '<div class="wr-session-body">' +
        '<div class="wr-agents-panel">' +
          '<h3 class="wr-section-title">专家团队</h3>' +
          '<div class="wr-agents-grid">' + agentsHtml + '</div>' +
          (session.status === 'active' ? renderAssignTaskForm(session) : '') +
        '</div>' +
        '<div class="wr-findings-panel">' +
          '<h3 class="wr-section-title">发现 (' + allFindings.length + ')</h3>' +
          '<div class="wr-findings-list">' + (findingsHtml || '<div class="wr-empty-sm">暂无发现</div>') + '</div>' +
        '</div>' +
        '<div class="wr-blackboard-panel">' +
          '<h3 class="wr-section-title">共享黑板</h3>' +
          blackboardHtml +
        '</div>' +
        '<div class="wr-timeline-panel">' +
          '<h3 class="wr-section-title">时间线</h3>' +
          '<div class="wr-timeline-list">' + (timelineHtml || '<div class="wr-empty-sm">暂无记录</div>') + '</div>' +
        '</div>' +
      '</div>';

    SC.$('#wr-back-btn').addEventListener('click', function() {
      state.activeSessionId = null;
      state.agentStatuses = {};
      state.findings = [];
      state.timeline = [];
      state.blackboard = { entries: [], hypotheses: [], sharedFacts: [] };
      state.handoffs = [];
      SC.wsSend('warroom_list', {});
      renderSessionList(container);
    });

    if (session.status === 'active') {
      var stopBtn = SC.$('#wr-stop-btn');
      if (stopBtn) {
        stopBtn.addEventListener('click', function() {
          SC.wsSend('warroom_stop', { session_id: state.activeSessionId });
        });
      }

      var assignBtn = SC.$('#wr-assign-btn');
      if (assignBtn) {
        assignBtn.addEventListener('click', function() {
          var agentType = SC.$('#wr-assign-agent').value;
          var task = SC.$('#wr-assign-task').value.trim();
          if (!agentType || !task) return;
          SC.wsSend('warroom_assign_task', {
            session_id: state.activeSessionId,
            agent_type: agentType,
            task: task
          });
          SC.$('#wr-assign-task').value = '';
        });
      }

      var broadcastBtn = SC.$('#wr-broadcast-btn');
      if (broadcastBtn) {
        broadcastBtn.addEventListener('click', function() {
          var msg = SC.$('#wr-broadcast-msg').value.trim();
          if (!msg) return;
          SC.wsSend('warroom_broadcast', {
            session_id: state.activeSessionId,
            message: msg
          });
          SC.$('#wr-broadcast-msg').value = '';
        });
      }
    }

    SC.$$('.wr-session-card').forEach(function(card) {
      card.addEventListener('click', function() {
        state.activeSessionId = card.dataset.sessionId;
        SC.wsSend('warroom_status', { session_id: state.activeSessionId });
      });
    });
  }

  function renderBlackboardContent(bb) {
    var entries = bb.entries || [];
    var hypotheses = bb.hypotheses || [];
    var sharedFacts = bb.sharedFacts || [];

    if (entries.length === 0 && hypotheses.length === 0 && sharedFacts.length === 0) {
      return '<div class="wr-empty-sm">暂无共享数据</div>';
    }

    var html = '';

    if (sharedFacts.length > 0) {
      html += '<div class="wr-bb-section"><div class="wr-bb-section-label">✓ 共享事实</div>';
      sharedFacts.forEach(function(f) {
        var srcM = agentMeta(f.source);
        var confPct = Math.round((f.confidence || 0) * 100);
        var confirmedCount = f.confirmed_by ? f.confirmed_by.length : 0;
        html += '<div class="wr-bb-shared-fact liquid-glass-light">' +
          '<div class="wr-bb-fact-content">' + SC.escapeHtml(f.content) + '</div>' +
          '<div class="wr-bb-fact-meta">' +
            '<span style="color:' + srcM.color + '">' + srcM.icon + ' ' + SC.escapeHtml(srcM.label) + '</span>' +
            '<span class="wr-bb-fact-confirmed">' + confirmedCount + ' 确认</span>' +
            '<span class="wr-bb-fact-conf">' + confPct + '%</span>' +
          '</div>' +
        '</div>';
      });
      html += '</div>';
    }

    if (hypotheses.length > 0) {
      html += '<div class="wr-bb-section"><div class="wr-bb-section-label">💡 假设</div>';
      hypotheses.forEach(function(h) {
        var propM = agentMeta(h.proposed_by);
        var hConfPct = Math.round((h.confidence || 0) * 100);
        var statusCls = h.status === 'confirmed' ? 'wr-bb-hyp-confirmed' : (h.status === 'refuted' ? 'wr-bb-hyp-refuted' : 'wr-bb-hyp-proposed');
        var statusLabel = h.status === 'confirmed' ? '已确认' : (h.status === 'refuted' ? '已否决' : '待验证');
        var supCount = h.supporting_evidence ? h.supporting_evidence.length : 0;
        var conCount = h.contradicting_evidence ? h.contradicting_evidence.length : 0;
        html += '<div class="wr-bb-hypothesis liquid-glass-light">' +
          '<div class="wr-bb-hyp-head">' +
            '<span class="wr-bb-hyp-status ' + statusCls + '">' + statusLabel + '</span>' +
            '<span class="wr-bb-hyp-conf">' + hConfPct + '%</span>' +
          '</div>' +
          '<div class="wr-bb-hyp-desc">' + SC.escapeHtml(h.description) + '</div>' +
          '<div class="wr-bb-hyp-meta">' +
            '<span style="color:' + propM.color + '">' + propM.icon + ' ' + SC.escapeHtml(propM.label) + '</span>' +
            '<span class="wr-bb-hyp-evidence">✅' + supCount + ' ❌' + conCount + '</span>' +
          '</div>' +
        '</div>';
      });
      html += '</div>';
    }

    if (entries.length > 0) {
      html += '<div class="wr-bb-section"><div class="wr-bb-section-label">📝 共享观察</div>';
      entries.slice().reverse().forEach(function(e) {
        var eM = agentMeta(e.author);
        html += '<div class="wr-bb-entry">' +
          '<span class="wr-bb-entry-author" style="color:' + eM.color + '">' + eM.icon + '</span>' +
          '<span class="wr-bb-entry-value">' + SC.escapeHtml(e.value) + '</span>' +
          '<span class="wr-bb-entry-category">' + SC.escapeHtml(e.category || '') + '</span>' +
        '</div>';
      });
      html += '</div>';
    }

    return html;
  }

  function renderAssignTaskForm(session) {
    var options = '';
    (session.agents || []).forEach(function(a) {
      var m = agentMeta(a.agent_type);
      options += '<option value="' + a.agent_type + '">' + m.icon + ' ' + m.label + '</option>';
    });
    return '<div class="wr-assign-form">' +
      '<h4 class="wr-section-title">分配任务</h4>' +
      '<div class="wr-assign-row">' +
        '<select id="wr-assign-agent" class="wr-input wr-input-sm">' + options + '</select>' +
        '<input type="text" id="wr-assign-task" placeholder="输入排查任务..." class="wr-input">' +
        '<button class="wr-btn wr-btn-primary wr-btn-sm" id="wr-assign-btn">发送</button>' +
      '</div>' +
      '<div class="wr-assign-row">' +
        '<input type="text" id="wr-broadcast-msg" placeholder="广播消息给所有专家..." class="wr-input">' +
        '<button class="wr-btn wr-btn-ghost wr-btn-sm" id="wr-broadcast-btn">广播</button>' +
      '</div>' +
    '</div>';
  }

  function handleWarRoomList(data) {
    state.sessions = data.sessions || [];
    if (!state.activeSessionId) {
      renderWarRoomView();
    }
  }

  function handleWarRoomStarted(data) {
    state.sessions.unshift(data);
    state.activeSessionId = data.id;
    state.agentStatuses = {};
    state.findings = [];
    state.timeline = [];
    state.blackboard = { entries: [], hypotheses: [], sharedFacts: [] };
    state.handoffs = [];
    if (data.blackboard) {
      state.blackboard = {
        entries: data.blackboard.entries || [],
        hypotheses: data.blackboard.hypotheses || [],
        sharedFacts: data.blackboard.shared_facts || []
      };
    }
    renderWarRoomView();
  }

  function handleWarRoomStatus(data) {
    for (var i = 0; i < state.sessions.length; i++) {
      if (state.sessions[i].id === data.id) {
        state.sessions[i] = data;
        break;
      }
    }
    if (data.blackboard) {
      state.blackboard = {
        entries: data.blackboard.entries || [],
        hypotheses: data.blackboard.hypotheses || [],
        sharedFacts: data.blackboard.shared_facts || []
      };
    }
    renderWarRoomView();
  }

  function handleWarRoomStopped(data) {
    for (var i = 0; i < state.sessions.length; i++) {
      if (state.sessions[i].id === data.session_id) {
        state.sessions[i].status = 'closed';
        break;
      }
    }
    if (state.activeSessionId === data.session_id) {
      renderActiveSession(SC.$('#view-warroom'));
    }
  }

  function handleWarRoomAgentStatus(data) {
    state.agentStatuses[data.agent_type] = data.status;
    if (state.activeSessionId === data.session_id) {
      var container = SC.$('#view-warroom');
      if (container && container.classList.contains('active')) {
        renderActiveSession(container);
      }
    }
  }

  function handleWarRoomFindings(data) {
    if (state.activeSessionId === data.session_id) {
      state.findings = data.findings || [];
      var container = SC.$('#view-warroom');
      if (container && container.classList.contains('active')) {
        renderActiveSession(container);
      }
    }
  }

  function handleWarRoomTimeline(data) {
    if (state.activeSessionId === data.session_id) {
      state.timeline = data.entries || [];
    }
  }

  function handleWarRoomUpdate(data) {
    SC.wsSend('warroom_list', {});
  }

  function handleWarRoomBlackboardUpdate(data) {
    if (state.activeSessionId === data.session_id) {
      if (data.entries) {
        state.blackboard.entries = data.entries;
      }
      if (data.hypotheses) {
        state.blackboard.hypotheses = data.hypotheses;
      }
      if (data.shared_facts) {
        state.blackboard.sharedFacts = data.shared_facts;
      }
      renderActiveSession(SC.$('#view-warroom'));
    }
  }

  function handleWarRoomHandoff(data) {
    if (state.activeSessionId === data.session_id) {
      state.handoffs.push(data);
      renderActiveSession(SC.$('#view-warroom'));
    }
  }

  function handleWarRoomConfidenceChange(data) {
    if (state.activeSessionId === data.session_id) {
      var findings = state.findings.length > 0 ? state.findings : [];
      for (var i = 0; i < findings.length; i++) {
        if (findings[i].id === data.finding_id) {
          findings[i].confidence = data.new_confidence;
          break;
        }
      }
      renderActiveSession(SC.$('#view-warroom'));
    }
  }

  function handleWarRoomAutoTriggered(data) {
    if (!data) return;

    SC.toast('告警自动触发 War Room: ' + (data.title || ''), 'info');

    SC.wsSend('warroom_list', {});

    var container = SC.$('#view-warroom');
    if (container) {
      renderWarRoomView();
    }
  }

  SC.warroom = {
    render: renderWarRoomView,
    showNewForm: renderNewForm,
    handleList: handleWarRoomList,
    handleStarted: handleWarRoomStarted,
    handleStatus: handleWarRoomStatus,
    handleStopped: handleWarRoomStopped,
    handleAgentStatus: handleWarRoomAgentStatus,
    handleFindings: handleWarRoomFindings,
    handleTimeline: handleWarRoomTimeline,
    handleUpdate: handleWarRoomUpdate,
    handleBlackboardUpdate: handleWarRoomBlackboardUpdate,
    handleHandoff: handleWarRoomHandoff,
    handleConfidenceChange: handleWarRoomConfidenceChange,
    handleAutoTriggered: handleWarRoomAutoTriggered
  };
})();
