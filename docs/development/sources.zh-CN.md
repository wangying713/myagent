# 来源、版本和译解说明

资料整理与核对日期：2026-10-13。下列提交是本次读取和整理时固定的来源版本，便于以后核查。

## Google 官方 Go 规范

- 官方入口：[Go 风格规范总览](https://google.github.io/styleguide/go/)。
- 来源仓库：[Google 风格规范仓库](https://github.com/google/styleguide)。
- 来源分支：`gh-pages`。
- 来源提交：`0a7d5c8768211371646a3c1beeff8cd224b974be`。
- 许可：[CC BY 3.0 署名许可](https://creativecommons.org/licenses/by/3.0/)。原作者为 Google。本目录明确保留来源和署名，翻译、删减与重新组织由本次整理完成。

| 来源文件 | 中文文件 | 处理方式 |
| --- | --- | --- |
| [总览](https://github.com/google/styleguide/blob/0a7d5c8768211371646a3c1beeff8cd224b974be/go/index.md) | [阅读入口](README.md) | 整理文档层级和阅读顺序 |
| [核心规范](https://github.com/google/styleguide/blob/0a7d5c8768211371646a3c1beeff8cd224b974be/go/guide.md) | [中文核心规范](google-go-guide.zh-CN.md) | 保留主要论述和规则，删减部分示例，个别例子改写成中文 |
| [具体规范](https://github.com/google/styleguide/blob/0a7d5c8768211371646a3c1beeff8cd224b974be/go/decisions.md) | [中文具体规范](google-go-decisions.zh-CN.md) | 按原文主题提炼规则、理由及例外 |
| [最佳实践](https://github.com/google/styleguide/blob/0a7d5c8768211371646a3c1beeff8cd224b974be/go/best-practices.md) | [中文最佳实践](google-go-best-practices.zh-CN.md) | 按常见工程场景重点译解 |

Google 内部的日志设施、审阅流程和工具不能直接视为本项目应安装的东西；中文资料把相关内容标明为内部背景，或仅保留通用原则。译解不构成 Google 的官方中文版本。

## 社区技能来源

### Samuel Berthe 的 Go 技能集

- [来源仓库](https://github.com/samber/cc-skills-golang)。
- 来源提交：`8e899e20ff0cd4dc524af3993e4c62d8ee8c5717`。
- [上游许可](https://github.com/samber/cc-skills-golang/blob/8e899e20ff0cd4dc524af3993e4c62d8ee8c5717/LICENSE)：MIT，作者 Samuel Berthe。
- 已阅读风格、命名、错误、测试、上下文和并发技能正文。
- 本地保留的是中文用途概要、差异比较和重新组织的主题草案，没有复制整套上游技能正文、脚本或参考文件。

### Jeff Allan 的 Go 综合技能

- [来源技能目录](https://github.com/Jeffallan/claude-skills/tree/1be15d8064f88fc25216442406d40add8fd23b53/skills/golang-pro)。
- 来源提交：`1be15d8064f88fc25216442406d40add8fd23b53`。
- [上游许可](https://github.com/Jeffallan/claude-skills/blob/1be15d8064f88fc25216442406d40add8fd23b53/LICENSE)：MIT。
- 已阅读技能正文并查看参考文件目录。
- 本地提供中文用途概要和约束比较，未照搬其覆盖率目标或实现模板。

## Google 官方技能目录

查阅了 [Google 官方技能仓库](https://github.com/google/skills)的公开说明与技能清单。其定位是 Google 产品与技术技能，包含云平台及 Genkit 等专项能力。本次目录检索未发现通用 Google Go 风格规范技能；这只是本次检索结果，不代表所有 Google 仓库都没有相关资料。

## 中文技能的身份与范围

本目录四个 `SKILL.md` 由本次整理编写，主要根据 Google 规范进行中文适配；技能分类参考社区候选。它们不是上游技能的全文翻译，不代表 Google 或社区作者对本项目的背书。

草案中的规范性依据沿用 Google 文档的层级，辅助建议明确结合场景选择。代码示例为简化示意，不是本项目的实际实现。

阅读正文、示例注释和技能描述使用中文。保留的拉丁字符用于代码、命令、网址、作者名称、版本标识及 `SKILL.md` 格式字段，而非英文阅读正文。
