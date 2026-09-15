import sqlite3
c = sqlite3.connect('canyon-springs.db')
for (name,) in c.execute("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name"):
    print('---- ' + name + ' ----')
    for col in c.execute('PRAGMA table_info(%s)' % name):
        print('  %-20s %-10s %s' % (col[1], col[2], 'PK' if col[5] else ''))