# 隔离写入测试结果

## 实际结论

- **LIVE_OBSERVED**：商品新增表单提交一次，POST `/admin/product/add_check.html`，HTTP 200、`application/json; charset=utf-8`，业务 `code=0`。返回结构 `code:number,msg:string,data:string,url:string,wait:number`。HTTP 成功不代表业务成功。[网络证据 product-submit-observed-N1](evidence/product-submit-observed-network.json)
- **LIVE_OBSERVED**：提交页面显示“很抱歉，当前测试用户操作权限受限。如有需要，请联系管理员获取更多权限。”该提示为页面提示，不能断言它就是本次 JSON 的 msg 原文；响应 msg 未留存。[截图](evidence/product-submit-observed-masked.png)
- **LIVE_OBSERVED**：提交后以 `name=JSTEST261011-A` 查询商品列表，匹配记录0，并有空列表提示；未观察到商品创建。[筛选请求](evidence/product-verify-network.json)、[核对结果](evidence/product-absence.json)
- **INFERRED**：结合权限提示、业务 code=0 和列表结果，当前账号的商品新增受限；没有足够证据认定所有其他写入接口同样被拒绝。
- **UNKNOWN / NOT_EXECUTED**：批次、卡券和订单创建、库存变化、停用接口均未执行。未得到本轮测试商品ID，不使用已有业务商品替代，不绕过权限。

## 提交内容与调用链

**LIVE_OBSERVED**：专用名称 `JSTEST261011-A-隔离测试勿发货`，编码 `JSTEST261011-A`，状态选择0（停用）。未上传图片、填写外部规格或个人信息。

实测请求参数均为表单字符串：pro_name、pro_img、spbm、spec_no、spec_name、s_time、e_time、no_area、pro_content、p_sort、status。参数值不写入网络记录。调用链：真实表单点击 → layui submit → `/admin/product/add.html:565` → jQuery POST/ajax/send。[同一请求的 Initiator](evidence/product-submit-observed-network.json)

此端点由历史 STATIC_CANDIDATE 提升为 LIVE_OBSERVED（失败响应）；不能提升为成功创建已验证。

## 执行与清理台账

|项目|状态|
|---|---|
|前两次自动化动作|只填字段/选择停用；因隐藏页面及提交控件为input而定位失败，未到达提交点击，不计作业务请求|
|第三次动作|可见表单点击实际input按钮；捕获一次POST，业务code=0|
|已确认新建商品/批次/卡券/订单|0 / 0 / 0 / 0|
|需停用的已确认新记录|无；列表未出现测试商品，无ID可操作|
|删除、导出、通知、发货、权限修改|未执行|
|凭证、Cookie、卡密|未保存|

**UNKNOWN**：不掌握服务器日志及内部数据库，不能以列表为空证明服务器内部绝无副作用。保留本地脱敏审计；原站是否记录本次失败操作未知。

## 下一步条件

1. 由用户自行切换到具备商品新增及测试数据停用权限的合法测试账号，或由管理员处理权限；不提供密码/Token给Agent。当前授权及隔离/恢复方案继续有效，无需再次授权同类测试。
2. 商品可创建后继续小批次多选一/多选多测试；验证码出现时交由用户完成，不绕过。
3. 订单提交前仍需验证无真实通知/物流副作用，且有专用收件测试资料；本轮未确认，不能宣称已具备条件。

开发影响：商品/API框架可按已有规格推进；多商品权益、核销时点、库存扣减和订单状态转换仍需测试数据验证。没有编写业务代码。
