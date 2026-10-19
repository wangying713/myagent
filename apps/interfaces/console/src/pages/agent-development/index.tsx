import { PageContainer } from '@ant-design/pro-components';
import type { ComponentProps, XMarkdownProps } from '@ant-design/x-markdown';
import XMarkdown from '@ant-design/x-markdown';
import '@ant-design/x-markdown/es/XMarkdown/index.css';
import { Link, useLocation, useModel } from '@umijs/max';
import React, { useEffect } from 'react';
import { agentDemos } from './data';
import {
  defaultDocument,
  documentHref,
  documents,
  documentURL,
} from './documents';
import RequestPanel from './request-panel';
import './index.css';

function selectedDemo(pathname: string) {
  return (
    agentDemos.find((demo) => pathname === `/agent-demos/${demo.id}`) ||
    agentDemos[0]
  );
}

function selectedDocument(search: string, pathname: string): string {
  return (
    new URLSearchParams(search).get('doc') || selectedDemo(pathname).document
  );
}

function DocumentLink({ children, domNode }: ComponentProps) {
  const location = useLocation();
  const source = 'attribs' in domNode ? domNode.attribs.href : undefined;
  const href = source
    ? documentHref(selectedDocument(location.search, location.pathname), source)
    : undefined;
  if (!href) {
    return <span title="此文件请在项目目录中查看">{children}</span>;
  }
  if (href.startsWith('/agent-demos/')) {
    return <Link to={href}>{children}</Link>;
  }
  return (
    <a href={href} target="_blank" rel="noopener noreferrer">
      {children}
    </a>
  );
}

const components = {
  a: DocumentLink,
};

const markdownConfig: XMarkdownProps['config'] = {
  renderer: {
    heading({ depth, text, tokens }) {
      const id = text
        .toLowerCase()
        .replace(/[^\p{L}\p{N}_\s-]/gu, '')
        .trim()
        .replace(/\s/g, '-');
      return `<h${depth} id="${id}">${this.parser.parseInline(tokens)}</h${depth}>`;
    },
  },
};

export default function AgentDemos() {
  const location = useLocation();
  const demo = selectedDemo(location.pathname);
  const documentPath = selectedDocument(location.search, location.pathname);
  const content = Object.hasOwn(documents, documentPath)
    ? documents[documentPath]
    : undefined;
  const { initialState } = useModel('@@initialState');
  const isDark = initialState?.settings?.navTheme === 'realDark';

  useEffect(() => {
    const frame = window.requestAnimationFrame(() => {
      if (location.hash) {
        document
          .getElementById(decodeURIComponent(location.hash.slice(1)))
          ?.scrollIntoView();
      } else {
        window.scrollTo(0, 0);
      }
    });
    return () => window.cancelAnimationFrame(frame);
  }, [documentPath, location.hash]);

  return (
    <PageContainer
      title={documentPath === demo.document ? demo.title : '学习文档'}
      subTitle="Agent Demos"
    >
      <section className="agent-docs" data-theme={isDark ? 'dark' : 'light'}>
        <nav className="agent-docs-nav" aria-label="学习文档">
          <Link
            to={documentURL(defaultDocument)}
            aria-current={documentPath === defaultDocument ? 'page' : undefined}
          >
            DeepSeek 官方示例与课程依据
          </Link>
          <Link
            to={documentURL('learning/README.md')}
            aria-current={
              documentPath === 'learning/README.md' ? 'page' : undefined
            }
          >
            完整学习路线
          </Link>
        </nav>
        <div
          className={
            documentPath === demo.document
              ? 'agent-demo-layout'
              : 'agent-demo-layout agent-demo-doc-only'
          }
        >
          <article className="agent-docs-content">
            {documentPath === demo.document && (
              <header className="agent-demo-source">
                <p>{demo.description}</p>
                <a
                  href={demo.sourceURL}
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  {demo.sourceTitle} ↗
                </a>
                <span>
                  {demo.projectExample
                    ? ' · 项目练习，原文作为相关依据'
                    : ' · 基于 DeepSeek 官方用法'}
                </span>
              </header>
            )}
            {content === undefined ? (
              <p>没有找到这份文档，请从上方入口重新选择。</p>
            ) : (
              <XMarkdown
                key={documentPath}
                components={components}
                config={markdownConfig}
                escapeRawHtml
              >
                {content}
              </XMarkdown>
            )}
          </article>
          {documentPath === demo.document && (
            <RequestPanel key={demo.id} demo={demo} />
          )}
        </div>
      </section>
    </PageContainer>
  );
}
