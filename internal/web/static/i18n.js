// SmartClaw - i18n Framework
(function() {
  'use strict';

  var currentLang = 'en';
  var translations = {};
  var fallbackTranslations = {};

  function t(key, params) {
    var value = (translations[key] !== undefined) ? translations[key] :
                (fallbackTranslations[key] !== undefined) ? fallbackTranslations[key] : key;
    if (params) {
      Object.keys(params).forEach(function(k) {
        value = value.replace(new RegExp('\\{\\{' + k + '\\}\\}', 'g'), params[k]);
      });
    }
    return value;
  }

  function setLanguage(lang) {
    currentLang = lang;
    document.documentElement.lang = lang;
    try { localStorage.setItem('smartclaw-lang', lang); } catch (e) {}
    loadTranslations(lang);
    applyTranslations();
  }

  function getLanguage() {
    return currentLang;
  }

  function loadTranslations(lang) {
    var data = SC.state && SC.state.i18n && SC.state.i18n[lang];
    if (data) {
      translations = data;
    } else if (lang === 'zh') {
      translations = ZH_TRANSLATIONS;
    } else {
      translations = EN_TRANSLATIONS;
    }
  }

  function applyTranslations() {
    document.querySelectorAll('[data-i18n]').forEach(function(el) {
      var key = el.getAttribute('data-i18n');
      if (key) el.textContent = t(key);
    });
    document.querySelectorAll('[data-i18n-placeholder]').forEach(function(el) {
      var key = el.getAttribute('data-i18n-placeholder');
      if (key) el.placeholder = t(key);
    });
    document.querySelectorAll('[data-i18n-title]').forEach(function(el) {
      var key = el.getAttribute('data-i18n-title');
      if (key) el.title = t(key);
    });
    document.querySelectorAll('[data-i18n-aria-label]').forEach(function(el) {
      var key = el.getAttribute('data-i18n-aria-label');
      if (key) el.setAttribute('aria-label', t(key));
    });
  }

  function init() {
    var saved;
    try { saved = localStorage.getItem('smartclaw-lang'); } catch (e) {}
    if (saved) {
      currentLang = saved;
    } else if (navigator.language) {
      var navLang = navigator.language.slice(0, 2).toLowerCase();
      if (navLang === 'zh' || navLang === 'en') {
        currentLang = navLang;
      }
    }
    fallbackTranslations = EN_TRANSLATIONS;
    loadTranslations(currentLang);
    applyTranslations();
  }

  // English translations (built-in)
  var EN_TRANSLATIONS = {
    'app.title': 'SmartClaw',
    'sidebar.files': 'Files',
    'sidebar.sessions': 'Sessions',
    'sidebar.agents': 'Agents',
    'sidebar.skills': 'Skills',
    'sidebar.memory': 'Memory',
    'sidebar.wiki': 'Wiki',
    'sidebar.tools': 'MCP Tools',
    'sidebar.settings': 'Settings',
    'chat.placeholder': 'Type a message... (Shift+Enter for new line)',
    'chat.send': 'Send',
    'chat.stop': 'Stop',
    'chat.new': 'New Chat',
    'chat.thinking': 'Thinking',
    'chat.edit': 'Edit',
    'chat.retry': 'Retry',
    'chat.copy': 'Copy',
    'chat.quote': 'Quote Reply',
    'chat.copyCode': 'Copy Code',
    'chat.copyId': 'Copy Message ID',
    'session.new': 'New Session',
    'session.search': 'Search sessions...',
    'session.delete': 'Delete',
    'session.rename': 'Rename',
    'files.search': 'Search files...',
    'files.noFiles': 'No files loaded',
    'agents.noAgents': 'No active agents',
    'skills.noSkills': 'No skills yet',
    'skills.create': 'Create Skill',
    'memory.search': 'Search memory...',
    'tools.installed': 'Installed',
    'tools.discover': 'Discover',
    'tools.search': 'Search MCP servers...',
    'templates.title': 'Prompt Templates',
    'templates.search': 'Search templates...',
    'templates.create': 'Create Template',
    'templates.insert': 'Insert',
    'search.placeholder': 'Search messages...',
    'search.messages': 'Messages',
    'search.code': 'Code',
    'search.noResults': 'No results found',
    'search.recent': 'Recent searches',
    'share.title': 'Share Conversation',
    'share.link': 'Share Link',
    'share.copyLink': 'Copy Link',
    'share.exportMd': 'Export Markdown',
    'share.exportPdf': 'Export PDF',
    'share.copied': 'Link copied!',
    'settings.theme': 'Theme',
    'settings.font': 'Font Size',
    'settings.model': 'Model',
    'settings.language': 'Language',
    'notifications.title': 'Notifications',
    'notifications.markRead': 'Mark all read',
    'notifications.clearAll': 'Clear all',
    'login.title': 'Welcome to SmartClaw',
    'login.apiKey': 'API Key',
    'login.submit': 'Login',
    'empty.files': 'No files loaded',
    'empty.files.desc': 'Open a project directory to see files',
    'empty.sessions': 'No conversations yet',
    'empty.sessions.desc': 'Start your first conversation',
    'empty.agents': 'No active agents',
    'empty.agents.desc': 'Agents will appear when processing tasks',
    'tab.new': 'New Chat',
    'tab.close': 'Close',
    'tab.closeOthers': 'Close Others',
    'tab.rename': 'Rename',
    'welcome.desc': 'Your AI-powered coding assistant. Ask me anything to get started.',
    'view.schedule': 'Schedule',
    'view.cost': 'Cost Intelligence',
    'view.context': 'Context',
    'view.workflows': 'Workflows',
    'view.onboarding': 'Getting Started',
    'nav.chat': 'Chat',
    'nav.sessions': 'Sessions',
    'nav.agents': 'Agents',
    'nav.skills': 'Skills',
    'nav.memory': 'Memory',
    'nav.settings': 'Settings',
    'nav.files': 'Files',
    'nav.wiki': 'Wiki',
    'nav.mcp': 'MCP',
    'nav.context': 'Context',
    'nav.cron': 'Schedule',
    'nav.cost': 'Cost',
    'nav.workflows': 'Workflows',
    'nav.onboarding': 'Get Started',
    'settings.provider': 'Provider Configuration',
    'settings.apiKey': 'API Key',
    'settings.baseUrl': 'Base URL',
    'settings.customModel': 'Custom Model',
    'settings.openaiCompat': 'OpenAI Compatible',
    'settings.saveApply': 'Save & Apply',
    'settings.themeEditor': 'Theme Editor',
    'settings.dashboard': 'Dashboard',
    'memory.l1': 'L1 Prompt',
    'memory.l2': 'L2 Sessions',
    'memory.l3': 'L3 Skills',
    'memory.l4': 'L4 User Model',
    'memory.observations': 'Observations',
    'skills.createBtn': '+ Create',
  };

  // Chinese translations
  var ZH_TRANSLATIONS = {
    'app.title': 'SmartClaw',
    'sidebar.files': '文件',
    'sidebar.sessions': '会话',
    'sidebar.agents': '代理',
    'sidebar.skills': '技能',
    'sidebar.memory': '记忆',
    'sidebar.wiki': '知识库',
    'sidebar.tools': 'MCP 工具',
    'sidebar.settings': '设置',
    'chat.placeholder': '输入消息... (Shift+Enter 换行)',
    'chat.send': '发送',
    'chat.stop': '停止',
    'chat.new': '新对话',
    'chat.thinking': '思考中',
    'chat.edit': '编辑',
    'chat.retry': '重试',
    'chat.copy': '复制',
    'chat.quote': '引用回复',
    'chat.copyCode': '复制代码',
    'chat.copyId': '复制消息 ID',
    'session.new': '新建会话',
    'session.search': '搜索会话...',
    'session.delete': '删除',
    'session.rename': '重命名',
    'files.search': '搜索文件...',
    'files.noFiles': '未加载文件',
    'agents.noAgents': '无活跃代理',
    'skills.noSkills': '暂无技能',
    'skills.create': '创建技能',
    'memory.search': '搜索记忆...',
    'tools.installed': '已安装',
    'tools.discover': '发现',
    'tools.search': '搜索 MCP 服务器...',
    'templates.title': '提示模板',
    'templates.search': '搜索模板...',
    'templates.create': '创建模板',
    'templates.insert': '插入',
    'search.placeholder': '搜索消息...',
    'search.messages': '消息',
    'search.code': '代码',
    'search.noResults': '未找到结果',
    'search.recent': '最近搜索',
    'share.title': '分享对话',
    'share.link': '分享链接',
    'share.copyLink': '复制链接',
    'share.exportMd': '导出 Markdown',
    'share.exportPdf': '导出 PDF',
    'share.copied': '链接已复制！',
    'settings.theme': '主题',
    'settings.font': '字号',
    'settings.model': '模型',
    'settings.language': '语言',
    'notifications.title': '通知',
    'notifications.markRead': '全部已读',
    'notifications.clearAll': '清除全部',
    'login.title': '欢迎使用 SmartClaw',
    'login.apiKey': 'API 密钥',
    'login.submit': '登录',
    'empty.files': '未加载文件',
    'empty.files.desc': '打开项目目录查看文件',
    'empty.sessions': '暂无会话',
    'empty.sessions.desc': '开始你的第一次对话',
    'empty.agents': '无活跃代理',
    'empty.agents.desc': '处理任务时代理会自动出现',
    'tab.new': '新对话',
    'tab.close': '关闭',
    'tab.closeOthers': '关闭其他',
    'tab.rename': '重命名',
    'welcome.desc': '你的AI编程助手，随时提问开始使用。',
    'view.schedule': '定时任务',
    'view.cost': '成本智能',
    'view.context': '上下文',
    'view.workflows': '工作流',
    'view.onboarding': '开始使用',
    'nav.chat': '对话',
    'nav.sessions': '会话',
    'nav.agents': '代理',
    'nav.skills': '技能',
    'nav.memory': '记忆',
    'nav.settings': '设置',
    'nav.files': '文件',
    'nav.wiki': '知识库',
    'nav.mcp': 'MCP',
    'nav.context': '上下文',
    'nav.cron': '定时',
    'nav.cost': '成本',
    'nav.workflows': '工作流',
    'nav.onboarding': '开始',
    'settings.provider': '服务商配置',
    'settings.apiKey': 'API 密钥',
    'settings.baseUrl': '接口地址',
    'settings.customModel': '自定义模型',
    'settings.openaiCompat': 'OpenAI 兼容',
    'settings.saveApply': '保存并应用',
    'settings.themeEditor': '主题编辑器',
    'settings.dashboard': '仪表盘',
    'memory.l1': 'L1 提示',
    'memory.l2': 'L2 会话',
    'memory.l3': 'L3 技能',
    'memory.l4': 'L4 用户模型',
    'memory.observations': '观察记录',
    'skills.createBtn': '+ 创建',
  };

  SC.i18n = {
    t: t,
    setLanguage: setLanguage,
    getLanguage: getLanguage,
    init: init,
    applyTranslations: applyTranslations,
    translations: translations
  };

  // Shorthand
  SC.t = t;
})();
