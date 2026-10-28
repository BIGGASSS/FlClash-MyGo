"""Regenerate the synthetic Drift v21 migration fixture; no user data."""
import base64, hashlib, io, json, os, sqlite3, tempfile, zipfile
root = os.path.dirname(__file__)
with tempfile.TemporaryDirectory() as temp:
    path = os.path.join(temp, 'database.sqlite')
    db = sqlite3.connect(path)
    db.executescript('''
PRAGMA user_version=21;
CREATE TABLE profiles (id INTEGER NOT NULL, type TEXT NOT NULL, label TEXT NOT NULL, url TEXT NOT NULL, current_group_name TEXT, age_secret_key TEXT, last_update_date INTEGER, extend_type TEXT NOT NULL, script_id INTEGER, match_target TEXT, auto_update_duration_millis INTEGER NOT NULL, subscription_info TEXT, auto_update INTEGER NOT NULL CHECK(auto_update IN (0,1)), selected_map TEXT NOT NULL, unfold_set TEXT NOT NULL, "order" INTEGER, overrides TEXT, PRIMARY KEY(id));
CREATE TABLE scripts (id INTEGER NOT NULL PRIMARY KEY, label TEXT NOT NULL, last_update_time INTEGER, url TEXT, "order" INTEGER);
CREATE TABLE rules (id INTEGER NOT NULL PRIMARY KEY, profile_id INTEGER, rule_action TEXT NOT NULL, content TEXT, rule_target TEXT, rule_provider TEXT, sub_rule TEXT, no_resolve INTEGER NOT NULL DEFAULT 0, src INTEGER NOT NULL DEFAULT 0, "order" TEXT);
CREATE TABLE disabled_rules (profile_id INTEGER NOT NULL, rule_id INTEGER NOT NULL, PRIMARY KEY(profile_id,rule_id));
CREATE TABLE proxy_groups (id INTEGER NOT NULL PRIMARY KEY, profile_id INTEGER NOT NULL, name TEXT NOT NULL, type TEXT NOT NULL, proxies TEXT, use TEXT, definition TEXT NOT NULL DEFAULT '{}', "order" TEXT);
CREATE TABLE custom_proxies (id INTEGER NOT NULL PRIMARY KEY, definition TEXT NOT NULL, "order" TEXT, empty_options TEXT);
CREATE TABLE proxy_dialers (profile_id INTEGER NOT NULL, proxy_id INTEGER NOT NULL, target TEXT NOT NULL, PRIMARY KEY(profile_id,proxy_id));
CREATE TABLE clash_providers (id INTEGER NOT NULL PRIMARY KEY, kind TEXT NOT NULL, label TEXT NOT NULL, url TEXT NOT NULL, behavior TEXT, format TEXT, "order" INTEGER);
''')
    for id, type, label, extend, script, order in [(11,'url','Subscription','standard',None,0),(12,'file','Scripted','script',31,2),(13,'custom','Custom','standard',None,1)]:
        db.execute('INSERT INTO profiles VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)', (id,type,label,'https://example.invalid/sub' if type=='url' else '', 'Select',None,1800000000,extend,script,'DIRECT',3600000,json.dumps({'upload':1024,'download':4096,'total':1048576,'expire':1900000000}),1,json.dumps({'Select':'Node'}),json.dumps(['Select']),order,json.dumps({'dns':{'ipv6':True,'nameserver':['ignored']},'dns-override-keys':['ipv6'],'proxy-providers':{'Subscription':{'filter':'Node'}}})))
    db.execute('INSERT INTO scripts VALUES (31,?,?,?,?)', ('Transform',1800000000,None,0))
    for id, owner, action, content, target, provider, order in [(41,None,'DOMAIN','global.example','DIRECT',None,'a0'),(42,None,'DOMAIN','disabled.example','REJECT',None,'a1'),(43,11,'IP_CIDR','192.0.2.0/24','MATCH',None,'a0'),(44,13,'RULE_SET',None,'Select','Local set','a0'),(45,13,'MATCH',None,'Select',None,'a1')]:
        db.execute('INSERT INTO rules VALUES (?,?,?,?,?,?,NULL,?,?,?)', (id,owner,action,content,target,provider,action=='IP_CIDR',0,order))
    db.execute('INSERT INTO disabled_rules VALUES (11,42)')
    db.execute('INSERT INTO proxy_groups VALUES (51,13,?,?,?,?,?,?)', ('Select','select',json.dumps(['Node','Loop']),json.dumps(['Subscription']),json.dumps({'icon':'https://example.invalid/icon.svg','url':'https://www.gstatic.com/generate_204','interval':300}),'a0'))
    for id, name, target in [(61,'Node','DIRECT'),(62,'Loop','Select')]:
        db.execute('INSERT INTO custom_proxies VALUES (?,?,?,?)', (id,json.dumps({'name':name,'type':'ss','server':'192.0.2.1','port':443,'cipher':'aes-128-gcm','password':'test-fixture'}),'a'+str(id),json.dumps(['sni'])))
        db.execute('INSERT INTO proxy_dialers VALUES (13,?,?)',(id,target))
    db.execute('INSERT INTO clash_providers VALUES (71,?,?,?,?,?,0)', ('rule','Local set','','classical','yaml'))
    db.commit(); db.close()
    nonce = bytes(range(16)); mask=bytes.fromhex('9d42e71b68b435ca7f20d15683fa0ca9'); password='example-password'.encode()
    encoded='v1.'+base64.urlsafe_b64encode(nonce).decode()+'.'+base64.urlsafe_b64encode(bytes(b^nonce[i%16]^mask[i%16] for i,b in enumerate(password))).decode()
    config = {'version':1,'currentProfileId':13,'appSettingProps':{'locale':'ja','sidebarExpanded':False,'minimizeOnExit':False,'acceptedDisclaimerVersion':1,'delayTestTimeout':5000,'filters':[{'label':'Custom','regex':'Node'}]},'themeProps':{'themeMode':'dark','primaryColor':0xff123456,'pureBlack':True,'colorStyle':'tinted','textScale':{'enable':True,'scale':1.1}},'networkProps':{'systemProxy':False,'bypassDomain':['localhost','127.*']},'davProps':{'uri':'https://example.invalid/dav','user':'sample','password':encoded},'patchClashConfig':{'mixed-port':17890,'mode':'rule','tun':{'enable':False},'dns':{'ipv6':False,'nameserver':['https://example.invalid/dns-query']},'dns-override-keys':['ipv6']},'future-setting':{'preserve':True}}
    with zipfile.ZipFile(os.path.join(root,'flutter-v21.zip'),'w',zipfile.ZIP_DEFLATED) as z:
        z.writestr('config.json',json.dumps(config))
        z.write(path,'database.sqlite')
        z.writestr('profiles/11.yaml','# original comment\nproxies: []\nproxy-groups: []\nrules: ["MATCH,DIRECT"]\nunknown-setting: keep\n')
        z.writestr('profiles/12.yaml','proxies: []\nrules: ["MATCH,DIRECT"]\n')
        z.writestr('scripts/31.js','function main(config) { config.extra = "preserved"; return config; }')
        z.writestr('providers/rules/'+hashlib.md5(b'71@').hexdigest(),'payload: ["DOMAIN,local.example"]\n')
