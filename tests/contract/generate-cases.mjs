import fs from 'node:fs';
import { createHash } from 'node:crypto';
import YAML from '../../frontend/node_modules/yaml/dist/index.js';
const raw = fs.readFileSync(
  new URL('../../docs/development/openapi.yaml', import.meta.url),
  'utf8',
);
const specification = YAML.parse(raw);
const operations = [];
for (const [path, item] of Object.entries(specification.paths)) {
  for (const [method, operation] of Object.entries(item)) {
    if (!operation.operationId) continue;
    operations.push({
      id: operation.operationId,
      method: method.toUpperCase(),
      path,
      security: operation.security ?? specification.security ?? [],
      permission: operation['x-permission'],
      json_body: !!operation.requestBody?.content?.['application/json'],
      query: (operation.parameters ?? [])
        .filter((parameter) => parameter.in === 'query')
        .map((parameter) => parameter.name),
    });
  }
}
const target = new URL('../../backend/tests/integration/testdata/operations.json', import.meta.url);
fs.mkdirSync(new URL('.', target), { recursive: true });
fs.writeFileSync(
  target,
  JSON.stringify(
    {
      generated: '由 OpenAPI 自动生成；禁止手工编辑',
      sha256: createHash('sha256').update(raw).digest('hex'),
      operations,
    },
    null,
    2,
  ) + '\n',
);
console.log(`${operations.length} 个操作的鉴权与边界回归输入已生成`);
