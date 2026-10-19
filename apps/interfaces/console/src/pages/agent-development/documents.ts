import content from './content.docs';
import { agentDemos } from './data';

export const documents = content;

export const defaultDocument = 'learning/deepseek-examples.md';

export function documentURL(documentPath: string): string {
  const demo = agentDemos.find((item) => item.document === documentPath);
  return demo
    ? `/agent-demos/${demo.id}`
    : `/agent-demos/overview?doc=${encodeURIComponent(documentPath)}`;
}

// 文档内的相对链接继续在阅读页打开，外部资料保留原地址。
export function documentHref(
  documentPath: string,
  href: string,
): string | undefined {
  const base = new URL(`https://learning.invalid/${documentPath}`);
  const target = new URL(href, base);
  if (target.origin !== base.origin) {
    return ['https:', 'http:', 'mailto:'].includes(target.protocol)
      ? href
      : undefined;
  }
  const path = decodeURIComponent(target.pathname.slice(1));
  if (!Object.hasOwn(documents, path)) {
    return undefined;
  }
  return `${documentURL(path)}${target.hash}`;
}
