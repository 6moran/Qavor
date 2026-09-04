# -*- coding: utf-8 -*-
"""静态验证 Docker 配置：不依赖 Docker 引擎的干跑检查"""
import yaml, os, sys

os.chdir(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
ok = True

# 1. compose YAML 语法
with open('docker-compose.yml', encoding='utf-8') as f:
    compose = yaml.safe_load(f)
print('[1] docker-compose.yml 语法: OK,', len(compose.get('services', {})), '个服务:',
      ', '.join(compose['services'].keys()))

# 2. build context / dockerfile 存在性
for name, svc in compose['services'].items():
    if 'build' in svc:
        ctx = svc['build'].get('context', '.')
        df = svc['build'].get('dockerfile', 'Dockerfile')
        path = os.path.join(ctx, df) if ctx != '.' else df
        exists = os.path.exists(path)
        print('[2] %-12s context=%-12s dockerfile=%s %s' % (name, ctx, path, 'OK' if exists else 'MISSING!'))
        if not exists:
            ok = False

# 3. 挂载源文件存在性（跳过命名卷——顶层 volumes 声明的）
named_volumes = set(compose.get('volumes', {}) or {})
vol_ok = True
for name, svc in compose['services'].items():
    for vol in svc.get('volumes', []):
        src = vol.split(':')[0].replace('./', '')
        if src in named_volumes or (src and src.startswith('$')):
            continue
        if src and not os.path.exists(src):
            print('[3] %s: 挂载源不存在 -> %s' % (name, src))
            vol_ok = False
            ok = False
if vol_ok:
    print('[3] 挂载文件检查: OK（命名卷 %s 由 Docker 自动创建）' % sorted(named_volumes))

# 4. 环境变量交叉检查（只检查 ${VAR} 引用是否在 .env.example 有文档；
#    ${VAR:-default} 有兜底值，缺失也不阻塞启动）
import re
with open('docker-compose.yml', encoding='utf-8') as f:
    compose_text = f.read()
refs = set(re.findall(r'\$\{(\w+)', compose_text))
with open('.env.example', encoding='utf-8') as f:
    env_provided = {l.split('=')[0] for l in f if '=' in l and not l.startswith('#')}
missing = refs - env_provided
print('[4] compose 通过 ${} 引用的变量:', sorted(refs))
print('[4] .env.example 未文档化:', sorted(missing) if missing else '无, OK')
if missing:
    ok = False

# 5/6. CI/CD workflow 语法
for wf in ('.github/workflows/cd.yml', '.github/workflows/ci.yml'):
    with open(wf, encoding='utf-8') as f:
        data = yaml.safe_load(f)
    print('[5] %s 语法: OK, jobs: %s' % (wf, ', '.join(data.get('jobs', {}).keys())))

# 7. nginx.conf 基础检查：括号配平 + 关键指令存在
with open('frontend/nginx.conf', encoding='utf-8') as f:
    nginx = f.read()
brackets = nginx.count('{') - nginx.count('}')
checks = {
    'SPA try_files': 'try_files $uri $uri/ /index.html;' in nginx,
    '/api 反代': 'proxy_pass http://qavor-api:8080;' in nginx,
    'SSE proxy_buffering off': 'proxy_buffering off;' in nginx,
    'SSE HTTP/1.1': 'proxy_http_version 1.1;' in nginx,
    'index.html 禁缓存': 'no-cache' in nginx,
    'gzip': 'gzip on;' in nginx,
}
print('[6] nginx.conf 括号配平:', 'OK' if brackets == 0 else '不平衡(%d)!' % brackets)
for k, v in checks.items():
    print('    %-22s %s' % (k, 'OK' if v else 'MISSING!'))
    if not v or brackets != 0:
        ok = False

# 8. Dockerfile 引用的本地路径存在
for df, base in (('Dockerfile', '.'), ('frontend/Dockerfile', 'frontend')):
    with open(df, encoding='utf-8') as f:
        content = f.read()
    import re
    copies = re.findall(r'COPY\s+(\S+)', content)
    for c in copies:
        if c.startswith('--from'):
            continue
        for part in c.split():
            p = os.path.join(base, part)
            if '*' not in part and not os.path.exists(p):
                print('[7] %s 引用不存在: %s' % (df, p))
                ok = False
print('[7] Dockerfile COPY 引用检查: 完成')

print('\n===== 结论:', '全部通过' if ok else '存在问题(见上)', '=====')
sys.exit(0 if ok else 1)
