// 管理本机前端开发服务；运行配置保留在 console，复用 npm start。
import { execFile, spawn } from 'node:child_process';
import { once } from 'node:events';
import { access, mkdir, open, readFile, rm, writeFile } from 'node:fs/promises';
import { createServer } from 'node:net';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { promisify } from 'node:util';

const execute = promisify(execFile);
const scriptPath = fileURLToPath(import.meta.url);
const consoleDir = path.resolve(path.dirname(scriptPath), '..');
const runtimeDir = path.resolve(consoleDir, '../../../.runtime/console');
const pidFile = path.join(runtimeDir, 'server.pid');
const logFile = path.join(runtimeDir, 'server.log');
const pageURL = 'http://127.0.0.1:8000/welcome';
const healthURL = 'http://127.0.0.1:8000/api/health';
const startupTimeout = 120_000;
const shutdownTimeout = 20_000;
const pollInterval = 300;
const delay = (milliseconds) =>
  new Promise((resolve) => setTimeout(resolve, milliseconds));

async function managedPID() {
  try {
    const text = (await readFile(pidFile, 'utf8')).trim();
    if (!/^[1-9]\d*$/.test(text) || Number(text) <= 1) {
      return null;
    }
    const pid = Number(text);
    process.kill(pid, 0);
    const { stdout } = await execute('ps', ['-p', text, '-o', 'command=']);
    // 核对绝对脚本路径，避免 PID 复用时终止其他项目的 Node 进程。
    if (!stdout.trim().endsWith(` ${scriptPath} run`)) {
      return null;
    }
    return pid;
  } catch (error) {
    if (
      error.code === 'ENOENT' ||
      error.code === 'ESRCH' ||
      typeof error.code === 'number'
    ) {
      return null;
    }
    throw error;
  }
}

async function check() {
  if (Number(process.versions.node.split('.')[0]) < 20) {
    throw new Error('前端需要 Node.js 20 或以上版本。');
  }
  await execute('npm', ['--version']);
  try {
    await access(path.join(consoleDir, 'node_modules/.bin/max'));
  } catch {
    throw new Error(`前端依赖未安装，请先在 ${consoleDir} 执行 npm ci。`);
  }
  if (await managedPID()) {
    return;
  }
  // 禁止开发服务器悄悄改用其他端口，也不终止非脚本管理的进程。
  const listener = createServer();
  try {
    listener.listen(8000, '127.0.0.1');
    await once(listener, 'listening');
  } catch {
    throw new Error(
      '前端端口 8000 已占用；若是手动启动的 npm start，请先在原终端按 Ctrl+C。',
    );
  } finally {
    if (listener.listening) {
      await new Promise((resolve, reject) => {
        listener.close((error) => (error ? reject(error) : resolve()));
      });
    }
  }
}

async function stop() {
  const pid = await managedPID();
  if (!pid) {
    await rm(pidFile, { force: true });
    console.log('没有运行中的脚本管理前端。');
    return;
  }
  console.log(`正在停止前端服务（PID：${pid}）……`);
  // 启动器是独立进程组的组长，统一通知 npm、Umi 和构建器退出。
  try {
    process.kill(-pid, 'SIGTERM');
  } catch (error) {
    if (error.code !== 'ESRCH') {
      throw error;
    }
  }
  const deadline = Date.now() + shutdownTimeout;
  while (await managedPID()) {
    if (Date.now() >= deadline) {
      throw new Error(`前端退出等待超时，未强制终止；请检查 ${logFile}。`);
    }
    await delay(pollInterval);
  }
  await rm(pidFile, { force: true });
  console.log('前端服务已停止。');
}

async function ready() {
  try {
    const page = await fetch(pageURL, { signal: AbortSignal.timeout(1500) });
    const html = await page.text();
    if (!page.ok || !html.includes('<html')) {
      return false;
    }
    const health = await fetch(healthURL, {
      signal: AbortSignal.timeout(1500),
    });
    const response = await health.json();
    return (
      health.ok && response.code === 'ok' && response.data?.status === 'ok'
    );
  } catch {
    return false;
  }
}

async function start() {
  const log = await open(logFile, 'w', 0o600);
  let child;
  try {
    child = spawn(process.execPath, [scriptPath, 'run'], {
      cwd: consoleDir,
      detached: true,
      stdio: ['ignore', log.fd, log.fd],
    });
    await once(child, 'spawn');
    await writeFile(pidFile, `${child.pid}\n`, { mode: 0o600 });
    child.unref();
  } finally {
    await log.close();
  }
  await waitReady(child.pid);
}

async function waitReady(pid) {
  console.log('正在等待前端页面与后台代理就绪……');
  const deadline = Date.now() + startupTimeout;
  while (Date.now() < deadline) {
    if (!(await managedPID())) {
      await rm(pidFile, { force: true });
      throw new Error(`前端启动失败，请检查 ${logFile}。`);
    }
    if (await ready()) {
      console.log(`前端服务已就绪（PID：${pid}）。`);
      console.log(`管理台：${pageURL}`);
      console.log(`前端日志：${logFile}`);
      return;
    }
    await delay(pollInterval);
  }
  throw new Error(
    `前端页面或后台代理在 120 秒内未就绪，进程仍由 PID 文件跟踪；请检查 ${logFile}。`,
  );
}

function run() {
  // npm start 中的 HOST、PORT、UMI_ENV 沿用 package.json，不在根目录复制。
  const child = spawn('npm', ['start'], {
    cwd: consoleDir,
    env: { ...process.env, BROWSER: 'none' },
    stdio: 'inherit',
  });
  for (const signal of ['SIGINT', 'SIGTERM']) {
    process.on(signal, () => child.kill(signal));
  }
  child.on('error', () => {
    console.error('无法启动 npm，请确认 Node.js 和 npm 已安装。');
    process.exit(1);
  });
  child.on('exit', (code, signal) => process.exit(code ?? (signal ? 1 : 0)));
}

async function main() {
  const action = process.argv[2];
  if (
    process.argv.length !== 3 ||
    !['run', 'check', 'start', 'restart', 'stop', 'status'].includes(action)
  ) {
    throw new Error(
      '请通过项目根目录的 ./scripts/server start|restart|stop|status 管理服务。',
    );
  }
  if (action === 'run') {
    run();
    return;
  }
  if (action === 'check') {
    await check();
    return;
  }
  await mkdir(runtimeDir, { recursive: true, mode: 0o700 });
  const lockDir = path.join(runtimeDir, 'lock');
  try {
    await mkdir(lockDir);
  } catch (error) {
    if (error.code === 'EEXIST') {
      throw new Error(
        `另一个前端管理操作正在运行；若已异常中断，请确认后删除 ${lockDir}。`,
      );
    }
    throw error;
  }
  try {
    if (action === 'status') {
      const pid = await managedPID();
      console.log(
        pid
          ? `前端服务正在运行（PID：${pid}）。\n管理台：${pageURL}\n前端日志：${logFile}`
          : '没有运行中的脚本管理前端。',
      );
      process.exitCode = pid ? 0 : 1;
    } else if (action === 'stop') {
      await stop();
    } else {
      await check();
      const pid = await managedPID();
      if (action === 'start' && pid) {
        console.log(`前端已运行（PID：${pid}），保持运行。`);
        await waitReady(pid);
        return;
      }
      if (action === 'restart') {
        await stop();
      }
      await check();
      await start();
    }
  } finally {
    await rm(lockDir, { recursive: true });
  }
}

main().catch((error) => {
  console.error(error.message);
  process.exitCode = 1;
});
