const SiteDocuments=(()=>{
  const version='2026-10-12';
  const routes={
    '/privacy':{title:'隐私政策',intro:'说明我们处理哪些信息、为什么处理，以及哪些资料会公开。',sections:[
      {id:'scope',title:'适用范围与运营信息',paragraphs:['本政策适用于“社区联合黑名单”的玩家档案、举报、案件讨论、申诉和社区协作服务。目前为本地预览版本，运营方名称与公开联系渠道均待补充；上线前应以实际运营信息替换占位内容。','我们以身份验证、证据核查、记录审核和接受复核为处理目的。不会仅因有人提交举报就自动认定作弊；网站不直接连接游戏服务器执行处罚。']},
      {id:'collection',title:'我们处理的信息',table:{head:['信息类型','用途与可见范围'],rows:[['Steam 身份与账号权限','Steam 返回验证后的 SteamID64。本站保存账号标识、角色、所属社区和停用状态；当前不主动获取 Steam 好友、购买记录、头像或游戏库存。'],['举报与公开案件','玩家昵称、被举报 SteamID64、游戏、社区、分类、原因、视频链接、时间、状态与审核记录。标为公开的案件可被任何访客查询。'],['案件讨论与审核意见','提交人账号标识、内容、时间及审核意见可随公开案件展示。Steam 登录账号的本站名称含 SteamID，请勿将其视为匿名举报。'],['申诉材料','案件编号、本人 SteamID、申诉内容、时间和回复。仅本人及有权限的管理员或案件所属社区审核员可查看。'],['社区申请','社区名称、介绍、视频地址及联系人信息。批准后的名称、介绍与视频可公开；申请人及联系方式不进入公开目录。'],['会话与安全信息','登录会话、认证状态、CSRF 校验值，以及用于限流的连接 IP。管理员密码只保存加盐派生值；本站不收集 Steam 密码。']] }},
      {id:'storage',title:'保存与删除',paragraphs:['当前数据保存在本机数据库中，服务仅监听本地地址。本版本没有启用云数据库同步、第三方广告分析或后台遥测。','登录会话最长有效 8 小时；认证中间状态最长 10 分钟。会话和限流状态保存在服务内存中，服务重启后清空；主题偏好保存在你的浏览器中。','账号、案件、讨论、申诉、社区资料与操作记录会持续保存，当前版本尚未实现按天自动清除。保存期限应按案件核查、申诉复核、审计需要和适用法律确定，不以无限保留为默认政策；必要性消失后应由管理员删除、匿名化或停止非必要处理。公开隐藏与处罚到期不等于已从数据库删除。','本地备份如含个人信息，应受同样的访问限制，并随实际保存期限清理。我们不承诺已上线当前代码尚未具备的自动注销、自动到期删除或备份擦除功能。']},
      {id:'sharing',title:'公开、共享与第三方',paragraphs:['公开案件包含玩家标识、证据说明及审核过程。请在提交前去除真实姓名、电话号码、住址、证件、聊天中的无关个人信息和任何密码或密钥；不要把第三人的敏感信息作为无关证据公开。','名单接口只导出已确认、公开、SteamID 有效且未到期的记录，包括 SteamID、原因、到期时间和案件编号。未确认、申诉中、撤销、隐藏和到期记录不进入该接口。接入方可能保存最长 24 小时的缓存，网站更新后不会瞬间改变接入方已有的副本。','Steam 登录会跳转至 Valve 的认证页面；本站将认证响应送至固定的 Steam 官方地址核验。Bilibili 和 YouTube 视频以外链提供，本站不自动抓取或嵌入播放器。点击后由对应平台处理访问、账号及 Cookie。','不出售个人信息，不将举报材料用于广告画像。第三方接入社区须独立承担其处理活动责任；法律要求提供信息时，仅在合法要求和必要范围内处理。']},
      {id:'rights',title:'查询、更正、删除与投诉',paragraphs:['你可以在个人中心查看自己的举报、申诉和社区申请，查看或撤销登录会话；案件对应的 Steam 账号可以提交申诉与复核材料。','如需更正、复制、删除信息、注销账号或限制公开，请通过下方运营联系渠道向管理员说明请求类型及案件编号。联系渠道仍为占位时，可在本地测试中直接向网站维护者提出；不要把隐私请求或身份材料发到公开案件讨论。','我们应在核验账号归属、确认必要范围后处理请求；不得要求与请求无关的敏感证明。删除与审计保存发生冲突时，应说明依据、保存范围和期限，不能仅以“网站规则”拒绝法定权利。','当前尚无自助注销和数据导出按钮，这类请求需人工处理。你仍可停止使用网站并撤销会话；停止使用不会自动删除已提交记录。']},
      {id:'safety',title:'保护措施与未成年人',paragraphs:['我们使用权限隔离、登录会话校验、请求限流、来源验证及视频地址白名单保护数据。保护措施不能消除全部风险；发现误公开、账号异常或泄露应立即联系管理员，必要时先隐藏相关内容并限制访问。','请勿提交未成年人真实身份等无关敏感信息。不满 14 周岁者涉及个人信息的操作应由监护人参与；发现缺少所需授权的材料，应停止非必要处理并联系监护人或采取适当删除措施。']},
      {id:'updates',title:'政策更新',paragraphs:['本页标注版本日期。处理目的、公开范围、接收方或权利渠道发生重要变化时，应在生效前通过公告等方式说明，并在需要时重新取得授权；不会以更新条款为由扩大历史材料的使用范围。']}
    ]},
    '/terms':{title:'服务协议',intro:'约定网站的使用边界、内容管理权限和各方责任。',sections:[
      {id:'service',title:'服务性质与协议范围',paragraphs:['社区联合黑名单提供证据提交、公开档案、人工审核协作、申诉复核和名单查询服务。本站为独立社区网站，与游戏开发商、Valve 或 BFBAN 无隶属或官方授权关系。','网站结论用于社区管理参考，不是司法认定。任何“待审核”“讨论中”“待自证”等状态均不等于确认作弊；接入服务器应自行评估证据、规则、适用范围和后续变更。','本协议、隐私政策、举报与申诉规则、社区协作规范和名单接入条款共同说明服务规则。隐私事项以隐私政策为准；适用法律的强制性规定优先。仅浏览公开资料不会授权我们公开你的非公开信息。']},
      {id:'account',title:'账号与使用资格',paragraphs:['普通用户和社区审核员只使用 Steam 登录。管理员账号可以使用密码登录。首次通过 Steam 验证后会建立本站账号；登录方式不证明用户一定诚实，也不证明被举报者违规。','你应保护 Steam 账号与管理员凭证，不得出租、出借账号、冒用他人身份、绕过角色权限或以多个账号干扰审核。发现异常应撤销可疑会话并向管理员反映。']},
      {id:'conduct',title:'禁止行为与网站管理权限',paragraphs:['禁止伪造证据、恶意举报、重复刷屏、操纵审核、骚扰他人、泄露隐私、侵害知识产权，以及探测、攻击、批量滥用接口或传播恶意程序。','为维护服务与证据可信度，网站可核验材料、要求补充说明、退回或拒绝无效申请，隐藏或移除涉嫌违规内容，限制提交频率，暂停或停用滥用账号，并撤销社区或审核权限。紧急安全事件可先限制访问，随后记录处理原因并接受复核。','网站保留安排维护、修复漏洞、调整界面或接口、停止某项服务的权利。可预见的重要变化应提前公告；对正在处理的案件应说明影响和可行的查询或复核方式。']},
      {id:'content',title:'提交内容与必要使用许可',paragraphs:['你应确保提交内容真实、关联、来源合法，并有权提供。仅授权网站在核查、展示已公开案件、保存审核历史与接受申诉所必要的范围内使用，不转移你的版权。','必要许可为非独占许可，允许为上述目的存储、整理和展示材料摘要与合法视频外链；不允许将材料出售、用于商业宣传或不相关的模型训练。未公开申诉和联系方式不因该许可变为公开。','网站可按规则对不实、无关、违法或泄露隐私的内容予以限制，但不保证对每一条内容实时核验。禁止因举报人同意而擅自公开第三人的敏感信息。']},
      {id:'decisions',title:'审核与复核',paragraphs:['网站有权根据材料、独立审核建议和社区规则更新案件状态，不保证每份举报被受理、在固定时限内结案或作出特定结论。证据不足时应保留不确定状态；分类标签仅表示举报主张。','当事人可按规则申诉，更正事实或提出利益冲突。网站可保留必要审核历史以解释结论变化，但应减少不必要的公开个人信息。']},
      {id:'risk',title:'服务可用性与责任边界',paragraphs:['Steam 或视频平台故障、链接失效、网络中断、维护、安全事件和接入方缓存可能影响服务。网站不保证永久在线、记录完整或第三方平台内容一直可用，也不保证名单没有误判。','在法律允许范围内，网站不承担由第三方独立决定的服务器处罚、用户违法提交或擅自传播资料所导致且与本站无可归责行为无关的损失。对本站依法应承担的责任，不以本协议作全面排除，也不免除故意或重大过失造成损害等法定责任。','恶意提交、越权访问或其他违约造成实际损害时，网站保留依法要求停止侵害、纠正或赔偿合理损失的权利；不会设置不合理的固定罚款。']},
      {id:'change',title:'协议变更、终止与争议',paragraphs:['重要协议变更应以公告和版本日期说明，不追溯扩大原有内容许可。你可停止使用服务；依法应继续保存的必要审计信息仍按隐私政策处理。','对网站处理有异议，可提交复核或通过运营联系渠道协商。协商未解决时，可依法向有管辖权的机构寻求救济。本站规则不限制法定投诉、诉讼或其他救济权利。']}
    ]},
    '/reporting-rules':{title:'举报与申诉规则',intro:'把指控、证据和结论分开，让每一步都能被复核。',sections:[
      {id:'evidence',title:'举报材料要求',paragraphs:['填写可核对的 SteamID64、游戏昵称、发生时间、地图或服务器和具体行为。每份举报只选择一个最主要的分类；其他行为放在详细原因中。','只接受 Bilibili 或 YouTube 的 HTTPS 视频播放页，包括符合验证规则的 YouTube 短视频或分享页。不接受网盘、可执行文件、下载站、其他域名、跳转链接和 B站短链。视频须保留合理上下文，并给出明确时间点。','链接白名单减少不必要风险，但不能证明视频真实或平台页面绝对安全。不要上传剪辑误导的材料，也不要在原因、讨论或申诉中放入密码、身份资料或无关个人信息。']},
      {id:'review',title:'审核步骤与状态',paragraphs:['举报先进入待审核。社区审核员结合完整证据记录独立意见；意见提交不会自动封禁，管理员按权限更新案件结论。','“讨论中”“需更多管理投票”“待自证”“自证材料已提交”均属于待处理过程。网站不会用分类标签代替证据或把待处理记录输出成确认名单。','只有已确认、公开且仍有效的记录进入名单接口。无效、撤销、隐藏、申诉中与到期记录不输出。审核可以改变结论，处理原因应留在时间线中。']},
      {id:'appeal',title:'申诉与证据补充',paragraphs:['案件对应的 Steam 账号登录后，可通过案件页申请复核并说明争议点；管理员也可按权限代为记录申诉。提供相关视频地址和时间点，不需要提交无关身份证明。','申诉正文及处理回复不在公开讨论中展示。公开讨论只用于可公开的案件补充，不能替代私密申诉；其他账号不能冒充当事人提交申诉。','申诉受理和处理回复不自动撤销处罚。管理员应在复核后单独更新案件状态；接入社区应重新获取名单，并处理已撤销、隐藏或到期的记录。']},
      {id:'abuse',title:'滥用、利益冲突与纠错',paragraphs:['禁止凭个人矛盾举报、重复提交未结案件、伪造视频、以辱骂代替说明或组织刷票。网站可合并、退回或隐藏重复及违规材料，并限制持续滥用账号。','审核员涉及自身、所在战队或其他明显利益冲突时，应说明并回避相关结论。发现错认账号、材料不完整、规则适用错误或隐私泄露，应及时纠正或隐藏，并记录处理经过。','公开案件不授予任何人实施人肉搜索、网暴、骚扰或跨社区无限期惩罚的权利。']}
    ]},
    '/community-guidelines':{title:'社区协作规范',intro:'共享可核查的记录，同时保留每个社区的管理责任。',sections:[
      {id:'join',title:'申请与权限',paragraphs:['社区通过管理者的 Steam 账号提交名称、介绍及联系信息。网站有权核实管理关系、拒绝不完整或明显虚假的申请，并在管理主体变更时重新核验。','批准加入并不自动获得全部管理权限。管理员可按实际职责分配、调整或撤销审核权限；社区审核员只处理权限范围内的案件，不得擅自跨社区修改或导出私密资料。']},
      {id:'standards',title:'审核与记录责任',paragraphs:['社区应按证据说明判断依据，不因排名、技巧高低或个人争执直接确认作弊。提出自证要求应说明必要性，不要求安装来源不明程序或提供密码、完整设备控制权。','独立意见、结论、到期时间与撤销均应保持可追溯。审核建议不等于自动封禁；社区不应向网站承诺未经核实的事实。','涉及利益冲突应说明并安排其他人员复核。错误记录和泄露隐私应优先处理，不能为维护社区面子延迟纠正。']},
      {id:'confidentiality',title:'保密与接入责任',paragraphs:['申请联系方式、非公开案件和申诉材料只能用于受理、核查、复核或必要联系。不得对外转发、出售、建立无关画像或公开举报人的私密信息。','社区接入名单后应关注撤销、申诉、隐藏及到期变化，依本社区规则决定措施，并提供有效申诉渠道。网站不代替接入方履行其隐私、安全和管理责任。']},
      {id:'withdraw',title:'整改、暂停与退出',paragraphs:['滥用审核权限、伪造材料、泄露隐私或长期不处理纠错时，网站可要求整改、暂停接入或撤销社区资格。紧急情形可先停用权限；保留必要记录并接受复核。','社区可以申请退出。退出不自动删除历史案件，也不能免除必要的纠错义务；历史资料按隐私政策和必要保存范围处理。']}
    ]},
    '/api-terms':{title:'名单接入条款',intro:'接口用于社区管理参考，接入方应处理更新、撤销和申诉。',sections:[
      {id:'purpose',title:'允许用途',paragraphs:['允许在有管理权限的游戏服务器中查询、缓存有效名单，用于人工核查和依据社区规则进行管理。禁止出售名单、批量人肉检索、骚扰玩家、歧视性画像或将数据用于不相关的广告、征信等用途。','接口可被限流、调整或暂停；网站不保证永久可用或完全无误。密钥不应放在公开前端、公开仓库、截图或请求查询参数中。']},
      {id:'refresh',title:'下载、缓存与纠错',paragraphs:['建议每日下载一次，缓存有效期为 24 小时。下载新数据后应完整替换缓存，不无限累加旧记录；申诉中、撤销、隐藏或到期的记录应停止继续作为有效处罚依据。','请求失败、返回 401、429、异常结构或旧时间戳时，不应把失败响应当作空名单，也不应将过期缓存视为新的确认结论。保留故障状态并转人工检查。','来源应使用 HTTPS 并验证证书。本地预览地址仅用于开发，不应作为线上接入地址；网站当前仍保留在本地。']},
      {id:'responsibility',title:'接入方责任与终止',paragraphs:['接入方独立决定服务器规则和处理结果，应让玩家知道数据来源并提供申诉渠道。不得把本站名单描述成游戏官方封禁、VAC 封禁或法律结论。','缓存、日志和二次展示应控制权限、限定必要范围并及时更新。出现泄露、错误使用或滥用时，网站可撤销接口访问、要求停止使用并依法处理；无法免除任何一方依法应承担的责任。']}
    ]},
    '/cookies':{title:'Cookie 与本地存储',intro:'只使用登录、安全校验和界面偏好所需的存储。',sections:[
      {id:'inventory',title:'存储清单',table:{head:['名称','作用','有效期'],rows:[['alliance_session · Cookie','维持本站登录；HttpOnly，脚本无法直接读取。','最长 8 小时；退出、撤销或服务重启后失效。'],['steam_login_state · Cookie','关联 Steam 登录发起与回调，防止登录请求被替换。','最长 10 分钟，回调时清除。'],['alliance-theme · localStorage','记录你主动选择的明暗主题，仅保存在当前浏览器。','无自动到期；由浏览器清除网站数据时删除。']] }},
      {id:'choices',title:'如何管理',paragraphs:['你可以在个人中心撤销其他登录会话，或退出当前账号。浏览器设置中可清除本站 Cookie 和网站数据；清除后需要重新登录，主题回到默认设置。','禁止必要 Cookie 后仍可浏览公开页面，但 Steam 登录和需登录的提交功能可能无法使用。当前没有使用广告、追踪或第三方分析 Cookie，因此没有用弹窗要求接受这类追踪。','Steam、Bilibili 与 YouTube 的网页有各自的存储政策。本站不加载其嵌入播放器；只有点击相应链接后才进入第三方网页。']}
    ]},
    '/design':{title:'设计材料',intro:'社区联合黑名单的名称、图标、色彩与使用说明。',design:true,sections:[
      {id:'identity',title:'名称与图形',paragraphs:['品牌名称统一为“社区联合黑名单”。图标以两个连接环和中间的名单条目组成：连接环表示社区协作，条目表示证据与记录。','图标和字标为本站自行编写的 SVG 矢量文件，可以无损缩放。未使用 BFBAN 的标志、图片或品牌名；本页不声称商标已注册。']},
      {id:'usage',title:'展示规范',paragraphs:['图标建议至少显示 24 像素，四周留出约图标宽度 1/8 的空间。请保持纵横比例，不拉伸、旋转或擅自添加官方认证符号。','字标适用于页眉、介绍资料和社区接入说明。不得使用本站标志暗示游戏官方认证、保证某项举报成立，或冒充本站发布处罚决定。品牌文件的授权用途由实际运营方确认。']},
      {id:'credits',title:'参考与第三方说明',paragraphs:['首页卡片循环的结构与节奏参考 BFBAN 的开源首页组件；本站重新编写 SVG 连接动画、页面结构和视觉样式。账号查询页参考 Community Ban List 的身份、来源社区、有效与历史记录组织方式，使用本站公开数据独立实现。相关网站内容与本站审核结论没有关联。'],links:[['BFBAN 开源项目','https://github.com/BFBAN/bfban-website'],['参考首页组件','https://github.com/BFBAN/bfban-website/blob/3d1b43f7c3c38a3b1ed409204346770ecb036ec0/front/src/views/Home.vue'],['Community Ban List 查询参考','https://communitybanlist.com/search/76561199100014505'],['Community Ban List 数据与评分说明','https://communitybanlist.com/faq']]}
    ]}
  };
  const base=document.querySelector('meta[name=site-base]')?.content||'';
  function el(tag,copy,cls){const node=document.createElement(tag);if(copy)node.textContent=copy;if(cls)node.className=cls;return node}
  function anchor(copy,path,cls){const a=el('a',copy,cls);a.href=path.startsWith('/')?base+path:path;if(path.startsWith('https:')){a.target='_blank';a.rel='noopener noreferrer'}return a}
  function render(path){
    const doc=routes[path],root=document.getElementById('document-view');root.replaceChildren();
    const breadcrumb=el('div','','breadcrumb');breadcrumb.append(anchor('首页','/'),el('span',' / '+doc.title));root.append(breadcrumb);
    const heading=el('div','','document-heading');heading.append(el('h1',doc.title),el('p',doc.intro),el('span','版本日期 '+version,'document-version'));root.append(heading);
    const layout=el('div','','document-layout'),side=el('aside','','document-sidebar'),nav=el('nav');nav.setAttribute('aria-label',doc.title+'目录');nav.append(el('b','本页内容'));for(const section of doc.sections)nav.append(anchor(section.title,'#'+section.id));side.append(nav,anchor('查看全部网站协议','#footer-policies','document-back'));layout.append(side);
    const article=el('article','','document-content');
    if(!doc.design){const notice=el('div','','document-notice');notice.append(el('b','本地预览 · 运营信息待补充'),el('p','当前运营方与联系渠道使用占位内容。上线前需补齐真实信息，并根据实际运营方式确认这些条款。'));article.append(notice)}
    if(doc.design){const previews=el('div','','brand-resources');for(const [file,title]of [['brand-mark.svg','图标 SVG'],['brand-wordmark.svg','横向字标 SVG']]){const card=el('figure'),img=el('img');img.src=base+'/'+file;img.alt=title;const caption=el('figcaption');caption.append(el('b',title));const download=anchor('下载源文件','/'+file,'button');download.download='community-blacklist-'+file;caption.append(download);card.append(img,caption);previews.append(card)}article.append(previews);const colors=el('div','','brand-colors');for(const [name,hex,cls]of [['档案蓝','#203d68','brand-navy'],['连接金','#f4c74f','brand-gold'],['页面灰','#f7f8fb','brand-paper']]){const item=el('div'),swatch=el('span','',cls);swatch.setAttribute('aria-hidden','true');item.append(swatch,el('b',name),el('code',hex));colors.append(item)}article.append(colors)}
    for(const section of doc.sections){const block=el('section','','document-section');block.id=section.id;block.append(el('h2',section.title));for(const p of section.paragraphs||[])block.append(el('p',p));if(section.table){const wrap=el('div','','document-table-wrap'),table=el('table'),thead=el('thead'),head=el('tr');for(const copy of section.table.head){const th=el('th',copy);th.scope='col';head.append(th)}thead.append(head);table.append(thead);const body=el('tbody');for(const row of section.table.rows){const tr=el('tr');for(const copy of row)tr.append(el('td',copy));body.append(tr)}table.append(body);wrap.append(table);block.append(wrap)}if(section.links){const links=el('div','','document-source-links');for(const [copy,href]of section.links)links.append(anchor(copy,href));block.append(links)}article.append(block)}
    if(!doc.design){const contact=el('section','','document-section document-contact');contact.id='operator-contact';contact.append(el('h2','运营与联系'),el('p','运营方：［待补充］'),el('p','隐私、投诉与纠错联系：［待补充］'),el('p','占位内容不构成真实联系方式。个人案件复核可通过案件页的申诉入口提交；隐私或删除请求请私下联系本地网站维护者。'));article.append(contact)}
    layout.append(article);root.append(layout);document.title=doc.title+' · 社区联合黑名单';
  }
  return {routes,render};
})();
