import * as esbuild from 'esbuild';
import { readFileSync, writeFileSync, mkdirSync, statSync } from 'fs';
import { dirname, join } from 'path';
import { fileURLToPath } from 'url';

const __dirname = dirname(fileURLToPath(import.meta.url));

const appFiles = [
  'common.js', 'state.js', 'virtual-list.js', 'ui.js', 'ws.js',
  'tabs.js', 'chat.js', 'sessions.js', 'files.js', 'skills.js',
  'memory.js', 'wiki.js', 'agents.js', 'notifications.js', 'tools.js',
  'upload.js', 'share.js', 'dag.js', 'voice.js', 'editor.js',
  'templates.js', 'search.js', 'app.js'
];

console.log('Concatenating', appFiles.length, 'files...');
let bundle = `/* SmartClaw Web UI Bundle - Built ${new Date().toISOString()} */\n`;
for (const f of appFiles) {
  try {
    bundle += readFileSync(join(__dirname, f), 'utf8') + '\n';
  } catch (e) {
    console.warn(`Warning: Could not read ${f}: ${e.message}`);
  }
}

const distDir = join(__dirname, 'dist');
mkdirSync(distDir, { recursive: true });
writeFileSync(join(distDir, 'bundle.js'), bundle);

console.log('Minifying with esbuild...');
await esbuild.build({
  entryPoints: [join(distDir, 'bundle.js')],
  outfile: join(distDir, 'bundle.min.js'),
  minify: true,
  sourcemap: true,
  target: ['es2020'],
  logLevel: 'info',
});

const size = statSync(join(distDir, 'bundle.min.js')).size;
console.log(`\u2713 Bundle: ${(size / 1024).toFixed(1)}KB raw, gzipped ~${(size / 1024 / 3).toFixed(1)}KB estimated`);
