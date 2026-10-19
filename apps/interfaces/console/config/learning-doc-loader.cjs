const { readdirSync, readFileSync } = require('node:fs');
const { join, relative } = require('node:path');

// 在构建时读取项目文档，浏览器不请求新的后台接口。
module.exports = function learningDocLoader() {
  // 文档在前端目录之外，重启或重建时重新读取，不登记越界的监听路径。
  this.cacheable?.(false);
  const root = join(__dirname, '../../../../docs');
  const documents = {};
  const readDirectory = (directory) => {
    for (const entry of readdirSync(directory, { withFileTypes: true })) {
      const file = join(directory, entry.name);
      if (entry.isDirectory()) {
        readDirectory(file);
      } else if (entry.name.endsWith('.md')) {
        documents[relative(root, file).split('\\').join('/')] = readFileSync(
          file,
          'utf8',
        );
      }
    }
  };
  readDirectory(root);
  return `export default ${JSON.stringify(documents)};`;
};
