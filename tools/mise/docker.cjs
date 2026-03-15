#!/usr/bin/env node

const { spawnSync } = require('node:child_process');
const path = require('node:path');

const action = process.argv[2];
const allowed = new Set(['build', 'up', 'down', 'ps', 'logs']);

if (!allowed.has(action)) {
  console.error('Usage: node ./tools/mise/docker.cjs <build|up|down|ps|logs>');
  process.exit(1);
}

const repoRoot = path.resolve(__dirname, '..', '..');
const composeFile = path.join(repoRoot, 'infra', 'docker', 'docker-compose.yml');

function runOrExit(args) {
  const result = spawnSync('docker', args, {
    cwd: repoRoot,
    stdio: 'inherit',
    shell: false,
  });

  if (result.error) {
    console.error(`[docker] command failed: ${result.error.message}`);
    process.exit(1);
  }

  if (typeof result.status === 'number' && result.status !== 0) {
    process.exit(result.status);
  }
}

switch (action) {
  case 'build':
    runOrExit(['compose', '-f', composeFile, 'build']);
    break;
  case 'up':
    runOrExit(['compose', '-f', composeFile, 'up', '-d', '--build']);
    break;
  case 'down':
    runOrExit(['compose', '-f', composeFile, 'down']);
    break;
  case 'ps':
    runOrExit(['compose', '-f', composeFile, 'ps']);
    break;
  case 'logs':
    runOrExit(['compose', '-f', composeFile, 'logs', '--tail', '200']);
    break;
  default:
    console.error(`Unsupported action: ${action}`);
    process.exit(1);
}
