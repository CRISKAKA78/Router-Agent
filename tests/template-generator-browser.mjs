// Playwright is isolated in tests/package.json for browser verification only.
// The generator's build and runtime have no Node dependency.
import {chromium, expect} from '@playwright/test';
import {spawn} from 'node:child_process';
import {mkdtemp, mkdir, readFile, writeFile} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {resolve} from 'node:path';
const root = resolve(import.meta.dirname, '..');
const output = resolve(root, process.env.RMP_GENERATOR_OUTPUT || 'build/template-generator-blazor');
const url = process.env.GENERATOR_URL || 'http://127.0.0.1:5188';
const apiOrigin = 'http://127.0.0.1:18680';
await mkdir(output, {recursive:true});
const data = await mkdtemp(resolve(tmpdir(), 'generator-blazor-api-'));
const server = spawn(process.env.RMP_SERVER_BIN || resolve(root, 'build/router-config/router-server.exe'),
  ['-listen','127.0.0.1:18681','-http-listen','127.0.0.1:18680','-repository-dir',data,'-tunnel-data-listen','127.0.0.1:18682','-tunnel-port-first','38300','-tunnel-port-last','38320'],
  {cwd:root, windowsHide:true, stdio:'pipe'});
let browser;
const errors = [], checks = [];
const check = name => { checks.push(name); console.log('PASS ' + name); };
async function api(path, method='GET', body) {
  const response = await fetch(apiOrigin + '/api/v1/' + path, {method, headers:{'Content-Type':'application/json','Idempotency-Key':crypto.randomUUID()}, ...(body ? {body:JSON.stringify(body)} : {})});
  const result = await response.json();
  if (!response.ok) throw Error(JSON.stringify(result));
  return result.data;
}
try {
  for (let attempt=0; attempt<100; attempt++) {
    try { await api('probe-templates'); break; } catch { if (attempt===99) throw Error('Go API did not start'); }
    await new Promise(resolve => setTimeout(resolve,100));
  }
  browser = await chromium.launch({headless:true,channel:process.env.RMP_BROWSER_CHANNEL || 'msedge'});
  const page = await browser.newPage({viewport:{width:1920,height:1080}});
  page.on('pageerror', error => errors.push(String(error)));
  page.on('console', message => { if (message.type()==='error') errors.push(message.text()); });
  page.setDefaultTimeout(12000);
  const tab = name => page.getByRole('tab',{name:new RegExp(name)}).click();
  const select = key => page.locator('.attribute-item').filter({has:page.locator('code',{hasText:new RegExp('^'+key+'$')})}).click();
  const confirm = async (name) => {
    const dialog=page.getByRole('dialog');
    await expect(dialog.locator('input,textarea')).toHaveCount(0);
    await dialog.getByRole('button',{name,exact:true}).click();
    await expect(dialog).toBeHidden();
  };
  const open = async json => {
    await page.locator('#project-file').setInputFiles({name:'import.project.json',mimeType:'application/json',buffer:Buffer.from(JSON.stringify(json))});
  };
  async function download(action) {
    const event=page.waitForEvent('download'); await action();
    const file=await event; return JSON.parse(await readFile(await file.path(),'utf8'));
  }
  await page.goto(url);
  await page.getByRole('button',{name:'加载示例',exact:true}).last().click();
  await expect(page.locator('.attribute-item')).toHaveCount(6);
  await select('memory_total');
  await page.getByRole('button',{name:'复制属性',exact:true}).click();
  await expect(page.getByLabel('属性标识',{exact:true})).toHaveValue('memory_total_copy');
  await page.getByLabel('模拟采集值',{exact:true}).fill('123');
  await select('memory_total');
  await expect(page.getByLabel('模拟采集值',{exact:true})).toHaveValue('262144');
  await select('memory_total_copy');
  await page.locator('.editor-heading').getByRole('button',{name:'删除',exact:true}).click();
  await expect(page.getByRole('dialog').getByRole('button',{name:'取消',exact:true})).toBeFocused();
  await page.keyboard.press('Escape');
  await expect(page.getByRole('dialog')).toBeHidden();
  await expect(page.locator('.attribute-item')).toHaveCount(7);
  await page.locator('.editor-heading').getByRole('button',{name:'删除',exact:true}).click();
  await confirm('删除');
  await expect(page.locator('.attribute-item')).toHaveCount(6);
  await page.getByLabel('搜索属性').fill('used');
  await expect(page.locator('.attribute-item')).toHaveCount(1);
  await page.getByLabel('搜索属性').fill('');
  check('attribute selection, independent copy, deletion confirmation and search');

  await select('used_percent');
  await page.getByLabel('属性标识',{exact:true}).fill('memory_total');
  await expect(page.locator('#attribute-key-error')).toContainText('重复');
  await page.getByLabel('属性标识',{exact:true}).fill('used_percent');
  await page.getByLabel('来源内容',{exact:true}).fill('memory_total / 0');
  await tab('预览与校验');
  await expect(page.locator('tbody tr').filter({hasText:'used_percent'})).toContainText('除数');
  await tab('属性与公式');
  await page.getByLabel('来源内容',{exact:true}).fill('(memory_total - memory_free) / memory_total * 100');
  await tab('预览与校验');
  await expect(page.locator('tbody tr').filter({hasText:'used_percent'})).toContainText('75');
  check('immediate field conflicts, formula failure and successful preview');

  const project = await download(() => page.keyboard.press('Control+s'));
  if (project.schema_version!==2 || project.attributes.length!==6) throw Error('Project download incomplete');
  await expect(page.locator('.document-state')).toContainText('已保存');
  await page.reload();
  await expect(page.locator('.attribute-item')).toHaveCount(6);
  await expect(page.getByLabel('模板名称',{exact:true})).toHaveValue(project.name);
  check('Ctrl+S project download and browser draft reload');

  await page.getByLabel('更多操作').click();
  await page.getByRole('button',{name:'条件示例',exact:true}).click();
  await confirm('替换');
  await select('wan_type');
  await expect(page.locator('.rule-row')).toHaveCount(3);
  await page.getByLabel('规则 2 显示文本',{exact:true}).fill('');
  await expect(page.locator('.rule-row').nth(1).locator('.field-error')).toContainText('显示文本');
  await page.getByLabel('规则 2 显示文本',{exact:true}).fill('5G');
  await page.getByLabel('下移规则 1',{exact:true}).click();
  await expect(page.getByLabel('规则 1 条件',{exact:true})).toHaveValue('wan_proto == 1');
  await page.getByLabel('上移规则 2',{exact:true}).click();
  await page.getByLabel('删除规则 1',{exact:true}).click();
  await page.getByRole('dialog').getByRole('button',{name:'取消',exact:true}).click();
  await expect(page.locator('.rule-row')).toHaveCount(3);
  await tab('预览与校验');
  await expect(page.locator('tbody tr').filter({hasText:'wan_type'})).toContainText('4G');
  await expect(page.locator('tbody tr').filter({hasText:'wan_type'})).toContainText('命中规则 1');
  await tab('属性与公式'); await select('wan_proto');
  await page.getByLabel('模拟采集值',{exact:true}).fill('9');
  await tab('预览与校验');
  await expect(page.locator('tbody tr').filter({hasText:'wan_type'})).toContainText('默认结果');
  check('condition editing, near-field validation, ordering, delete cancel, matched and fallback previews');

  await tab('属性与公式'); await select('wan_type');
  for (const [width,height,theme] of [[1920,1080,'Light'],[2560,1440,'Dark'],[3440,1440,'Light'],[900,760,'Dark']]) {
    await page.setViewportSize({width,height});
    await page.getByLabel('更多操作').click();
    await page.getByLabel('外观',{exact:true}).selectOption(theme);
    await expect(page.locator('html')).toHaveAttribute('data-theme',theme.toLowerCase());
    await page.screenshot({path:resolve(output,`workspace-${width}-${theme}.png`)});
    const geometry=await page.evaluate(() => ({w:innerWidth,h:innerHeight,body:document.documentElement.scrollWidth,
      shell:document.querySelector('.app-shell').getBoundingClientRect().toJSON(),bar:document.querySelector('.status-bar').getBoundingClientRect().toJSON(),
      grid:document.querySelector('.field-grid') && getComputedStyle(document.querySelector('.field-grid')).gridTemplateColumns}));
    if (geometry.body>width || Math.abs(geometry.bar.bottom-height)>1 || geometry.shell.width!==width) throw Error('Workspace overflow/layout: '+JSON.stringify(geometry));
  }
  await page.getByLabel('折叠或展开属性导航').click();
  await expect(page.locator('.workspace')).toHaveClass(/pane-collapsed/);
  await page.getByLabel('折叠或展开属性导航').click();
  await page.setViewportSize({width:1920,height:1080});
  check('1920/2560/3440 and narrow layouts, Light/Dark themes and collapsible navigation');

  const rulesProject = await download(() => page.keyboard.press('Control+s'));
  await tab('导出与发布');
  const runtime = await download(() => page.getByRole('button',{name:'导出运行模板',exact:true}).click());
  if (Object.keys(runtime.properties).length!==2 || !runtime.properties.wan_type.command.includes('awk')) throw Error('Runtime export mismatch');
  await page.getByLabel('管理服务器地址',{exact:true}).fill(apiOrigin);
  await page.getByRole('button',{name:'保存并连接',exact:true}).click();
  await expect(page.getByRole('button',{name:'发布为新模板',exact:true})).toBeEnabled();
  await page.getByRole('button',{name:'发布为新模板',exact:true}).click();
  await expect(page.locator('.template-table tbody tr')).toHaveCount(1);
  let item=(await api('probe-templates')).items[0];
  if (item.properties.wan_type.command!==runtime.properties.wan_type.command) throw Error('Go API command differs from export');
  await page.getByRole('button',{name:'更新已绑定模板',exact:true}).click(); await confirm('更新');
  await expect.poll(async () => (await api('probe-templates')).items[0].version).toBe(2);
  // A concurrent client update must surface a conflict rather than overwrite the newer version.
  item=(await api('probe-templates')).items[0];
  await api('probe-templates/'+item.template_id,'PUT',{name:item.name,properties:item.properties,version:2});
  await page.getByRole('button',{name:'更新已绑定模板',exact:true}).click();
  await page.getByRole('dialog').getByRole('button',{name:'更新',exact:true}).click();
  await expect(page.locator('.workspace-toast')).toContainText('冲突');
  await page.getByRole('dialog').getByRole('button',{name:'取消',exact:true}).click();
  await page.getByRole('button',{name:/刷新/}).click();
  await page.getByRole('button',{name:'绑定 '+item.name,exact:true}).click();
  await page.getByRole('button',{name:'更新已绑定模板',exact:true}).click(); await confirm('更新');
  await expect.poll(async () => (await api('probe-templates')).items[0].version).toBe(4);
  await page.getByRole('button',{name:'删除 '+item.name,exact:true}).click(); await confirm('删除');
  await expect(page.locator('.template-table tbody tr')).toHaveCount(0);
  check('runtime export and real Go API create/update/conflict/rebind/delete');

  // Native draft envelope keeps the source graph and original binding when explicitly imported.
  await open({project:rulesProject,target:{origin:apiOrigin,id:item.template_id,version:4},savedAt:1});
  await confirm('替换');
  await expect(page.locator('.attribute-item')).toHaveCount(4);
  await tab('导出与发布');
  await expect(page.locator('.publishing-target')).toContainText(item.template_id);
  const v1={...project,schema_version:1};
  await open(v1); await confirm('替换');
  await expect(page.locator('.attribute-item')).toHaveCount(6);
  await page.locator('#project-file').setInputFiles({name:'invalid.json',mimeType:'application/json',buffer:Buffer.from('{broken')});
  await expect(page.locator('.workspace-toast')).toContainText('JSON');
  await expect(page.locator('.attribute-item')).toHaveCount(6);
  check('legacy v1 project, v2 native draft and invalid import protection');
  await page.getByRole('button',{name:'新建',exact:false}).first().click(); await confirm('替换');
  await page.getByLabel('模板名称',{exact:true}).fill('手动属性工程');
  await page.locator('.add-menu summary').click();
  await page.locator('.add-menu').getByRole('button',{name:/展示属性/}).click();
  await page.getByLabel('属性名称',{exact:true}).fill('WAN IP地址');
  await page.getByLabel('属性标识',{exact:true}).fill('wanip');
  await page.getByLabel('获取方式',{exact:true}).selectOption('Nvram');
  await page.getByLabel('来源内容',{exact:true}).fill('wan_ipaddr');
  await page.getByLabel('模拟采集值',{exact:true}).fill('192.168.1.1');
  await page.locator('.advanced-section summary').click();
  await page.getByLabel('采集超时',{exact:true}).fill('12');
  await expect(page.locator('.validation-status')).toContainText('校验通过');
  await page.locator('.add-menu summary').click();
  await page.locator('.add-menu').getByRole('button',{name:/虚拟属性/}).click();
  await page.getByLabel('属性名称',{exact:true}).fill('虚拟主机名');
  await page.getByLabel('属性标识',{exact:true}).fill('router_name');
  await page.getByLabel('获取方式',{exact:true}).selectOption('Uci');
  await page.getByLabel('来源内容',{exact:true}).fill('system.@system[0].hostname');
  await page.getByLabel('模拟采集值',{exact:true}).fill('router');
  await tab('预览与校验');
  await expect(page.locator('tbody tr')).toHaveCount(1);
  await expect(page.locator('tbody tr')).toContainText('192.168.1.1');
  await tab('导出与发布');
  const manual=await download(() => page.getByRole('button',{name:'导出运行模板',exact:true}).click());
  if (manual.properties.wanip.timeout_seconds!==12 || manual.properties.wanip.source!=='nvram') throw Error('Manual source/timeout mismatch');
  await tab('属性与公式'); await select('wanip');
  await page.getByLabel('更多操作').click(); await page.getByLabel('外观',{exact:true}).selectOption('Light');
  await expect(page.locator('.document-state')).toContainText('已保存');
  await page.screenshot({path:resolve(output,'editor-final-1920.png')});
  check('new project and manual display/virtual attributes, NVRAM/UCI sources and timeout');
  if (errors.length) throw Error('Browser errors: '+errors.join('\n'));
  await writeFile(resolve(output,'browser-results.json'),JSON.stringify({checks,errors},null,2));
  console.log(`PASS ${checks.length} browser scenario groups; no browser errors`);
} finally {
  await browser?.close();
  server.kill();
}
