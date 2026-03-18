#!/usr/bin/env node

const { spawnSync, spawn } = require('node:child_process');
const path = require('node:path');

const action = process.argv[2];
const allowed = new Set(['build', 'port', 'upload', 'monitor', 'upmon', 'clean', 'compiledb']);

if (!allowed.has(action)) {
  console.error('Usage: node ./tools/mise/fw.cjs <build|port|upload|monitor|upmon|clean|compiledb>');
  process.exit(1);
}

const repoRoot = path.resolve(__dirname, '..', '..');
const firmwareDir = path.join(repoRoot, 'firmware');

function isWindows() {
  return process.platform === 'win32';
}

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

function platformioArgs(subArgs) {
  if (isWindows()) {
    return { cmd: 'py', args: ['-m', 'platformio', ...subArgs] };
  }

  const pio = run('pio', ['--version']);
  if (pio.status === 0) {
    return { cmd: 'pio', args: subArgs };
  }

  return { cmd: 'python3', args: ['-m', 'platformio', ...subArgs] };
}

function runPlatformIO(subArgs, options = {}) {
  const { cmd, args } = platformioArgs(subArgs);
  const result = run(cmd, args, {
    stdio: options.stdio || 'inherit',
    cwd: options.cwd || firmwareDir,
  });
  if (typeof result.status === 'number' && result.status !== 0) {
    process.exit(result.status);
  }
  return result;
}

function parseDevices(raw) {
  const lines = raw.split(/\r?\n/);
  const devices = [];
  let current = null;

  const devicePattern = isWindows() ? /^COM\d+$/i : /^\/dev\/.+/;

  for (const line of lines) {
    const trimmed = line.trim();
    if (!trimmed) {
      continue;
    }

    if (devicePattern.test(trimmed)) {
      if (current) {
        devices.push(current);
      }
      current = { port: trimmed, info: '' };
      continue;
    }

    if (current) {
      current.info += `${trimmed}\n`;
    }
  }

  if (current) {
    devices.push(current);
  }

  return devices;
}

function getPortOverride(actionName) {
  const actionKey = `FW_${actionName.toUpperCase()}_PORT`;
  return process.env[actionKey] || process.env.FW_PORT || '';
}

function scoreDevice(device) {
  const port = device.port || '';
  const info = device.info || '';

  // Highest confidence: explicit ESP32-S3 USB VID:PID used by CoreS3 in USB CDC mode.
  if (/303A:1001/i.test(info)) {
    return 100;
  }

  // Common external USB serial adapters.
  if (/10C4:EA60|1A86:7523|0403:6001/i.test(info)) {
    return 90;
  }

  // Likely external serial devices.
  if (/^\/dev\/(ttyACM|ttyUSB)/.test(port) || /^COM\d+$/i.test(port)) {
    return 70;
  }

  // Likely internal UART on Linux SBC; do not prefer by default.
  if (/^\/dev\/(ttyAMA|ttyS)/.test(port)) {
    return -10;
  }

  // Unknown but present.
  return 10;
}

function getFirmwarePort() {
  const { cmd, args } = platformioArgs(['device', 'list']);
  const result = run(cmd, args, { stdio: 'pipe', cwd: firmwareDir });
  if (result.status !== 0) {
    console.error(result.stderr || result.stdout || 'platformio device list failed');
    process.exit(result.status || 1);
  }

  const devices = parseDevices(result.stdout || '');
  if (devices.length === 0) {
    throw new Error('No serial port found');
  }

  const ranked = devices
    .map((device) => ({ ...device, score: scoreDevice(device) }))
    .sort((a, b) => b.score - a.score);

  const best = ranked[0];
  if (!best || best.score < 0) {
    const ports = devices.map((d) => d.port).join(', ');
    throw new Error(
      `No reliable USB serial port found. Detected: ${ports}. ` +
        'Specify port with FW_PORT (or FW_UPLOAD_PORT/FW_MONITOR_PORT).'
    );
  }

  return best.port;
}

if (action === 'port') {
  try {
    console.log(getPortOverride('port') || getFirmwarePort());
  } catch (error) {
    console.error(error.message);
    process.exit(1);
  }
  process.exit(0);
}

if (action === 'build') {
  runPlatformIO(['run', '-e', 'stackchan_cores3']);
  process.exit(0);
}

if (action === 'clean') {
  runPlatformIO(['run', '-e', 'stackchan_cores3', '-t', 'clean']);
  process.exit(0);
}

if (action === 'compiledb') {
  // IntelliSense 用の compile_commands.json を firmware/ に生成します。
  // 生成後は VS Code で「C/C++: Reset IntelliSense Database」を実行してください。
  runPlatformIO(['run', '-e', 'stackchan_cores3', '--target', 'compiledb']);
  process.exit(0);
}

if (action === 'upload') {
  const port = getPortOverride('upload') || getFirmwarePort();
  console.log(`Uploading to ${port}`);
  runPlatformIO(['run', '-e', 'stackchan_cores3', '-t', 'upload', '--upload-port', port]);
  process.exit(0);
}

if (action === 'monitor') {
  const port = getPortOverride('monitor') || getFirmwarePort();
  console.log(`Monitoring ${port}`);
  runPlatformIO(['device', 'monitor', '--baud', '115200', '--port', port, '--filter', 'direct']);
  process.exit(0);
}

if (action === 'upmon') {
  const port = getPortOverride('upload') || getPortOverride('monitor') || getPortOverride('upmon') || getFirmwarePort();
  console.log(`Upload+Monitor on ${port}`);
  const { cmd, args } = platformioArgs([
    'run',
    '-e',
    'stackchan_cores3',
    '-t',
    'upload',
    '-t',
    'monitor',
    '--upload-port',
    port,
    '--monitor-port',
    port,
  ]);

  const child = spawn(cmd, args, {
    cwd: firmwareDir,
    stdio: 'inherit',
    shell: false,
  });

  child.on('exit', (code) => process.exit(code || 0));
}
