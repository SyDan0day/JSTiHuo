"""Generate development secrets once; never overwrite existing configuration."""
from pathlib import Path
import secrets,json,os
p=Path(__file__).parent/'secrets'
p.mkdir(mode=0o700,exist_ok=True)
def write(name,value):
 f=p/name
 with f.open('x') as out: out.write(value)
 f.chmod(0o600)
if list(p.iterdir()): raise SystemExit('Existing secrets preserved; refusing regeneration')
import base64
def key(): return base64.b64encode(secrets.token_bytes(32)).decode()
password=secrets.token_hex(32)
write('db_root',password)
write('migration_dsn',f'root:{password}@tcp(mysql:3306)/jstihuo')
app_password=secrets.token_hex(32)
write('app_dsn',f'jstapp:{app_password}@tcp(mysql:3306)/jstihuo')
write('app-user.sql',f"CREATE USER 'jstapp'@'%' IDENTIFIED BY '{app_password}';\nGRANT SELECT, INSERT, UPDATE, DELETE ON jstihuo.* TO 'jstapp'@'%';\n")
for name in ['lookup','request','challenge']:write(name,key())
for name in ['pii','export']:write(name+'.json',json.dumps({'active':'1','keys':{'1':key()}}))
print('Local secrets generated; no values printed')
