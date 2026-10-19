# 联合社区作弊黑名单网站

参考了 [BFBAN](https://bfban.com/) 的公开首页、玩家数据库、案件状态，以及其 [MIT 开源项目](https://github.com/BFBAN/bfban-website) 的举报、详情和审核流程。此站使用独立 Go / JavaScript 实现、独立标识和数据，不导入 BFBAN 的玩家处罚，不宣称与 BFBAN 或游戏开发商有关联。

## 已实现

首页真实统计与案件动态；玩家搜索、状态／游戏／社区／日期筛选和分页；Steam 登录（首次验证后自动建号）；提交举报；证据链接；案件详情、处理时间线、讨论与隐藏讨论；审核意见投票；人工确认／撤销／自证等十种案件状态；申诉及后台回复；社区加入申请与审批；公告；普通用户、社区审核员、联合管理员权限；账号停用／角色管理；修改密码；审计记录；名单 API；移动端布局与明暗主题。

审核投票记录意见，最终处罚由有权审核员明确保存。仅 `public=true`、`status=active`、有效且未到期的 SteamID64 处罚进入名单。相同 SteamID 多条有效处罚保留最长到期，永久处罚优先。待审核不等于已确认；申诉不会自动撤销原结论。录入者需保留证据，申诉必须登录；非管理员只能为本人已验证的SteamID申诉。

普通用户不支持密码注册或邮件找回，使用Steam官方OpenID 2.0；尚未配置管理员SMTP找回、直接上传图片／视频、第三方对象存储和移动应用；当前证据以文字及外部链接提交。这些外部能力未伪装成已完成。

## 运行

Go 1.26+，纯 Go SQLite 驱动，默认 `127.0.0.1:8088`。`go test ./...`，`go build -o community-blacklist .`，运行二进制。首次随机管理员密码保存在 `.sqlite-initial-password`，用户名默认为 `admin`；文件已存在会停止初始化，避免覆盖凭证。首次登录后修改密码。

环境变量：`DATABASE_FILE`（默认 alliance.db）、`LISTEN_ADDR`、`ADMIN_USERNAME`、`ADMIN_PASSWORD`（可选首次初始化）、`COOKIE_SECURE=true`（HTTPS）、`BASE_PATH=/community`（反代路径）、`TRUST_PROXY_LOOPBACK=true`（只信任本机代理的 X-Real-IP）、`EXPORT_TOKEN`（可选 Bearer 名单密钥）。反代必须保留 Host，删除对外传入的 X-Real-IP 后重新设置。

独立域名不设 BASE_PATH；子路径部署由 Nginx 去掉 `/community/` 前缀后转发，前端 HTML、导航、API 和 Cookie 路径会使用 BASE_PATH。不共用游戏管理站的账号、会话或数据库。

## 数据库与迁移

SQLite WAL，分表保存 users、records、appeals、comments、votes、communities、announcements、audit_events，records 有 SteamID 和公开状态索引。事务与 revision 乐观锁防止并发覆盖，冲突返回失败并要求重新加载。会话在内存中，服务重启需重新登录。

旧 JSON 保持原样。仅对空数据库设置 `IMPORT_JSON=/absolute/data.json` 启动，可保留原账号密码哈希与案件；不得把非空数据库当迁移目标。原 `.initial-password` 不覆盖。D1 和旧 JSON 模式为兼容保留，正式站建议 SQLite。

当前存储接口每次操作加载各表状态，适合早期社区站；海量记录需改为 SQL 条件查询和分页，不能把目前的索引当成已经完成大规模压测。

备份在线数据库使用 SQLite backup API／`.backup`（包括 WAL 中已提交数据），或先停止本站服务再复制数据库；不要运行中只复制主 db 文件。备份同时保留独立初始化密码文件，权限 0600。还原前备份当前数据库，再停止本站、恢复文件、启动本站。此操作不需重启游戏服务。

## API

`GET /api/v1/players?q=&status=&game=&community=&from=&to=&sort=&page=1&size=20`；`GET /api/v1/player/{caseId}`；`GET /api/v1/statistics`；`GET /api/v1/communities`；`GET /api/v1/announcements`。

`GET /api/v1/bans` 返回 `{version:1,generatedAt,cacheHours:24,bans:[{steamId,reason,expiresAt,caseId}]}`。`steamId` 为字符串，永久 `expiresAt=null`。配置 EXPORT_TOKEN 后需 `Authorization: Bearer ...`，密钥不能放在 URL。服务器插件可每日下载一次、缓存24小时；本站部署不自动启用游戏服的预留导入插件。

写入接口要求精确同源 Origin；登录后的写入还要求 X-CSRF-Token。输入限制、限流、社区权限、停用账号、审计均在后端检查。前端以 textContent 展示用户文本，禁止 HTML 注入，不接受 javascript: 证据链接。

## 验证

`go test ./...` 包含 SQLite 持久化、并发 revision 冲突、举报不能自设处罚、普通用户权限、跨社区审核、隐藏讨论、联系人隐私、申诉、永久处罚优先、API 密钥、停用账号、密码变更撤销会话，以及原 JSON/D1 兼容流程。浏览器另检查首页、筛选、举报登录提示、统计和子路径部署。

## 本地与Steam登录（2026-10-12更新）

用户要求仅本地，已移除管理站 `/community/` 反代入口并停止、禁用独立网站服务，原管理站及游戏服务正常。撤下时完整数据库保存在 `local-only/alliance.db`，独立管理员密码保持原值，目录已排除版本库。代码与数据库不再上传或自动部署；服务器历史回滚备份保留。

本地设置 `LISTEN_ADDR=127.0.0.1:8089`、`DATABASE_FILE=local-only/alliance.db`、`PUBLIC_ORIGIN=http://127.0.0.1:8089`、`COOKIE_SECURE=false`、`BASE_PATH=`、`TRUST_PROXY_LOOPBACK=false`。通过 `start-local.ps1` 启动，Steam返回此本机地址。

普通用户及社区审核员只允许Steam登录；管理员密码登录为例外。普通密码注册和非管理员密码登录在后端拒绝。旧普通密码账号不会按昵称自动合并Steam账号，以防错误接管。审核员先Steam登录，再由管理员分配社区权限；管理员可创建其他管理员密码账号。

Steam密码只输入在Valve页面，本站只接收OpenID声明。后端将声明提交到固定HTTPS Steam端点进行 `check_authentication`，校验返回地址、签名覆盖、SteamID、浏览器绑定一次性state、时间nonce、防重放、停用状态；Steam账户初始角色固定为member。没有Web API Key也可做身份认证，此版本不调用昵称和头像Web API，页面显示已验证SteamID。

官方来源：[Steamworks网站身份认证](https://partner.steamgames.com/doc/features/auth#website)、[OpenID 2.0](https://openid.net/specs/openid-authentication-2_0.html)。真实用户的Steam授权由本人完成；自动化测试使用隔离的验证桩，不能证明真实账号已完成Valve授权。

## 链接限制与界面第二版（2026-10-12）

本次仅更新本地站点。前后端共享37组视频链接契约案例，举报、案件管理、评论、申诉、审核意见、公告及社区申请中的外链均限制为HTTPS视频播放页：Bilibili的BV/av视频和YouTube的watch/shorts/live/youtu.be视频。仅精确匹配已列出的平台域名，不接受任意子域名、账号夹带、非默认来源、显式端口、下载页、开放跳转页和嵌套URL参数。B23短链请用户先在平台打开，复制完整视频地址。

保存前统一转换为www.bilibili.com/video/...或www.youtube.com/watch?...，移除分享追踪参数；保留经校验的视频编号、分P和时间定位。普通说明文字和纯文本邮箱可保留，不自动链接邮箱。原数据库不批量修改，旧不合规链接不生成可点击入口。证据卡不嵌入视频、不会自动请求第三方播放器或缩略图。

首页加入原创档案视觉、快捷查询、真实最近案件和材料指引；播放器证据用平台卡展示，举报页即时反馈链接是否合法。来源为站内真实记录，示意图标注非真实玩家案件，未填充虚构统计。

验证：Go链接合同与接口绕过测试、Node前端合同测试、Go vet、前端语法、浏览器的查询跳转／非法链接提示／移动断点。

## 举报分类标签（2026-10-12）

举报表单可多选15个标签，分作弊行为、对局违规、言行违规、其他四组；详细原因和视频证据仍须填写。新举报至少选一个标签，后端按固定目录校验并去重。目录由内嵌的 `web/report-categories.json` 提供，前后端共用，公开接口 `GET /api/v1/report-categories` 返回 `{items:[{id,label,items:[{id,label}]}]}`。

案件中的 `tags` 是稳定标识字符串数组，保存在现有SQLite记录JSON中，不需重建数据库。档案列表、详情及审核后台展示对应中文标签；管理员可修正。历史案件允许无标签，旧API客户端编辑时省略tags会保留原标签，明确传空数组可清除。标签只描述举报行为，不自动改状态或处罚；新举报仍强制待审核。仍只运行本地。

## 单选、个人中心与请求防护（2026-10-12后续更新）

本节替代前节的“多选”规则：举报分类改为一个分组原生下拉，每份新举报必须选择且仅能选择一项。后端拒绝超过一个tags元素，包括重复元素；历史记录不批量更改，管理员可用下拉修正，省略tags的旧客户端仍保留原分类。数据库筛选新增 `tag=`，目录必须是合法固定标签。

个人中心提供自己的举报及待处理筛选、自己的Steam申诉与处理回复、本账号发起的社区申请、有效登录会话查询及撤销。新增 `GET /api/v1/account-summary`、`GET /api/v1/sessions`、`POST /api/v1/sessions/revoke`（body为{id}）、`POST /api/v1/sessions/revoke-others`。会话管理id为摘要，不能用于登录；只列出/撤销本人会话。会话仍保存在进程内，本地服务重启后重新登录。

社区申请的applicant由后端从已验证账号填写，不能由请求指定；公开目录删除联系人及申请人，只有本人个人中心和联合管理员能查看联系资料。历史无applicant的社区不按名字或联系人自动认领。目录的公开案件数从真实公开记录计算，隐藏记录不计入。隐藏案件仅提交人及有该社区权限的审核人员可读取；其他账号和匿名请求返回404，隐藏案件不开放公开讨论。

尚未处理的同人同社区举报阻止重复提交，提示在原案件补充证据；同案件同本人Steam的未处理申诉也阻止重复提交。页面增加复制案件链接及补充材料入口，社区页使用目录和可展开申请表。

请求域名固定为PUBLIC_ORIGIN，未配置时只允许loopback域名/IP，防止外部域名解析到本地服务。写入仍须精确Origin和CSRF，并拒绝浏览器标记的cross-site请求。API每来源IP每分钟240次，返回实际Retry-After；已有各业务限额继续生效。管理员登录在密码哈希计算前检查长度，请求体64KiB、请求头32KiB限额及HTTP超时保留。CSP增加object-src/frame-src限制、X-Frame-Options与Permissions-Policy。

验证包含匿名与跨用户查询、隐藏案件权限、社区申请所有者/批准状态伪造、CSRF拒绝、会话令牌不泄露、跨用户撤销拒绝、本人撤销有效且不影响他人、重复举报/申诉拒绝、域名与请求频率防护。Go测试、vet、37组视频链接前端契约通过；govulncheck报告No vulnerabilities found，指扫描时可达代码所涉及的已知漏洞，不等于完整渗透测试。

参考：[OWASP会话管理](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html)、[OWASP跨站请求防护](https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html)。站点继续只在127.0.0.1:8089运行。

## 品牌与首页原生动效

站点名称统一为社区联合黑名单，旧emblem.svg已撤下。原创brand-mark.svg及brand-wordmark.svg替换页眉、登录、页脚与favicon，详见BRAND.md。

按用户提供的[BFBAN开源首页](https://github.com/BFBAN/bfban-website/blob/3d1b43f7c3c38a3b1ed409204346770ecb036ec0/front/src/views/Home.vue)核对其双组卡片连续滚动与轻度错位布局，独立实现本站动画。没有运行参考仓库的脚本或加载其资源。图解直接使用内联SVG与本站独立CSS，连接线、光点、缓慢轨道及复核返回链连贯运动；桌面横向、手机竖向，文字不简单缩成小图。节点进入本站相应功能，图示不表示实时业务状态。

功能入口卡片以双组同宽轨道无缝循环；真实活动来源于statistics接口，仅不少于3条时滚动，少量记录静态展示。复制组只用于连续视觉，不新增记录或统计。鼠标悬停、键盘焦点暂停滚动，全局暂停按钮可控制全部首页动效。离开视口、切到后台及prefers-reduced-motion时暂停；减少动画时去掉复制组并展开可用入口。没有新增轮询或外部网络调用，CSP保持严格同源。


## 网站页脚与协议（2026-10-12）

页脚集中展示网站服务、规则与协议、资料与接入；API 文档和设计材料的入口位于页脚。新增 `/privacy`、`/terms`、`/reporting-rules`、`/community-guidelines`、`/api-terms`、`/cookies`、`/design`。公开网站和审核后台使用相同页脚。普通用户 Steam 登录与现有权限边界保持原样。

文档内容由 `web/site-docs.js` 渲染，样式为 `web/site-docs.css`，独立于案件接口，数据库故障不应阻止查阅协议。运营方和真实联系方式按用户要求使用占位；本站仍为本地预览。公开上线前应补齐运营信息，依据实际部署复核保存期限、数据范围、请求渠道与协议内容。当前版本没有自动注销或按天清除，不应将条款理解成相关功能已上线。

内容核对参考：[Steam 官方 OpenID 文档](https://partner.steamgames.com/doc/features/auth)、[国务院公报《网络数据安全管理条例》](https://www.gov.cn/gongbao/2024/issue_11646/material/gwygb202429.pdf)。条款按实际功能撰写，未声明获得合规认证。

设计材料页提供本站原创 SVG 图标与横向字标下载、色值和展示说明，并链接 BFBAN 开源参考。首页 SVG 与循环卡片已集成，支持暂停、离屏暂停、键盘访问与系统减少动画偏好。


## 玩家账号汇总（2026-10-13）

参考 [Community Ban List 查询页](https://communitybanlist.com/search/76561199100014505) 的按账号组织方式及其 [FAQ](https://communitybanlist.com/faq)，新增 `/search/{SteamID64}` 玩家账号档案。展示本地公开案件中的昵称、最近更新时间、涉及社区、有效确认/历史到期/待处理/撤销或其他记录。本站未接入该站数据库，不复制其信誉分、风险排名或处罚，也不主动请求 Steam 头像和昵称。没有公开记录不等于账号安全。

新增只读 `GET /api/v1/profiles/{SteamID64}?group=all&page=1&size=20`。group 允许 all、active、expired、pending、resolved；页码最多 1000000，每页最多 50。返回 account、displayName、lastUpdated、asOf、publicOnly、publicTotal、counts、communities、items、total、page、size、group。items 沿用公开 Record 字段并补充 group。返回的昵称和计数始终排除隐藏案件，管理员访问此公开接口也不例外；仅按账号精确匹配，不按名称认领或合并。以查询时刻判定到期，未知到期格式不会列为有效确认；不改变数据库中原始状态。

主页查询支持17位 SteamID64和 `https://steamcommunity.com/profiles/数字ID` 链接，昵称仍使用原列表搜索。案件列表/详情中的 SteamID 可进入账号汇总；举报入口预填公开账号和昵称，不自动提交。提供复制账号和档案链接、分组查询、20/50条分页、刷新、Steam官方主页链接；页脚的协议、设计材料和 API 文档入口继续保留。新增 player-profile.js 与 player-profile.css 均为本站同源资源，严格 CSP 与现有登录、权限及视频链接规则保持不变。

验证：Go 全套测试通过，覆盖隐藏信息与计数不泄漏（含管理员）、精确账号匹配、分组/分页、空账号记录、参数越界/重复拒绝、到期边界与无效日期。14 个 Steam 查询格式契约通过。浏览器验证数字主页链接、公开真实记录、空分组、分页大小、案件双向入口和举报预填；没有实际发送举报或增加业务记录。桌面 1905px、手机 375px 均无横向溢出；尺寸模拟已清除。仍只运行本机。
