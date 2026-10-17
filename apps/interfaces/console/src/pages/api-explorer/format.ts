/**
 * 响应正文的展示格式化：只调整 JSON 字符串之外的空白，保留数值精度、字段顺序和转义内容。
 * 可选展开字符串里嵌套的 JSON 对象或数组，便于阅读协议正文中被转义的请求和响应。
 */

/** 字符串作为完整标记保留，其余字符逐个处理，避免重新编码数值。 */
const JSON_TOKEN = /"(?:\\.|[^"\\])*"|[^\s]/g;

/** 嵌套展开的层数上限，避免协议正文多层互嵌时无限展开。 */
const MAX_NESTED_LEVELS = 6;

export interface FormatResponseOptions {
  /** 展开字符串里嵌套的 JSON 对象或数组；默认关闭，保留原始转义。 */
  expandNested?: boolean;
}

export function formatResponseJSON(
  body: string,
  options: FormatResponseOptions = {},
): string {
  try {
    JSON.parse(body);
  } catch {
    return body;
  }

  return formatTokens(
    tokenizeJSON(body),
    0,
    options.expandNested === true ? MAX_NESTED_LEVELS : 0,
  );
}

function tokenizeJSON(text: string): string[] {
  return text.match(JSON_TOKEN) ?? [];
}

/** 按当前缩进写出标记，levels 为剩余可展开的嵌套层数。 */
function formatTokens(tokens: string[], depth: number, levels: number): string {
  const output: string[] = [];
  const newline = () => output.push('\n', '  '.repeat(depth));

  for (let index = 0; index < tokens.length; index += 1) {
    const token = tokens[index];
    switch (token) {
      case '{':
      case '[':
        output.push(token);
        depth += 1;
        if (tokens[index + 1] !== '}' && tokens[index + 1] !== ']') {
          newline();
        }
        break;
      case '}':
      case ']':
        depth -= 1;
        if (tokens[index - 1] !== '{' && tokens[index - 1] !== '[') {
          newline();
        }
        output.push(token);
        break;
      case ',':
        output.push(token);
        newline();
        break;
      case ':':
        output.push(': ');
        break;
      default:
        output.push(nestedJSON(token, depth, levels) ?? token);
    }
  }
  return output.join('');
}

/** 字符串里的对象或数组按同一套规则递归展开，其余标记保持原样。 */
function nestedJSON(
  token: string,
  depth: number,
  levels: number,
): string | undefined {
  if (levels <= 0) {
    return undefined;
  }
  const nested = unwrapStringJSON(token);
  if (nested === undefined) {
    return undefined;
  }
  return formatTokens(tokenizeJSON(nested), depth, levels - 1);
}

/**
 * 取出字符串标记内部嵌套的 JSON 文本，只还原定界用的引号和反斜杠，
 * 其余转义和数值保持原文；内容不是完整的对象或数组时返回 undefined。
 */
function unwrapStringJSON(token: string): string | undefined {
  if (!token.startsWith('"') || !token.endsWith('"')) {
    return undefined;
  }

  const nested = token
    .slice(1, -1)
    .replace(/\\(["\\])/g, '$1')
    .trim();
  if (!nested.startsWith('{') && !nested.startsWith('[')) {
    return undefined;
  }
  try {
    JSON.parse(nested);
  } catch {
    return undefined;
  }
  return nested;
}
