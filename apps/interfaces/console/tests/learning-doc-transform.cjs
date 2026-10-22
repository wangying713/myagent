const { createHash } = require('node:crypto');
const { readFileSync } = require('node:fs');
const loadDocuments = require('../config/learning-doc-loader.cjs');

// Jest 沿用构建时的文档读取规则，只把模块导出转换为 CommonJS。
module.exports = {
  // 占位文件不包含文档正文，缓存键须同时覆盖外部文档及读取器的实现。
  getCacheKey(source, filename, options) {
    return createHash('sha256')
      .update(source)
      .update(filename)
      .update(options.configString)
      .update(options.instrument ? 'instrument' : '')
      .update(readFileSync(__filename))
      .update(
        readFileSync(require.resolve('../config/learning-doc-loader.cjs')),
      )
      .update(loadDocuments.call({}))
      .digest('hex');
  },
  process() {
    return {
      code: loadDocuments
        .call({})
        .replace(/^export default /, 'module.exports = '),
    };
  },
};
