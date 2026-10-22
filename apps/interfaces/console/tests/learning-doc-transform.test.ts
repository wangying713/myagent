import {
  copyFileSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

interface DocumentTransformer {
  getCacheKey(
    source: string,
    filename: string,
    options: { configString: string; instrument?: boolean },
  ): string;
  process(): { code: string };
}

describe('文档转换缓存', () => {
  it('源文档、读取器和插桩设置变化时使缓存失效', () => {
    const root = mkdtempSync(join(tmpdir(), 'saklay-doc-transform-'));
    const consoleDir = join(root, 'apps/interfaces/console');
    try {
      for (const directory of ['config', 'tests']) {
        mkdirSync(join(consoleDir, directory), { recursive: true });
      }
      mkdirSync(join(root, 'docs'));
      for (const file of [
        'config/learning-doc-loader.cjs',
        'tests/learning-doc-transform.cjs',
      ]) {
        copyFileSync(join(__dirname, '..', file), join(consoleDir, file));
      }
      const documentPath = join(root, 'docs/example.md');
      writeFileSync(documentPath, '# 第一版');
      const transformer = jest.requireActual<DocumentTransformer>(
        join(consoleDir, 'tests/learning-doc-transform.cjs'),
      );
      const options = { configString: 'test' };
      const filename = join(consoleDir, 'src/content.docs');
      const key = transformer.getCacheKey('占位文件', filename, options);
      expect(transformer.process().code).toContain('# 第一版');
      expect(transformer.getCacheKey('占位文件', filename, options)).toBe(key);
      writeFileSync(documentPath, '# 第二版');
      const changed = transformer.getCacheKey('占位文件', filename, options);
      expect(changed).not.toBe(key);
      expect(transformer.process().code).toContain('# 第二版');
      const loader = join(consoleDir, 'config/learning-doc-loader.cjs');
      writeFileSync(
        loader,
        `${readFileSync(loader, 'utf8')}\n// 读取器版本变化\n`,
      );
      expect(transformer.getCacheKey('占位文件', filename, options)).not.toBe(
        changed,
      );
      expect(
        transformer.getCacheKey('占位文件', filename, {
          ...options,
          instrument: true,
        }),
      ).not.toBe(transformer.getCacheKey('占位文件', filename, options));
    } finally {
      rmSync(root, { recursive: true, force: true });
    }
  });
});
