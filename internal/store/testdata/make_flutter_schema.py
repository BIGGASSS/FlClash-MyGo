"""Extract Drift v21 SQL from the pinned generated Dart (no Flutter SDK needed).
Run from repository root: python3 internal/store/testdata/make_flutter_schema.py
Source: FlClash 7fd417fb9a9a277c6a9339a8ac87f907df7c25f7 (GPL-3.0).
"""
import pathlib
import re

root = pathlib.Path(__file__).resolve().parents[3]
source = (root / 'build/upstream/lib/database/generated/database.g.dart').read_text()
statements = ['-- Generated from pinned FlClash Drift declarations; see testdata/make_flutter_schema.py.', 'PRAGMA user_version = 21;']
for match in re.finditer(r'class \$(\w+)Table extends .*?(?=\nclass |\Z)', source, re.S):
    block = match.group()
    name = re.search(r"static const String \$name = '([^']+)'", block)
    if not name:
        continue
    head = block.split('List<GeneratedColumn> get $columns')[0]
    columns, fields = [], {}
    for col in re.finditer(r"(\w+)\s*=\s*GeneratedColumn<[^>]+>\(\s*'([^']+)',\s*aliasedName,\s*(true|false),\s*type: DriftSqlType\.(\w+),(.*?)(?:\n\s*\);|\n\s*\)\.withConverter)", head, re.S):
        field, sqlname, nullable, kind, rest = col.groups()
        fields[field] = sqlname
        sqltype = {'int': 'INTEGER', 'dateTime': 'INTEGER', 'bool': 'INTEGER', 'string': 'TEXT'}[kind]
        declaration = f'"{sqlname}" {sqltype}' + (' NOT NULL' if nullable == 'false' else '')
        constraint = re.search(r"constraintIsAlways\(\s*'([^']+)'", rest)
        if constraint:
            declaration += ' ' + constraint[1]
        default = re.search(r"defaultValue: const Constant\((.*?)\)", rest)
        if default:
            declaration += ' DEFAULT ' + {'false': '0', 'true': '1'}.get(default[1], default[1])
        columns.append(declaration)
    keys = re.search(r'get \$primaryKey => \{([^}]+)\}', block)[1].split(',')
    columns.append('PRIMARY KEY (' + ', '.join('"' + fields[k.strip()] + '"' for k in keys) + ')')
    statements.append(f'CREATE TABLE "{name[1]}" (' + ', '.join(columns) + ');')
for sql in re.findall(r"'(CREATE INDEX [^']+)'", source):
    statements.append(sql + ';')
(root / 'internal/store/flutter_schema.sql').write_text('\n'.join(statements) + '\n')
