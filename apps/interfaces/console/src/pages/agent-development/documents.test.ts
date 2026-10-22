import { agentDemos } from './data';
import {
  defaultDocument,
  documentHref,
  documents,
  documentURL,
} from './documents';

describe('学习文档链接', () => {
  it('使用构建时的读取规则加载默认文档和所有章节', () => {
    for (const path of [
      defaultDocument,
      ...agentDemos.map((demo) => demo.document),
    ]) {
      expect(documents[path]).toMatch(/^#/);
    }
  });

  it('打开同目录章节，并保留锚点', () => {
    expect(documentHref('learning/README.md', '03-agent.md#工具结果')).toBe(
      `${documentURL('learning/03-agent.md')}#${encodeURIComponent('工具结果')}`,
    );
  });

  it('从分阶段清单返回上层学习路线', () => {
    expect(documentHref('learning/levels/beginner.md', '../README.md')).toBe(
      documentURL('learning/README.md'),
    );
  });

  it('可以继续阅读引用的开发规范', () => {
    expect(documentHref('learning/README.md', '../development/README.md')).toBe(
      documentURL('development/README.md'),
    );
  });

  it('外部官方文档保持外部链接', () => {
    const href =
      'https://github.com/deepseek-ai/deepseek-harness/blob/master/README.md';
    expect(documentHref('learning/deepseek-examples.md', href)).toBe(href);
  });

  it('当前文档中的锚点保留当前文档', () => {
    expect(documentHref('learning/README.md', '#学习方法')).toBe(
      `${documentURL('learning/README.md')}#${encodeURIComponent('学习方法')}`,
    );
  });

  it.each([
    'missing.md',
    '../../AGENTS.md',
    'javascript:alert(1)',
    'toString',
  ])('不把未收录文件或可执行地址变成文档链接：%s', (href) => {
    expect(documentHref('learning/README.md', href)).toBeUndefined();
  });
});
