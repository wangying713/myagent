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
