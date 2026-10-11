from pathlib import Path
import re
root=Path(__file__).resolve().parents[2]
source=(root/'docs/development/schema.sql').read_text()
actual='\n'.join(p.read_text() for p in sorted((root/'backend/migrations').glob('*.sql')))
def statements(s):
 return sorted(x.strip() for x in s[s.index('CREATE TABLE'):].split(';') if x.strip() and not x.strip().startswith('SET '))
assert statements(source)==statements(actual),'DDL differs from authoritative schema'
assert len(re.findall(r'CREATE TABLE',actual))==37
print('37 tables and all constraints match schema.sql')
