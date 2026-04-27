import { test as base, expect } from '@playwright/test';
import { ChildProcess, spawn } from 'child_process';
import { resolve } from 'path';

interface SmartClawFixture {
  serverProcess: ChildProcess | null;
}

const test = base.extend<SmartClawFixture>({
  serverProcess: async ({}, use) => {
    let proc: ChildProcess | null = null;
    try {
      const binaryPath = resolve(__dirname, '../smartclaw');
      proc = spawn(binaryPath, ['web', '--port', '8080'], {
        stdio: 'pipe',
        env: { ...process.env },
      });

      // Wait for server to be ready
      await new Promise<void>((resolve, reject) => {
        const timeout = setTimeout(() => reject(new Error('Server startup timeout')), 10000);
        proc!.stdout?.on('data', (data: Buffer) => {
          if (data.toString().includes('8080')) {
            clearTimeout(timeout);
            resolve();
          }
        });
        proc!.stderr?.on('data', (data: Buffer) => {
          if (data.toString().includes('8080')) {
            clearTimeout(timeout);
            resolve();
          }
        });
      });

      await use(proc);
    } finally {
      if (proc) {
        proc.kill('SIGTERM');
      }
    }
  },
});

export { test, expect };
