import { formatResponseJSON } from './format';

it('格式化嵌套响应并保留大整数、小数、指数和重复字段', () => {
  const body =
    '{"id":9007199254740993,"amount":0.123456789012345678901,"large":1e400,"id":9223372036854775807,"data":{"items":[null,false,{},[]]}}';
  expect(formatResponseJSON(body)).toBe(`{
  "id": 9007199254740993,
  "amount": 0.123456789012345678901,
  "large": 1e400,
  "id": 9223372036854775807,
  "data": {
    "items": [
      null,
      false,
      {},
      []
    ]
  }
}`);
});

it('字符串中的空白、结构字符及转义保持原样', () => {
  const text = String.raw`"中文, : {} []  空格\n换行\t制表符\"引号\\反斜杠\u4e2d"`;
  expect(formatResponseJSON(`{ "text" : ${text} }`)).toBe(
    `{\n  "text": ${text}\n}`,
  );
});

it.each([
  '',
  '网关错误',
  '<html>Bad Gateway</html>',
  '{"data":',
  '{"x":1,}',
])('非 JSON 或不完整响应保持原文：%s', (body) => {
  expect(formatResponseJSON(body)).toBe(body);
});

it.each([
  '{}',
  '[]',
  'null',
  'false',
  '9007199254740993',
  '"回答"',
])('保留空容器和顶层值：%s', (body) => {
  expect(formatResponseJSON(` \n ${body}\t `)).toBe(body);
});

it('展开字符串里嵌套的对象和数组', () => {
  const body = String.raw`{"data":{"request":"{\"max_tokens\":512,\"tools\":[{\"type\":\"function\"}]}"}}`;
  expect(formatResponseJSON(body, { expandNested: true })).toBe(`{
  "data": {
    "request": {
      "max_tokens": 512,
      "tools": [
        {
          "type": "function"
        }
      ]
    }
  }
}`);
});

it('嵌套字符串里的转义、大整数和再次嵌套保持原样', () => {
  const body = String.raw`{"content":"{\"id\":9007199254740993,\"note\":\"第一行\n第二行\",\"inner\":\"{\\\"ok\\\":true}\"}"}`;
  expect(formatResponseJSON(body, { expandNested: true })).toBe(`{
  "content": {
    "id": 9007199254740993,
    "note": "第一行\\n第二行",
    "inner": {
      "ok": true
    }
  }
}`);
});

it('默认保留嵌套字符串的转义', () => {
  const body = String.raw`{"request":"{\"a\":17}"}`;
  const formatted = `{
  "request": "{\\"a\\":17}"
}`;
  expect(formatResponseJSON(body)).toBe(formatted);
  expect(formatResponseJSON(body, { expandNested: false })).toBe(formatted);
});

it.each([
  '{"answer":"一共 391 支笔。"}',
  '{"count":"391"}',
  '{"text":"[未闭合"}',
  String.raw`{"text":"{\"a\":1,}"}`,
])('不展开非对象或无效的嵌套内容：%s', (body) => {
  expect(formatResponseJSON(body, { expandNested: true })).toBe(
    formatResponseJSON(body),
  );
});

it('嵌套层数超过上限后保留转义', () => {
  let body = String.raw`{"leaf":"{\"value\":1}"}`;
  for (let level = 0; level < 6; level += 1) {
    body = JSON.stringify({ nested: body });
  }
  const formatted = formatResponseJSON(body, { expandNested: true });
  expect(formatted.match(/"nested": \{/g)).toHaveLength(6);
  expect(formatted).not.toContain('"value": 1');
});
