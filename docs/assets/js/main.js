/* ============================================
   recursive-llm-ts Documentation Site - JS
   ============================================ */

document.addEventListener('DOMContentLoaded', () => {
  initMobileMenu();
  initTabs();
  initCopyButtons();
  initSyntaxHighlight();
  initActiveNav();
  initDemoRunner();
});

/* ---------- Mobile Menu ---------- */
function initMobileMenu() {
  const btn = document.querySelector('.mobile-menu-btn');
  const nav = document.querySelector('.site-nav');
  if (!btn || !nav) return;

  btn.addEventListener('click', () => {
    nav.classList.toggle('open');
    btn.setAttribute('aria-expanded', nav.classList.contains('open'));
  });

  // Close menu when clicking a nav link
  nav.querySelectorAll('a').forEach(link => {
    link.addEventListener('click', () => nav.classList.remove('open'));
  });
}

/* ---------- Tabs ---------- */
function initTabs() {
  document.querySelectorAll('.tabs').forEach(tabGroup => {
    const buttons = tabGroup.querySelectorAll('.tab-btn');
    const container = tabGroup.parentElement;
    const panels = container.querySelectorAll('.tab-panel');

    buttons.forEach(btn => {
      btn.addEventListener('click', () => {
        const target = btn.dataset.tab;

        buttons.forEach(b => b.classList.remove('active'));
        panels.forEach(p => p.classList.remove('active'));

        btn.classList.add('active');
        const panel = container.querySelector(`[data-panel="${target}"]`);
        if (panel) panel.classList.add('active');
      });
    });
  });
}

/* ---------- Copy to Clipboard ---------- */
function initCopyButtons() {
  document.querySelectorAll('.code-copy').forEach(btn => {
    btn.addEventListener('click', () => {
      const block = btn.closest('.code-block');
      const code = block.querySelector('code');
      if (!code) return;

      const text = code.textContent;
      navigator.clipboard.writeText(text).then(() => {
        const orig = btn.textContent;
        btn.textContent = 'Copied!';
        setTimeout(() => { btn.textContent = orig; }, 2000);
      });
    });
  });

  // Install command copy
  document.querySelectorAll('.copy-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      const cmd = btn.closest('.install-cmd');
      const text = cmd.querySelector('.cmd-text')?.textContent || 'npm install recursive-llm-ts';
      navigator.clipboard.writeText(text).then(() => {
        btn.innerHTML = '&#10003;';
        setTimeout(() => { btn.innerHTML = '&#128203;'; }, 2000);
      });
    });
  });
}

/* ---------- Basic Syntax Highlighting ---------- */
function initSyntaxHighlight() {
  document.querySelectorAll('code[data-lang]').forEach(block => {
    const lang = block.dataset.lang;
    let html = escapeHtml(block.textContent);

    if (lang === 'typescript' || lang === 'ts' || lang === 'javascript' || lang === 'js') {
      html = highlightTS(html);
    } else if (lang === 'bash' || lang === 'sh') {
      html = highlightBash(html);
    } else if (lang === 'go') {
      html = highlightGo(html);
    }

    block.innerHTML = html;
  });
}

function escapeHtml(text) {
  return text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
}

function highlightTS(html) {
  // Use placeholder-based approach to prevent regexes from matching inside already-highlighted spans
  var placeholders = [];
  function placeholder(text, cls) {
    var id = '\x00PH' + placeholders.length + '\x00';
    placeholders.push('<span class="token-' + cls + '">' + text + '</span>');
    return id;
  }
  // Comments
  html = html.replace(/(\/\/.*)/g, function(m) { return placeholder(m, 'comment'); });
  // Strings (double and single quotes, template literals)
  html = html.replace(/('(?:[^'\\]|\\.)*')/g, function(m) { return placeholder(m, 'string'); });
  html = html.replace(/("(?:[^"\\]|\\.)*")/g, function(m) { return placeholder(m, 'string'); });
  html = html.replace(/(`(?:[^`\\]|\\.)*`)/g, function(m) { return placeholder(m, 'string'); });
  // Keywords
  html = html.replace(/\b(import|from|export|const|let|var|function|return|async|await|new|class|interface|type|extends|implements|if|else|for|while|switch|case|break|continue|try|catch|throw|typeof|instanceof|of|in)\b/g,
    function(m) { return placeholder(m, 'keyword'); });
  // Types
  html = html.replace(/\b(string|number|boolean|void|null|undefined|any|never|Promise|Array|Record|Partial)\b/g,
    function(m) { return placeholder(m, 'type'); });
  // Numbers
  html = html.replace(/\b(\d+(?:\.\d+)?)\b/g, function(m) { return placeholder(m, 'number'); });
  // Special values
  html = html.replace(/\b(true|false|null|undefined)\b/g, function(m) { return placeholder(m, 'const'); });
  // Restore placeholders
  for (var i = 0; i < placeholders.length; i++) {
    html = html.replace('\x00PH' + i + '\x00', placeholders[i]);
  }
  return html;
}

function highlightBash(html) {
  var placeholders = [];
  function placeholder(text, cls) {
    var id = '\x00PH' + placeholders.length + '\x00';
    placeholders.push('<span class="token-' + cls + '">' + text + '</span>');
    return id;
  }
  // Comments
  html = html.replace(/(#.*)/g, function(m) { return placeholder(m, 'comment'); });
  // Strings
  html = html.replace(/('(?:[^'\\]|\\.)*')/g, function(m) { return placeholder(m, 'string'); });
  html = html.replace(/("(?:[^"\\]|\\.)*")/g, function(m) { return placeholder(m, 'string'); });
  // Keywords
  html = html.replace(/\b(export|npm|cd|go|node|ts-node|mkdir|cp|mv|rm|git|docker)\b/g,
    function(m) { return placeholder(m, 'keyword'); });
  // Restore placeholders
  for (var i = 0; i < placeholders.length; i++) {
    html = html.replace('\x00PH' + i + '\x00', placeholders[i]);
  }
  return html;
}

function highlightGo(html) {
  var placeholders = [];
  function placeholder(text, cls) {
    var id = '\x00PH' + placeholders.length + '\x00';
    placeholders.push('<span class="token-' + cls + '">' + text + '</span>');
    return id;
  }
  // Comments
  html = html.replace(/(\/\/.*)/g, function(m) { return placeholder(m, 'comment'); });
  // Strings
  html = html.replace(/("(?:[^"\\]|\\.)*")/g, function(m) { return placeholder(m, 'string'); });
  html = html.replace(/(`(?:[^`]*)`)/g, function(m) { return placeholder(m, 'string'); });
  // Keywords
  html = html.replace(/\b(package|import|func|return|var|const|type|struct|interface|if|else|for|range|switch|case|break|go|defer|chan|select|map|make|new|nil|err)\b/g,
    function(m) { return placeholder(m, 'keyword'); });
  // Types
  html = html.replace(/\b(string|int|int64|float64|bool|error|byte)\b/g,
    function(m) { return placeholder(m, 'type'); });
  // Restore placeholders
  for (var i = 0; i < placeholders.length; i++) {
    html = html.replace('\x00PH' + i + '\x00', placeholders[i]);
  }
  return html;
}

/* ---------- Active Navigation ---------- */
function initActiveNav() {
  const path = window.location.pathname;
  document.querySelectorAll('.site-nav a').forEach(link => {
    const href = link.getAttribute('href');
    if (href && path.endsWith(href.replace('./', ''))) {
      link.classList.add('active');
    }
  });

  // Sidebar scroll spy
  const sidebarLinks = document.querySelectorAll('.sidebar-link');
  if (sidebarLinks.length === 0) return;

  const observer = new IntersectionObserver((entries) => {
    entries.forEach(entry => {
      if (entry.isIntersecting) {
        const id = entry.target.getAttribute('id');
        sidebarLinks.forEach(link => {
          link.classList.toggle('active', link.getAttribute('href') === `#${id}`);
        });
      }
    });
  }, { rootMargin: '-20% 0px -70% 0px' });

  document.querySelectorAll('.docs-content h2[id], .docs-content h3[id]').forEach(heading => {
    observer.observe(heading);
  });
}

/* ---------- Interactive Demo Runner ---------- */
function initDemoRunner() {
  const demoSelect = document.getElementById('demo-example-select');
  const demoCode = document.getElementById('demo-code');
  const demoOutput = document.getElementById('demo-output');
  const demoRunBtn = document.getElementById('demo-run-btn');

  if (!demoSelect || !demoCode) return;

  const examples = {
    completion: {
      code: `import { RLM } from 'recursive-llm-ts';

const rlm = new RLM('gpt-4o-mini', {
  api_key: process.env.OPENAI_API_KEY,
  max_iterations: 15,
});

const result = await rlm.completion(
  'What are the key findings in this research?',
  longResearchPaper   // 100K+ tokens
);

console.log(result.result);
console.log('LLM calls:', result.stats.llm_calls);
console.log('Iterations:', result.stats.iterations);`,
      output: `> Processing 127,439 tokens recursively...
> Depth 1: Splitting into 4 chunks (31,860 tokens each)
> Depth 2: Processing chunk 1/4...
> Depth 2: Processing chunk 2/4...
> Depth 2: Processing chunk 3/4...
> Depth 2: Processing chunk 4/4...
> Merging results from 4 chunks...

Result: "The key findings include: (1) Recursive decomposition
enables processing of arbitrarily large contexts without
degradation, (2) The approach maintains 94.7% accuracy compared
to full-context baselines while processing 10x more tokens..."

LLM calls: 8
Iterations: 12`
    },
    structured: {
      code: `import { RLM } from 'recursive-llm-ts';
import { z } from 'zod';

const rlm = new RLM('gpt-4o-mini', {
  api_key: process.env.OPENAI_API_KEY,
});

const schema = z.object({
  sentiment: z.number().min(1).max(5),
  summary: z.string(),
  topics: z.array(z.enum([
    'pricing', 'features', 'support'
  ])),
  keyPhrases: z.array(z.object({
    phrase: z.string(),
    weight: z.number(),
  })),
});

const result = await rlm.structuredCompletion(
  'Analyze this call transcript',
  transcript,
  schema,
  { parallelExecution: true }
);

console.log(JSON.stringify(result.result, null, 2));`,
      output: `{
  "sentiment": 4,
  "summary": "Customer expressed satisfaction with the product
    features but requested improvements to the pricing model
    for enterprise tiers.",
  "topics": ["pricing", "features"],
  "keyPhrases": [
    { "phrase": "love the dashboard", "weight": 0.92 },
    { "phrase": "enterprise pricing", "weight": 0.87 },
    { "phrase": "team collaboration", "weight": 0.78 }
  ]
}`
    },
    files: {
      code: `import { RLM } from 'recursive-llm-ts';

const rlm = new RLM('gpt-4o-mini', {
  api_key: process.env.OPENAI_API_KEY,
});

// Process an entire codebase as context
const result = await rlm.completionFromFiles(
  'Describe the architecture and key patterns',
  {
    type: 'local',
    path: '/path/to/project/src',
    extensions: ['.ts', '.tsx'],
    excludePatterns: ['*.test.ts', '*.spec.ts'],
    maxFileSize: 100_000,
  }
);

console.log(result.result);
console.log('Files:', result.fileStorage?.files.length);
console.log('Skipped:', result.fileStorage?.skipped.length);`,
      output: `> Scanning /path/to/project/src...
> Found 47 files matching filters
> Total context: 892KB (47 files)
> Skipped: 3 files (exceeded maxFileSize)

Result: "The codebase follows a layered architecture pattern:

1. API Layer (src/routes/): Express routes with middleware
2. Service Layer (src/services/): Business logic and orchestration
3. Data Layer (src/models/): Prisma ORM models
4. Shared (src/utils/): Validation, logging, error handling

Key patterns: Repository pattern for data access,
middleware chain for auth/validation, event-driven
notifications via Redis pub/sub."

Files: 47
Skipped: 3`
    },
    s3: {
      code: `import { RLM } from 'recursive-llm-ts';
import { z } from 'zod';

const rlm = new RLM('gpt-4o-mini', {
  api_key: process.env.OPENAI_API_KEY,
});

const schema = z.object({
  summary: z.string(),
  mainTopics: z.array(z.string()),
  sentiment: z.enum([
    'positive', 'negative', 'neutral'
  ]),
});

// Process S3 bucket files
const result = await rlm.structuredCompletionFromFiles(
  'Extract summary, topics, and sentiment',
  {
    type: 's3',
    path: 'my-reports-bucket',
    prefix: 'reports/2024/',
    extensions: ['.md', '.txt'],
    region: 'us-west-2',
    // Uses AWS_ACCESS_KEY_ID from env
  },
  schema
);

console.log(result.result);`,
      output: `> Connecting to S3: my-reports-bucket (us-west-2)
> Listing objects with prefix: reports/2024/
> Found 23 files matching filters
> Total context: 1.2MB

{
  "summary": "Q4 2024 reports show consistent revenue growth
    across all product lines, with SaaS subscriptions up 34%
    YoY and enterprise deals closing 2 weeks faster.",
  "mainTopics": [
    "Revenue Growth",
    "SaaS Metrics",
    "Enterprise Sales",
    "Customer Retention"
  ],
  "sentiment": "positive"
}`
    },
    observability: {
      code: `import { RLM } from 'recursive-llm-ts';

const rlm = new RLM('gpt-4o-mini', {
  api_key: process.env.OPENAI_API_KEY,
  meta_agent: { enabled: true },
  observability: {
    debug: true,
    trace_enabled: true,
    trace_endpoint: 'http://localhost:4318',
    langfuse_enabled: true,
    langfuse_public_key: process.env.LANGFUSE_PUBLIC_KEY,
    langfuse_secret_key: process.env.LANGFUSE_SECRET_KEY,
  }
});

const result = await rlm.completion(
  'what happened in the meeting?',
  meetingTranscript
);

// Access trace events
const events = rlm.getTraceEvents();
for (const e of events) {
  console.log(\`[\${e.type}] \${e.name}\`, e.attributes);
}`,
      output: `[DEBUG] Meta-agent optimizing query...
[DEBUG] Original: "what happened in the meeting?"
[DEBUG] Optimized: "Provide a detailed summary of the meeting
  including: key decisions made, action items assigned,
  participants and their contributions, and any unresolved
  issues or follow-ups needed."
[span_start] rlm.completion {trace_id: "abc123"}
[span_start] meta_agent.optimize {parent: "abc123"}
[llm_call] gpt-4o-mini {tokens: 342, duration_ms: 891}
[span_end] meta_agent.optimize {duration_ms: 912}
[span_start] rlm.recursive_loop {depth: 1}
[llm_call] gpt-4o-mini {tokens: 1247, duration_ms: 2341}
[llm_call] gpt-4o-mini {tokens: 856, duration_ms: 1672}
[span_end] rlm.recursive_loop {iterations: 4}
[span_end] rlm.completion {total_duration_ms: 5218}
[event] langfuse.export {events: 8, status: "ok"}`
    }
  };

  function updateDemo(key) {
    const example = examples[key];
    if (!example) return;
    demoCode.textContent = example.code;
    demoCode.dataset.lang = 'typescript';
    initSyntaxHighlight();
    if (demoOutput) demoOutput.textContent = '';
  }

  demoSelect.addEventListener('change', (e) => {
    updateDemo(e.target.value);
  });

  if (demoRunBtn) {
    demoRunBtn.addEventListener('click', () => {
      const key = demoSelect.value;
      const example = examples[key];
      if (!example || !demoOutput) return;

      demoOutput.textContent = '';
      demoRunBtn.disabled = true;
      demoRunBtn.textContent = 'Running...';

      // Simulate typing output
      let i = 0;
      const lines = example.output.split('\n');
      function typeLine() {
        if (i >= lines.length) {
          demoRunBtn.disabled = false;
          demoRunBtn.textContent = 'Run Example';
          return;
        }
        demoOutput.textContent += (i > 0 ? '\n' : '') + lines[i];
        demoOutput.scrollTop = demoOutput.scrollHeight;
        i++;
        setTimeout(typeLine, 80 + Math.random() * 120);
      }
      setTimeout(typeLine, 300);
    });
  }

  // Initialize first example
  updateDemo('completion');
}
