# 12 认证、凭证与安全

**DESIGN_DECISION**。管理员密码与卡密使用Argon2id PHC字符串、每条随机16字节salt；起始参数memory=64MiB、iterations=3、parallelism=1，部署测量单次100–300ms目标，不得降到不安全参数以通过性能测试。参考[OWASP密码存储建议](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html)；参数是本项目选择不是原站事实。卡密crypto/rand拒绝采样，不用math/rand、时间戳或卡号派生。输出秘密不写slog、audit、错误栈、OpenAPI示例真实数据。

## 会话与CSRF

256bit随机opaque会话，DB只存SHA256；四个Cookie __Host-jst-admin/voucher/order/preauth，Secure、HttpOnly、Path=/、SameSite=Strict，无Domain。本地HTTP仅APP_ENV=development可使用无__Host前缀且Secure=false的独立cookie名，生产启动强校验。管理员绝对8h/闲置30m，VOUCHER15m，ORDER30m，PREAUTH10m；每请求检查expires/主体version，退出删除。每次提权/登录旋转会话、CSRF；登录前bootstrap提供CSRF随机值并存hash，后续X-CSRF-Token恒时比较，校验Origin同源（不接受任意CORS）。PREAUTH用途单一，challenge5m/5次/一次消费，成功后对应业务检查失败也不重复使用挑战。

管理员临时密码必须force_password_change标记（见数据库补充字段），登录后只准changePassword/logout；修改需current_password+new_password两次前端确认，服务器12..128长度，不截断；更新hash/auth_version并撤销其他session。新会话再登录。最近复核reauth5分钟覆盖生成/导出卡密、重置、管理员/角色变更；不能靠前端字段伪造时间。

ORDER会话scope=READ或WRITE；卡号/订单号+手机查询只授READ且响应脱敏；写修改/售后提交要求WRITE，验证原卡密或下单成功直接签发。已消费卡密只作对应订单step-up，不能让ALREADY_REDEEMED阻断本人查询复核；过期/停用不撤销已成交订单的身份凭证，但密码重置版本变更使旧会话失效。卡密匹配要求卡对应order，不能用其他卡升级。管理员PII读取须order.pii+reason审计；默认列表只展示脱敏信息。

## RBAC与限流

API x-permission是服务端必须权限；角色复选框仅UI。角色变更不得授予操作者不具备的权限；最后有效超级管理能力不可移除（锁identity配置行并计算）；无隐式角色名判断。生产初始账号CLI创建，密码不放Compose或git。

数据库rate_limits共享计数：管理员5次失败/账号/15m+30次/IP/15m；鉴真/订单查询10次/标识HMAC/15m+60次/IP/15m；反馈5次/IP/小时；challenge20次/IP/10m。达到阈值429，不永久停用卡避免攻击者锁死卡。错误卡号和错误密码统一401、同hash计算成本，正确secret后可显示状态。只信受控代理传入IP；HMAC bucket清理24h。不宣称完全消除时序侧信道。

## 卡密导出与“加密”兼容

验证hash长期存储；导出材料是AES-256-GCM短期密文，独立keyring，nonce每文件随机，AAD=batch/voucher ID+format version，24h TTL。元数据和业务同事务，文件先写后关联，孤儿可清理。下载必须file.read+voucher.export+reauth+所属发行权限；仅流式解密，不生成明文磁盘副本、不返回公共签名URL。Cache-Control:no-store；明确Content-Disposition附件；安全CSV转义防公式执行。

seal提交后立刻拒绝新下载，下载和seal以同一文件授权锁协调：读端在事务内验证授权后持有文件访问租约/行锁直到完成流（最长60秒）；seal等待租约清空或超时终止流，不允许封存后新流继续开始。更简单首版允许file行锁覆盖最长60秒流，仅此低频管理动作例外，不能用于常规业务长事务。文件删除失败不恢复权限，Outbox重试。密文仍在备份时无法宣称物理擦除；恢复备份必须重放EXPIRED状态与清理，不恢复导出权限。

reset-secret旧hash立即替换、version递增、active=false、EXPORTABLE、新短期文件；仅未消费卡，旧凭证永不重新导出。无需复制原站未知算法。默认系统不群发卡密，用户交付行为另受权限控制。

## 数据/文件安全

PII姓名手机地址JSON用独立AES-GCM密钥，phone_lookup用独立HMAC密钥，不用可枚举裸hash；HMAC请求摘要再独立key，禁止记录原始请求。密钥版本存数据，密钥以容器只读secret挂载；轮换支持旧解密、新写新版本，后台任务重加密后再退旧钥，先备份验证。

SQL全部占位符；动态sort映射固定常量。上传图片PNG/JPEG实际解码再编码，5MiB/1600万像素上限，禁SVG/HTML，剥EXIF；导入 CSV 仅 UTF-8、5MiB、1000 行，拒 NUL 与超长字段；导出最多 10MiB，并保护公式前缀。私有文件由随机 storage_key 定位，不拼客户端文件名。下载检查归属，报告仅脱敏；首版不开放 PII 扩展导出。

CSP默认self，图片self/data受控，禁止内联脚本；前端v-html只用服务端清洗内容。安全响应头nosniff/frame-ancestors/self；反向代理TLS、body限额，服务端不抓取用户提供URL（防SSRF）。日志不存原始供应商响应。审计仅白名单字段变化，保留对象ID、操作者、结果、request_id。

## 会话撤销补充

DESIGN_DECISION：ORDER会话的subject_id为orders.id；鉴权联查voucher_redemptions取得voucher_id，按sessions.credential_version比较vouchers.credential_version。卡密轮换撤销该卡VOUCHER与ORDER会话；管理员会话比较admin_users.auth_version。读取会话返回force_password_change标记，临时口令状态仅允许changePassword/adminLogout，不允许读取业务模块。Session.csrf_token只在当前浏览器内存持有，严禁日志。下载使用200二进制，不返回201 JSON成功形状；缓存幂等记录仅授权对象ID，绝不缓存卡密字节。

鉴权中间件验证主体、作用域、有效期与凭证版本，不因卡刚刚被成功兑换而直接拒绝有效会话；redeem Service先处理同键重放，再判断卡可兑换性。这样响应丢失后仍可恢复同一订单，不能绕过一次性消费约束。


## A2 实施补充（2026-10-12）

读取 adminSession 时轮换 CSRF 并刷新闲置时间，支持页面刷新后仅凭 HttpOnly cookie 恢复会话；客户端采用响应中的新 CSRF。改密成功仍撤销所有管理员会话，不授予旧会话业务权限。为处理成功响应丢失，改密幂等记录 scope 绑定旧 opaque token 的 SHA256，request_hash 为独立 HMAC（原 CSRF hash + 原始旧/新密码），result_ref 仅管理员 ID/version/http_status。持同一旧 cookie、CSRF、请求体和键，在 24 小时内仅可重放成功 Action；变更请求返回409，不能藉此恢复任何业务会话。记录不含密码或 CSRF 明文。前端待确认请求仅在内存保留，重试不换键或正文。

挑战采用服务端生成四位数字 PNG，无客户端“已通过”标记；数据库记录 HMAC 答案、5分钟 TTL、5次上限。PREAUTH 成功登录时同事务删除该会话挑战后删除 PREAUTH，适应 ON DELETE RESTRICT 外键；原挑战及会话不能重放。验证码不能被视为自动化攻击完全防护，共享数据库限流同时执行。

初始化 CLI 只允许空管理员库，锁 identity_guard 防并发初始化，不提供默认账号口令。SetEnabled 复用该锁和两项能力（admin.write/role.write）的存活检查；管理员/角色编辑接口与页面在 G3 接入。当前 A2 只开放认证端点，领域及管理编辑接口尚未开放。
