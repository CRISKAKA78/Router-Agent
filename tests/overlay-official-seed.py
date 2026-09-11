"""Create an isolated EasyTier test account fixture, never a production database.
Only Python's standard library is required. Password is the public fixture value admin.
"""
import sqlite3
import sys
from pathlib import Path

path = Path(sys.argv[1])
if path.exists():
    raise SystemExit("Refusing to overwrite an existing database")
path.parent.mkdir(parents=True, exist_ok=True)
with sqlite3.connect(path) as db:
    db.execute("create table users (id integer not null primary key autoincrement, username varchar not null unique, password varchar not null)")
    db.execute("insert into users(username,password) values (?,?)", ("overlay-test", '$argon2id$v=19$m=19456,t=2,p=1$AF+NtTbTwyUs1+lAn8DSpg$Pnjxs01NNziay4oJBXFC5dZyscALq2hrcUeqAEUad5M'))
    db.execute("create table users_groups (user_id integer not null, group_id integer not null, primary key(user_id, group_id), foreign key(user_id) references users(id) on delete cascade on update cascade, foreign key(group_id) references groups(id) on delete cascade on update cascade)")
    db.executemany("insert into users_groups values(1,?)", [(1,), (2,)])
