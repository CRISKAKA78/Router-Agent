# 来源 IP 摘要与接口状态验证（ADR-048）

2026-09-09，依据用户三项需求及直接实施授权。任务开始时Git工作区干净；本轮修改WPF、相关测试和必要文档，不改Server、Probe或生成器实现，不替换用户进程和运行数据。实现交付时未提交/推送；随后用户明确授权将当前进度提交GitHub，本次归档源码、测试及文档，推送目标为origin/main，排除构建产物和运行数据。

## 完成行为

- 所选设备新增两行：出口IP使用公开`source_ip`，运营商及归属地按该IP异步查询并以运营商/国家·地区·城市展示。IPv4/IPv6来源均只显示一个地址；缺失来源不以Probe双栈出口代替。私网/保留地址不外查，未取得结果显示横杠和具体原因。
- 使用既有IpLocation的HTTPS查询、地址匹配、成功缓存、失败退避和取消机制；来源地址优先查询，切换设备/来源/连接清除旧显示，迟到结果不能覆盖新设备。详细属性仍保留原Probe双栈遥测及其查询，不改变协议语义。
- 设备详情的接口状态下并排外壳端口、系统端口，保留原表格和曲线。末尾同排接口采样时间按钮打开模态弹窗；没有独立采样配置页。关闭/保存后仍停留原端口分组。
- 弹窗支持0～86400秒、模板默认/全部/指定接口，勾选恢复模板默认并保存可清除接口覆盖。取消不提交，错误在框内提示；沿用原profile版本、幂等、离线待下发和不重新应用模板语义。
- 工作区更名为设备详情、远程维护、文件管理、配置管理；仓库工具保持末项。

## 验证与证据

环境：Windows、.NET SDK 10.0.400、现有Go工具链、Python 3.14/PyMuPDF。实际Go Server与TestProbe测试协议对端运行在隔离测试端口和目录。

| 命令或范围 | 结果 |
| --- | --- |
| `build/dotnet10/dotnet.exe build windows/RouterWorkbench.Desktop.Tests -c Release --nologo` | 成功，0警告/0错误 |
| `windows/build-desktop.ps1 -BuildOnly -Verify -OutputName windows-desktop-interfaces` | 当前Server构建、win-x64自包含发布及306项桌面检查通过；`build/interface-workspace/final-verify.log` |
| `D:/Python314/python.exe windows/RouterWorkbench.Desktop.Tests/render.py build/windows-desktop-interfaces/verification` | 32份WPF XPS布局转PNG、文字与图形检查通过；`build/interface-workspace/render.log` |
| `Invoke-RestMethod`调用现有`https://ipwho.is/8.8.8.8`查询参数，8秒期限 | 本机网络超时；没有将测试响应中的运营商/地点当作真实公网查询成功 |
| `git diff --check`、修改文档相对链接检查 | 通过 |

新增回归验证来源地址优先于两个不同的Probe探测地址、查询等待、前设备迟到响应、IPv6、私网和null；使用可控HTTP测试响应验证解析及UI绑定，既有查询缓存/限流/取消测试保持。实际API桌面回归覆盖接口分组同排位置、弹窗打开/取消、非法周期/空接口、离线保存、指定接口及恢复模板默认，并保留文件/配置/维护、重连、重复操作和退出检查。

首轮验证因摘要增加一行导致旧心跳测试固定行号失效；改为按字段名称定位，保留原对象稳定性/通知断言后通过。最终再次执行完整发布与桌面验证。渲染窗口内容时为透明弹窗面板补入实际Window背景，避免深色XPS预览误用白色画布；已查看浅深色接口页、采样弹窗及摘要布局。

## 产物与边界

新版客户端：`build/windows-desktop-interfaces/win-x64/RouterWorkbench.exe`。用户可以打开此独立包，或正常退出旧UI后运行`ui-windows.cmd`从当前源码构建；原Server/Probe无需因本轮UI变更升级。

本轮实际桌面交互由同源码测试宿主执行，使用独立设置路径；未直接启动发布EXE，因其入口会读取并保存用户默认配置、自动连接原Server。未修改用户配置来完成启动验证。厂商实机、物理DPI和干净目标机验收仍独立保留，WPF矢量布局不等于物理截图。

公网归属地受外部查询服务及本机网络可用性影响，本轮实查未成功；失败状态已实现并验证。没有修改Go/C++业务，因此不重跑无关全量Go/C++/ARM/生成器测试；历史公网双栈、ARM、sanitizer等缺口不在本轮宣称通过的范围。

规范：[ADR-048](DECISIONS.md#adr-048-来源-ip-摘要与接口状态弹窗)、[API](API.md)、[架构](ARCHITECTURE.md)。
