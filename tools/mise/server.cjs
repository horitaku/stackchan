#!/usr/bin/env node

const { spawnSync, spawn } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');

const action = process.argv[2];
const allowed = new Set(['run', 'kill8080', 'restart', 'restart_bg', 'healthz', 'build_ui']);

if (!allowed.has(action)) {
  console.error('Usage: node ./tools/mise/server.cjs <run|kill8080|restart|restart_bg|healthz|build_ui>');
  process.exit(1);
}

const repoRoot = path.resolve(__dirname, '..', '..');
const serverDir = path.join(repoRoot, 'server');
const webuiDir = path.join(serverDir, 'webui');

function run(cmd, args, options = {}) {
  const result = spawnSync(cmd, args, {
    stdio: options.stdio || 'pipe',
    encoding: 'utf8',
    cwd: options.cwd || process.cwd(),
    shell: false,
  });

  if (result.error) {
    throw result.error;
  }

  return result;
}

function runOrExit(cmd, args, options = {}) {
  const result = run(cmd, args, options);
  if (typeof result.status === 'number' && result.status !== 0) {
    process.exit(result.status);
  }
  return result;
}

function buildWebUI() {
  console.log('[server] Building WebUI (Svelte/Vite)');

  if (!fs.existsSync(path.join(webuiDir, 'node_modules'))) {
    runOrExit('npm', ['install'], { cwd: webuiDir, stdio: 'inherit' });
  }

  runOrExit('npm', ['run', 'build'], { cwd: webuiDir, stdio: 'inherit' });
}

function getPidsUsingPort8080() {
  if (process.platform === 'win32') {
    const result = run('netstat', ['-ano', '-p', 'tcp']);
    if (result.status !== 0) {
      return [];
    }

    const pids = new Set();
    for (const line of (result.stdout || '').split(/\r?\n/)) {
      if (!line.includes(':8080')) {
        continue;
      }

      const cols = line.trim().split(/\s+/);
      const pid = Number(cols[cols.length - 1]);
      if (Number.isInteger(pid) && pid > 0) {
        pids.add(pid);
      }
    }
    return [...pids];
  }

  const lsof = run('lsof', ['-ti', 'tcp:8080']);
  if (lsof.status === 0 && lsof.stdout) {
    return [...new Set(lsof.stdout.split(/\r?\n/).map((v) => Number(v.trim())).filter((v) => Number.isInteger(v) && v > 0))];
  }

  const fuser = run('fuser', ['-n', 'tcp', '8080']);
  if (fuser.status === 0 && fuser.stdout) {
    return [...new Set(fuser.stdout.split(/\s+/).map((v) => Number(v.trim())).filter((v) => Number.isInteger(v) && v > 0))];
  }

  return [];
}

function stopPort8080() {
  const pids = getPidsUsingPort8080();
  if (pids.length === 0) {
    console.log('[server] No process uses port 8080');
    return;
  }

  for (const pid of pids) {
    try {
      process.kill(pid, 'SIGKILL');
      console.log(`[server] Stopped PID=${pid}`);
    } catch (error) {
      console.log(`[server] Failed PID=${pid}: ${error.message}`);
    }
  }
}

async function healthz() {
  const uri = 'http://127.0.0.1:8080/healthz';
  const maxAttempts = 5;
  const delayMs = 500;

  for (let attempt = 1; attempt <= maxAttempts; attempt += 1) {
    try {
      const res = await fetch(uri, { method: 'GET' });
      const body = await res.text();

      if (!res.ok) {
        throw new Error(`HTTP ${res.status}: ${body}`);
      }

      console.log('STATUS=200');
      if (body) {
        console.log(body);
      }
      return;
    } catch (error) {
      if (attempt < maxAttempts) {
        await new Promise((resolve) => setTimeout(resolve, delayMs));
        continue;
      }

      console.error(`[server] healthz failed after ${maxAttempts} attempts: ${error.message}`);
      process.exit(1);
    }
  }
}

function runServerForeground() {
  const child = spawn('go', ['run', './cmd/stackchan-server'], {
    cwd: serverDir,
    stdio: 'inherit',
    shell: false,
  });

  child.on('exit', (code) => process.exit(code || 0));
}

function runServerBackground() {
  const logDir = path.join(repoRoot, '.logs');
  fs.mkdirSync(logDir, { recursive: true });

  const outLog = path.join(logDir, 'server.stdout.log');
  const errLog = path.join(logDir, 'server.stderr.log');

  const outFd = fs.openSync(outLog, 'a');
  const errFd = fs.openSync(errLog, 'a');

  const child = spawn('go', ['run', './cmd/stackchan-server'], {
    cwd: serverDir,
    detached: true,
    stdio: ['ignore', outFd, errFd],
    shell: false,
  });

  child.unref();

  console.log(`[server] started in background PID=${child.pid}`);
  console.log(`[server] stdout=${outLog}`);
  console.log(`[server] stderr=${errLog}`);
}

async function main() {
  switch (action) {
    case 'healthz':
      await healthz();
      return;
    case 'kill8080':
      stopPort8080();
      return;
    case 'build_ui':
      buildWebUI();
      return;
    case 'run':
      buildWebUI();
      runServerForeground();
      return;
    case 'restart':
      stopPort8080();
      buildWebUI();
      runServerForeground();
      return;
    case 'restart_bg':
      stopPort8080();
      buildWebUI();
      runServerBackground();
      return;
    default:
      console.error(`Unsupported action: ${action}`);
      process.exit(1);
  }
}

main();
