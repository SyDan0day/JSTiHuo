const fs = require('node:fs');
const readline = require('node:readline');
const YAML = require('../../frontend/node_modules/yaml');
const Ajv = require('../../frontend/node_modules/@redocly/ajv/dist/2020').default;
const source = YAML.parse(
  fs.readFileSync(
    require('node:path').join(__dirname, '../../docs/development/openapi.yaml'),
    'utf8',
  ),
);
const ajv = new Ajv({ strict: false, validateFormats: false });
ajv.addSchema({ $id: 'jst', components: source.components });
const operations = [];

/**
 * compile 用途：编译 OpenAPI JSON Schema，复用已锁定的 Ajv 2020 实现。
 * 参数：schema 为接口定义，引用绑定当前规范。
 * 返回值：验证函数或空；不忽略 required、类型、范围或额外字段约束。
 */
function compile(schema) {
  if (!schema) return undefined;
  const resolved = JSON.parse(
    JSON.stringify(schema).replaceAll('"#/components/', '"jst#/components/'),
  );
  return ajv.compile(resolved);
}
for (const [path, item] of Object.entries(source.paths)) {
  for (const [method, spec] of Object.entries(item)) {
    if (!spec.operationId) continue;
    const responses = new Map();
    for (const [status, response] of Object.entries(spec.responses)) {
      const definition = response.$ref
        ? source.components.responses[response.$ref.split('/').pop()]
        : response;
      responses.set(status, compile(definition.content?.['application/json']?.schema));
    }
    operations.push({
      id: spec.operationId,
      method: method.toUpperCase(),
      path,
      pattern: new RegExp(
        '^' + path.replace(/[.*+?^$()|[\]\\]/g, '\\$&').replace(/\{[^}]+\}/g, '[^/]+') + '$',
      ),
      request: compile(spec.requestBody?.content?.['application/json']?.schema),
      responses,
      success: 0,
      rejected: 0,
    });
  }
}

/**
 * validate 用途：对真实 HTTP 的成功输入及所有 JSON 响应执行契约校验。
 * 参数：input 只通过内存管道传递的请求与响应，不写入文件。
 * 返回值：安全的操作名、状态、字段路径；不返回数据或凭据值。
 */
function validate(input) {
  if (input.finish)
    return {
      ok: true,
      operations: operations.map(({ id, success, rejected }) => ({ id, success, rejected })),
    };
  const path = input.path.split('?')[0];
  const operation = operations.find(
    (item) => item.method === input.method && item.pattern.test(path),
  );
  if (!operation) return { ok: true };
  const success = input.status >= 200 && input.status < 300;
  operation[success ? 'success' : 'rejected']++;
  const errors = [];
  if (success && operation.request && !operation.request(input.request))
    errors.push(
      ...operation.request.errors.map(
        (error) =>
          'request ' +
          error.instancePath +
          ' ' +
          error.keyword +
          ' ' +
          JSON.stringify(error.params),
      ),
    );
  if (!operation.responses.has(String(input.status))) errors.push('response status unspecified');
  const response = operation.responses.get(String(input.status));
  if (response && !response(input.response))
    errors.push(
      ...response.errors.map(
        (error) =>
          'response ' +
          error.instancePath +
          ' ' +
          error.keyword +
          ' ' +
          JSON.stringify(error.params),
      ),
    );
  return { ok: errors.length === 0, operation: operation.id, status: input.status, errors };
}
readline.createInterface({ input: process.stdin, crlfDelay: Infinity }).on('line', (line) => {
  try {
    process.stdout.write(JSON.stringify(validate(JSON.parse(line))) + '\n');
  } catch {
    process.stdout.write('{"ok":false,"errors":["contract validator failed"]}\n');
  }
});
