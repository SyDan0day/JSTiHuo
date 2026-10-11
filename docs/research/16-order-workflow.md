# 16 订单查询与后续流程

本文件使用 OBSERVED / INFERRED / UNKNOWN 区分实测、源码推导和缺证；STATIC_CANDIDATE 只表示源码候选。没有提交查询凭证，没有进入已认证结果页，也没有执行订单修改。

## 证据

- V：[`card_verify` 页面事件脚本](evidence/phase2/verify-scripts/files/isolated/source-2-d7861d2ab6797d1ee14740b8c9c2961e6103a10e4e6b8430efa387444e673a21.js)，第 2–59 行；[导出 manifest](evidence/phase2/verify-scripts/manifest.json)。
- E：[`card_exchange` 页面事件脚本](evidence/phase2/exchange-scripts/files/isolated/source-2-9927d3c515472ccf6886827a294e52a7df80046718f7a18347ec97622d9b29bd.js)，第 2–57 行；[导出 manifest](evidence/phase2/exchange-scripts/manifest.json)。
- O：[`order_query` 页面事件脚本](evidence/phase2/order-scripts/files/isolated/source-3-db4d82038ed4145fe60e1e6cff40ead1c7b0a03f8fd36400ac5e1eef18194768.js)，第 2–46 行；[导出 manifest](evidence/phase2/order-scripts/manifest.json)。
- C：[`validate.js?v=88`](evidence/phase2/verify-scripts/files/isolated/source-5-7addd84b3be28cc706ebbf921edf30973d349630121b56934988259d141bbc1f.js)，压缩文件第 1–3 行；用下文函数名定位。

- H-O：[第一阶段订单查询 HTML](evidence/static/order.sanitized.html)，183–224 行。
- A-O：[第一阶段订单空输入](evidence/ax/04-order-empty.txt)。
- H-F：[第一阶段售后反馈 HTML](evidence/static/feedback.sanitized.html)，181–216、408–470 行。

## 已确认信息的范围

**OBSERVED**：公开查询页提示“可查询订单状态和物流轨迹，可修改未发货订单”；清空表单提交出现“请输入提货卡号或订单号”。这是页面承诺及空输入行为，不能升级为已验证的查询、物流或修改成功。（A-O；[第一阶段页面记录](02-page-details.md)）

**OBSERVED**：本阶段通过 REA 捕获并导出订单页面 O 和共同 C 脚本，manifest 保留来源及 hash。**INFERRED**：由这些源码可确认客户端设计包含“空值检查→拼图→查询预检 Ajax→POST 结果页”的链条。**UNKNOWN**：服务端成功响应、结果页内容和订单实际业务状态。网络实测独立见 [14](14-network-evidence.md)。

## 查询字段与请求构造

| HTML name / JS 选择器 | 类型和限制 | 判断与来源 |
|---|---|---|
| `bianma_jd` / `#bianma` | string；type=email、maxlength=32、autocomplete=off；提示订单号，空错误允许“卡号或订单号” | INFERRED，H-O 200、O 3–9 |
| `phone` / `#phone` | string；type=tel、maxlength=11；收货手机；inline 仅检查非空 | INFERRED，H-O 208、O 4、10–13 |
| `__token__` | hidden string，值不记录；进入序列化 | INFERRED，H-O 184 |
| 无 name / `#yzm` | 本地拼图完成标记；不进入 serialize | INFERRED，H-O 184、C |

**INFERRED**：序列化 payload 名称是 `bianma_jd`、`phone`、`__token__`；不能误记为 JS 局部变量 `bianma`。订单页没有反馈页的手机号正则，HTML tel 也不等同于校验 11 位中国手机号码。（O 2–23；H-O）

## 调用过程

1. **INFERRED**：`#smt` 匿名 click 回调先读取字段；空订单标识提示“请输入提货卡号或订单号”，空手机提示“请输入收货手机”，并 `return false`。（O 2–13）
2. **INFERRED**：`#yzm` 为空时程序化触发 `.pintu.click()` 并返回。共享 C 实现挑战图片加载、滑块提交与成功通知；订单页 `$HTH.onsuccess` 延迟约 1000ms 再触发 click。验证码细节及候选端点见 [15](15-voucher-workflow.md)。（O 14–18、42–46）
3. **INFERRED / STATIC_CANDIDATE**：禁用按钮、显示“请稍候...”；`$.post('/home/index/order_query_yzm.html', $('#order-form').serialize(), callback)`。它是 AJAX POST 候选，未显式指定 dataType；实际请求编码、响应类型和认证条件须由实测证明。（O 21–23）
4. **INFERRED / STATIC_CANDIDATE**：当 `d.code==1` 调 `$('#order-form').submit()`；H-O 的 action 是 `/home/index/order_info.html`，method=post，因此是 **POST 表单导航**，不是 Ajax 获取结果、不是 GET 跳转。（O 24–26；H-O 183）

| 预检回调分支 | 字段及 UI 恢复 | 证据 |
|---|---|---|
| `code==1` | 尝试提交结果页表单；随后仍执行拼图提示复位、按钮恢复 | INFERRED，O 24–26、35–36 |
| `code==2` | 清空 `#yzm`，保留订单号/手机，弹出 `d.msg` | INFERRED，O 27–29 |
| 其他 | 清空订单号、手机、`#yzm`，弹出 `d.msg` | INFERRED，O 31–33 |
| 全部回调收尾 | `.pintu` 提示复位；按钮文案变为“查询进度”（初始 HTML 为“立即查询”），解除 disabled | INFERRED，O 35–36；H-O 222 |
| 网络失败 | 未见 `.fail()`、`.always()` 或超时恢复，可能保持禁用 | INFERRED，O 23–37；未故障注入 |

**UNKNOWN**：`code` 类型与业务语义；`code==2` 是否代表验证码失效、查询失败或其它原因。脚本只给出了分支动作，不能给返回码补写语义。`msg` 预期供弹窗展示，但真实 response schema 未捕获。

## 结果页、物流与修改

| 能力 | 当前证据边界 | 验证所需条件 |
|---|---|---|
| 结果页渲染 | UNKNOWN；只有 POST action 候选，没有成功页脚本 | 有效沙箱订单标识与收货手机、允许只读查询 |
| 订单状态枚举/字段 | UNKNOWN；不能把查询按钮或 `code` 当作订单状态 | 代表各状态的脱敏测试订单 |
| 物流承运商、运单号、轨迹格式 | UNKNOWN；只有页面功能说明 | 可公开展示的沙箱物流记录，允许被动捕获 |
| 修改入口与字段 | UNKNOWN；页面称“未发货订单”可修改，未见真实入口或限制代码 | 授权结果页只读观察；若要保存修改需另行沙箱授权 |
| 已发货/取消/部分发货/多包裹 | UNKNOWN | 明确业务契约及对应测试数据 |

**INFERRED（新系统自主设计建议）**：“待处理→待发货→已发货→已完成”等状态只可作为后续设计讨论，不能写成原站已确认枚举。修改权限应由服务端结合状态和操作者身份判定；这也是设计要求，不是对原服务端实现的描述。

## 售后反馈与订单的关系

**INFERRED / STATIC_CANDIDATE**：H-F 441 行的 `#smt` 匿名 click 回调向 `/home/index/feedback_send.html` POST `#order-form.serialize()`。具名字段为 `order_sn`、`xm`、`phone`、`email`、`message`、`__token__`；`order_sn` 前端未强制填写，不能据此认定后台一定绑定有效订单。（H-F 181–216、408–457）

**INFERRED**：反馈页依次校验姓名非空、手机非空及 `/^1[34578]\d{9}$/`、留言非空、拼图标记；这套手机规则不能迁移为原订单查询规则。成功 `code==1` 调 `clearForm()`（原生 reset），展示 msg，用户确认弹窗后导航 `/home/index/index.html`；其他 code 弹出 msg。回调共同恢复按钮与拼图文案，但失败分支未显式清空 `#yzm`。这是静态恢复差异，尚未实际提交反馈。（H-F 411–470）

**UNKNOWN**：反馈是否产生工单、与订单的服务端关联、售后状态机及后台处理权限。无需为了补全这部分而提交真实反馈；应由管理员提供规则或沙箱环境。

## 可开发部分与阻塞

**INFERRED（新系统自主设计）**：可先制作查询输入、可访问的表单标签、前端错误提示、loading/网络失败恢复和独立结果展示组件；结果组件字段需保留待定契约。**UNKNOWN**：查询授权规则、真实状态/物流 schema 和可修改字段集合，阻塞查询后端兼容实现及修改订单功能。不得照搬示例候选端点接入真实站点。
