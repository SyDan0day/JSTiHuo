# 10 Vue用户端设计

**DESIGN_DECISION**：一个Vite工程、两套layout、按路由拆分用户/后台bundle；Vue3+TS strict、Router、Pinia。API types从OpenAPI生成到src/api/generated；不手写另一套字段。fetch封装credentials:same-origin，自动request_id/CSRF，写请求幂等键由用户动作创建并保存在sessionStorage（只键和非敏感草稿ID，不存地址/secret/token）。Pinia只存内存会话概要/选择；刷新通过session端点恢复，服务端为事实来源。

```text
frontend/src/
  app/{router,guards}.ts
  api/{client,errors,generated}/
  layouts/{PublicLayout,AdminLayout}.vue
  components/{FormField,ErrorBanner,LoadingButton,ProductCard,DatePicker,PermissionGate}.vue
  features/{catalog,voucher,redemption,orders,feedback,admin}/
  stores/{session,redemption}.ts
  styles/{tokens,public,admin}.css
  tests/
```

|页面路由|操作/API operationId|状态与行为|
|---|---|---|
|/|publicContent/publicProducts|轮播/快捷入口/公告，加载/空列表/错误可重试|
|/products、/products/:id|publicProducts/publicProduct|公开目录，不显示任意商品兑换成功按钮|
|/pages/:slug|publicContentPage|about/contact/notice/guide；HTML清洗后受控渲染|
|/verify|bootstrap/createChallenge/verifyVoucher|输入卡号密码→结果；不消费卡，不把鉴真等同登录账号|
|/redeem|verifyVoucher|通过后跳选品；状态不可兑则显示原因|
|/redeem/select|redemptionOptions|ONE_OF单选、N_OF多选；计数且恰好N才继续，quantity固定1|
|/redeem/address|redemptionCalendar/regions|地区级联、姓名手机地址、日期；禁选只是提示|
|/redeem/confirm|redeem|确认后同键提交；超时显示“结果确认中”，原键重试；成功到订单页|
|/orders/query|queryOrder|卡号或订单号+手机+挑战；错误不区分存在性|
|/orders/result|customerOrder|脱敏订单与状态，不显示卡密|
|/orders/edit|orderStepUp/customerOrderEdit|必要时凭卡密复核；完整新地址替换；version冲突刷新提示|
|/orders/tracking|customerShipments/customerTracking|无单号/在途/异常/签收分态；不模拟轨迹|
|/orders/after-sales|customerAfterSale/customerAfterSales|申请、进度；处理结果来自API|
|/feedback|createFeedback|200字符一致，提交成功展示反馈ID，不暗示有订单权限|

同源cookie为HttpOnly，JS只持CSRF随机值。secret输入提交后立即清空，不进URL/日志/localStorage；order READ和WRITE分别处理403/SCOPE_REQUIRED。401回对应认证入口，403显示无权限，409展示冲突，422按field_errors定位，429倒计时Retry-After，5xx保留非敏感选择且恢复按钮。Mutation禁止默认无限重试；只有相同幂等键明确重试。

视觉：用户端沿用红#cc0000、白底、礼品卡、服务承诺、底栏五入口；内容/品牌用自有素材。基准390×844、768、1440三个宽度；移动2列礼品、桌面4列，底部safe-area+实测栏高预留，键盘不挡提交按钮。表单label显式关联、错误aria-live、弹窗焦点陷阱/返回，最小触摸44px。长页图片懒加载但首屏主图优先；固定底栏不遮末尾。参考[原移动证据](../research/evidence/screenshots/02-verify-mobile.png)，不继承历史公告文案和视口截断缺陷。

编辑器上传只接受后台文件API，服务端再次清洗富文本；用户输入禁止v-html。取消点击不会自行撤销已提交订单；网络超时不弹“未下单成功”确定结论。导航旧.html别名可在反向代理301到新公开路由，但交易POST不代理到目标站。
