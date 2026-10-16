/** 只调整 JSON 字符串之外的空白，保留数值精度、字段顺序和转义内容。 */
export function formatResponseJSON(body: string): string {
  try {
    JSON.parse(body);
  } catch {
    return body;
  }

  // 字符串作为完整标记保留，其余字符逐个处理，避免重新编码数值。
  const tokens = body.match(/"(?:\\.|[^"\\])*"|[^\s]/g) ?? [];
  const output: string[] = [];
  let depth = 0;
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
        output.push(token);
    }
  }
  return output.join('');
}
