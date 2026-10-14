# Google TypeScript 代码规范（中文整理）

核对日期：2026-10-14。原作者：Google。依据：[官方 TypeScript 规范](https://google.github.io/styleguide/tsguide.html)。原文采用 [CC BY 3.0 署名许可](https://creativecommons.org/licenses/by/3.0/)，本文件作中文译解、删减、重新组织和示例改写，不是 Google 官方中文版本，也不是全文逐句翻译。

本文整理适合日常前端开发的语言规则。Google 内部构建、生成代码、类与装饰器的专项细节未全部展开，遇到相关问题应查原文。对本项目的例外和补充见[前端具体约定](frontend-decisions.zh-CN.md)。

## 1. 如何理解规则

- **必须、禁止**：原文的强制规则。
- **建议、优先、避免**：原文的推荐规则，需要结合场景判断。
- **允许**：合法选择，不代表所有代码都要这样写。

Google 明确说明，其内部 TypeScript 环境与外部项目存在差异；不能把内部工具和限制直接当成本项目的依赖。[官方适用范围](https://google.github.io/styleguide/tsguide.html)

## 2. 文件与模块

- 文件使用 UTF-8；可读的中文可以直接书写，不需要转成 Unicode 转义。
- 文件中先放必要的版权信息、文件说明，再放导入，最后是实现。
- 使用 ES 模块的 `import`、`export`；禁止为了组织普通代码新增 TypeScript `namespace`。与外部库接轨时存在例外。
- Google 要求具名导出，禁止默认导出，以便导出名称和引用保持一致。
- 只导出其他模块实际需要的内容，不导出可变的全局绑定，例如 `export let currentUser`。
- 不要仅为了归类函数而创建只含静态成员的工具类；直接导出函数和常量。

```ts
export interface UserSummary {
  id: string;
  name: string;
}

/** 返回用于列表展示的用户名称。 */
export function getUserLabel(user: UserSummary): string {
  return user.name;
}
```

同一逻辑项目内，Google 建议优先使用相对导入，并避免过多的父级目录跳转。命名空间导入和具名导入均允许，按可读性选择。只用于类型的符号**允许**使用 `import type`；重新导出类型时使用 `export type`。[模块规则](https://google.github.io/styleguide/tsguide.html#source-file-structure)

**项目例外**：Umi 页面、配置、运行时入口和 Mock 使用框架要求的默认导出；跨页面引用公共代码时使用项目已有的路径别名。不要为了符合这一节批量重写官方示例。

## 3. 命名

| 对象 | Google 规则 | 示例 |
| --- | --- | --- |
| 类、接口、类型、枚举、TSX 组件函数 | 大驼峰 | `UserSummary`、`UserTable` |
| 变量、参数、普通函数、方法、属性 | 小驼峰 | `currentUser`、`loadUsers` |
| 全局常量值 | 大写单词与下划线 | `DEFAULT_PAGE_SIZE` |
| 泛型参数 | 单个大写字母或大驼峰 | `T`、`UserRecord` |

- 名称应表达用途，避免只有当前作者知道的缩写。
- 普通接口不要刻意加 `I` 前缀，例如使用 `UserSummary`，而非 `IUserSummary`。
- 缩写按普通单词参与驼峰命名，例如 `userId`、`loadHttpUrl`。
- 不用前导或尾随下划线表示私有成员、可选参数或未使用参数。
- `const` 声明不意味着变量名都要大写；局部常量仍可使用小驼峰。

Google 文件命名使用小写及下划线的约定；本项目的组件文件、页面目录和生成文件采用现有 React/Umi 结构，具体见项目适配表。[命名规则](https://google.github.io/styleguide/tsguide.html#naming)

## 4. 变量、对象和集合

- 必须使用 `const` 或 `let`，默认使用 `const`；只有重新赋值时才使用 `let`。禁止 `var`。
- 一次声明一个变量，禁止 `let page = 1, size = 20`。
- 使用对象和数组字面量，不用 `new Object()`、`new Array()` 构造普通对象和数组。
- 数组遍历优先使用 `for...of`；需要下标时可用普通 `for` 或 `entries()`。`forEach` 也允许。
- 禁止用 `for...in` 遍历数组；对象遍历优先使用 `Object.keys`、`Object.values` 或 `Object.entries`。
- 展开对象、数组只产生浅拷贝，必须理解后续属性会覆盖前面的同名属性。
- 对象展开不要混入数组、类实例或可能为 `null`、`undefined` 的值。

```ts
const pageSize = 20;
let currentPage = 1;

const filters = {
  ...defaultFilters,
  keyword: searchText,
};
```

## 5. 类型

- 明显可推断的类型不必重复标注，例如 `const pageSize = 20`。
- 空数组、集合或复杂表达式在推断不足时补充类型，使意图清楚。
- 避免 `any`；优先使用具体接口、类型别名或泛型。
- 类型确实未知时使用 `unknown`，在读取属性或调用方法前先缩小类型范围。
- 少数合理的 `any`，例如测试中的局部模拟对象，需要说明原因；不要把例外扩散到业务接口。
- 类型断言 `as` 和非空断言 `!` 不提供运行时检查，不能用它们代替数据校验。
- 必须使用断言时采用 `as` 语法，并让安全依据清楚可见。
- 不要把 `{}` 当成任意对象类型：它还接受非空的基本类型。字典使用 `Record<string, T>`，未知值使用 `unknown`。
- 优先选择能表达需求的简单类型，不为减少少量重复而引入难以理解的条件类型、映射类型组合。

```ts
interface UserSummary {
  id: string;
  name: string;
}

const users: UserSummary[] = [];

function getErrorMessage(error: unknown): string {
  if (error instanceof Error) {
    return error.message;
  }

  // 此函数用于外部输入边界，兼容无法确定类型的异常值。
  return '操作失败，请稍后重试';
}
```

上例的外部输入兜底有明确边界；Google 不建议给确定只抛出 `Error` 的内部函数到处添加对字符串等异常值的防御逻辑。[类型规则](https://google.github.io/styleguide/tsguide.html#type-system)

## 6. 函数和控制流

- 解构参数保持简单，避免在函数参数中进行多层嵌套解构。
- 函数内部用空行划分逻辑段落，不在函数体首尾留无意义空行。
- 控制流通常使用花括号；Google 允许能完整放在一行的 `if` 省略花括号。本项目统一保留花括号。
- 避免在条件判断中赋值；确有需要时通过额外括号明确表达意图。
- 使用 `===`、`!==`。Google 允许用 `value == null` 同时判断 `null` 与 `undefined`，本项目的新代码优先显式表达需要的判断。
- `switch` 包含最后一个 `default` 分支；非空分支必须通过 `break`、`return` 或异常结束，不能意外贯穿。
- 普通字符串使用单引号，插值或多行字符串使用模板字符串。

## 7. 错误处理

- 异常情况优先抛出异常，不通过临时的错误容器绕过正常控制流。
- 使用 `new Error(...)` 或明确的 `Error` 子类，不抛字符串、数字或任意对象。
- `Promise` 的拒绝值也应是 `Error`。
- 捕获后应处理、补充信息或继续抛出；极少数可以忽略的情况需要写明原因。
- 仅在明确知道外部 API 会抛出非 `Error` 值时，才添加对应的兼容处理并说明来源。

```ts
if (pageSize <= 0) {
  throw new Error('分页大小必须大于 0');
}
```

不要把“请求失败”转换成“请求成功但列表为空”，否则调用方无法区分错误与真实空数据。[异常处理规则](https://google.github.io/styleguide/tsguide.html#exception-handling)；这一句关于列表状态的约束是本项目的具体应用。

## 8. 注释与语言边界

- 对外导出的模块 API 用 JSDoc 说明用途；描述明显的参数名、返回值时可以省略重复内容。
- 普通多行说明使用连续的 `//`，API 文档使用 `/** ... */`。
- 注释解释用途、限制和原因，不画装饰框，不复述每条语句。
- 禁止 `eval`、字符串形式的 `Function` 构造器；规范中对代码加载器有专项例外，普通业务代码不使用。
- 不修改内建对象或其原型，不随意给全局对象增加属性。
- 不依赖尚未标准化的语言特性；有特定运行环境要求时，应明确兼容范围。

本项目的注释、说明和运行提示使用中文；代码标识符保留惯用写法。这是项目约定，不是 Google 要求。

## 9. 本项目如何执行

语言原则依据本文，框架例外与具体开发要求依据[前端具体约定](frontend-decisions.zh-CN.md)，排版交给现有 Biome 配置。生成代码和未修改的官方 Demos 不要求在本次文档整理中批量整改。

来源及译解范围见[来源说明](sources.zh-CN.md#前端规范来源)。
