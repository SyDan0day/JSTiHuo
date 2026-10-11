# 17 公开后台登录页调查

## 访问范围

**OBSERVED**：用户提供的后台入口正常加载，文档标题“控制台”；实际路径`https://demo.haotihuo.com/admin/login?key=[REDACTED]`，本阶段仅保留key参数名。REA目标及脚本：[admin-capture](evidence/phase2/rea/admin-capture.json)（脱敏衍生证据）；受控导航请求：[admin-scenario-result](evidence/phase2/rea/admin-scenario-result.json)，Evidence `ev_701d4ef3f7e83ed5dce88495600d02cd043901963c40c1f1d7c853746dce4db0`。

**OBSERVED**：登录页面GET和`/admin/login/yzm.html`图片GET均200；总计11个加载请求。没有登录POST、账号填写、验证码识别/提交、注册、找回密码或管理数据操作。后台返回的页面名称不能证明当前已经登录。

## 表单、事件与脚本

下表为 **INFERRED（公开HTML与捕获JS）**，不是已经执行的登录功能测试。来源：[脱敏HTML](evidence/phase2/admin-login.sanitized.html)、[捕获内联副本](evidence/phase2/admin-login-inline.js)、[导出manifest](evidence/phase2/admin-scripts/manifest.json)。

|项|静态证据|
|---|---|
|表单|`form#formlogin`，没有显式action/method；按钮type=button，主要走Ajax而非原生submit|
|用户名|id `Username`、name `name`、type text、maxlength32，提示用户名/邮箱/手机|
|密码|id `Password`、name `pass`、type password、maxlength32|
|验证码|输入id `validate`、name `yzm`、type text、maxlength4；图片id `yzm`，与前台隐藏yzm含义不同|
|入口事件|`.btn-login.click(function(){…})`；document keydown拦截Enter并触发该按钮|
|前端校验|局部读值使用trim，依序判用户名、密码、验证码空值；分别写info_msg并focus；serialize发送的仍是原输入值，trim未写回表单|
|提交|`$.ajax({type:'POST',dataType:'json',url:'/admin/login/login_check.html',data:$('#formlogin').serialize()})`；字段name/pass/yzm|
|加载态|beforeSend显示“登录中...”并禁用按钮|
|成功候选|`data.code === 1`显示成功跳转文案，location到`/admin/index/index.html`|
|业务失败候选|显示data.msg，调用reloadcode，恢复按钮|
|网络失败候选|显示“网络错误，请重试”，恢复按钮|
|验证码刷新|reloadcode给`/admin/login/yzm.html`附时间戳并清验证码输入；初始图片实际加载，刷新功能没有执行|
|虚拟键盘|keyboard.js构建软键盘，`.v_keyword`切换显示；没有输入测试|
|找回密码|LoadUrl打开iframe `/admin/login/edit.html`；未打开，表单/发送机制未知|
|测试账号入口|提示链接指向外部`https://demo.fwxt.cn/haotihuo?url=demo.haotihuo.com`；仅发现链接，未注册或进入外站|

**UNKNOWN**：登录接口实际状态/响应、账号权限、锁定/限流、验证码有效期、key的服务端语义、登录后的会话方式。没有把用户提供的key当作测试账号或认证绕过手段。

## 管理功能的验证状态

|目标模块|当前证据与结论|阻塞条件|
|---|---|---|
|商品管理|**UNKNOWN** 未进入，无可验证菜单/CRUD字段|合法测试账号及授权范围|
|卡券类型与批次|**UNKNOWN** 未进入，不从前台卡号推定后台字段|同上，卡型/批次测试配置|
|多商品配置|**UNKNOWN** 未进入，不从营销文案推断配置规则|同上，明确只读配置案例|
|订单与发货管理|**UNKNOWN** 未进入|同上；不得执行创建、修改、发货|
|售后管理|**UNKNOWN** 未进入|测试工单只读访问|
|用户与角色权限|**UNKNOWN** 未进入|测试角色和权限矩阵；不能把菜单可见当作服务端授权证明|

**OBSERVED**：本阶段已验证的是公开登录页面和静态资源可加载；经过登录后验证的管理功能数量为0。当前没有合法测试账户，停止在登录前边界。没有尝试常见用户名/默认密码，没有扫描后台路径，没有请求成功回跳目标。
