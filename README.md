# schemadiff

Command line tool for declarative MySQL schema validation, normalization and diffing, based on Vitess' `schemadiff` library.

## Overview

The `schemadiff` command line tool is a thin wrapper around the Vitess [schemadiff](https://github.com/vitessio/vitess/tree/main/go/vt/schemadiff) library, which offers declarative schema analysis, validation, normalization, diffing and manipulation. Read more about the `schemadiff` library on the [Vitess blog](https://vitess.io/blog/2023-04-24-schemadiff/).

The command line tool makes schema normalization, validation, and diffing accessible in testing, automation and scripting environments. You may load or compare schemas from SQL files, directories, standard input, or as read from a MySQL server.

`schemadiff` is declarative, which means it does not need a MySQL server to operate. `schemadiff` works by parsing the schema's `CREATE TABLE|VIEW` statements and by applying MySQL compatible analysis and validation to those statements. For convenience, the `schemadiff` command line tool supports reading a schema from a MySQL server. `schemadiff` applies its own normalization of table/view definitions, resulting in consistent and as compact as possible representations of the schema.

The `schemadiff` executable supports these operations:

- `load`: read a table, view or full schema, validate, normalize, and output the normalized form.
- `diff`: given two schemas, _source_ and _target_, output the DDL (`CREATE`, `ALTER`, `DROP`) statements that when applied to the _source_ schema, result in _target_ schema. The output is empty when the two schemas are identical.
- `ordered-diff`: similar to `diff` but stricter, output the DDL in a sequential-applicable order, or fail if such order cannot be found. This operation resolves dependencies between the diffs themselves, such as changes made to both tables and views that depend on those tables, or tables involved in a foreign key relationships.
- `diff-table`: given two table definitions, _source_ and _target_, output the `ALTER TABLE` statement that would convert the _source_ table into _target_. The two tables may have different names. The output is empty when the two tables are identical.
- `diff-view`: given two view definitions, _source_ and _target_, output the `ALTER VIEW` statement that would convert the _source_ view into _target_. The two views may have different names. The output is empty when the two tables are identical.

`schemadiff` diffs according to a pre-defined set of _hints_. For example, `schemadiff` will completely ignore `AUTO_INCREMENT` values of compared tables. At this time these hints are not configurable.

`schemadiff` supports:

- MySQL `8.0` dialect.
- `TABLE` and `VIEW` definitions. Stored routines (procedures/functions/triggers/events) are unsupported.
- Nested views, view table and column validation.
- Foreign keys, nested foreign keys. Cyclic foreign key only supported on same-table cycle.
- Partitions; diffs mostly rebuild partitioning schemes and not optimal.

## Usage and examples

### load

- Read from standard output, validate and normalize:

```sh
$ echo "create table t (id int(11) unsigned primary key)" | schemadiff load
```
```sql
CREATE TABLE `t` (
	`id` int unsigned,
	PRIMARY KEY (`id`)
);
```

- Read an invalid definition:

```sh
$ echo "create table t (id int unsigned primary key, key name_idx (name))" | schemadiff load
invalid column `name` referenced by key `name_idx` in table `t`
```

- Read a definition with invalid view dependencies:

```sh
$ echo "create table t (id int primary key); create view v as select id from some_table" | schemadiff load
view `v` has unresolved/loop dependencies
```

- Read schema from file:

```sh
$ echo "create table t (id int primary key); create view v as select id from t" > /tmp/schema.sql
$ schemadiff load --source /tmp/schema.sql
```
```sql
CREATE TABLE `t` (
	`id` int,
	PRIMARY KEY (`id`)
);
CREATE VIEW `v` AS SELECT `id` FROM `t`;
```

- Read schema from directory. `schemadiff` reads all `.sql` files in given path. Each file is expected to contain a single statement:

```sh
$ schema_dir=$(mktemp -d)
$ echo "create table t (id int primary key)" > $schema_dir/t.sql
$ echo "create table t2 (id int primary key, name varchar(128) not null default '')" > $schema_dir/t2.sql
$ schemadiff load --source $schema_dir
```
```sql
CREATE TABLE `t` (
	`id` int,
	PRIMARY KEY (`id`)
);
CREATE TABLE `t2` (
	`id` int,
	`name` varchar(128) NOT NULL DEFAULT '',
	PRIMARY KEY (`id`)
);
```

- Read a full schema from a running MySQL server. `schemadiff` reads the `SHOW CREATE TABLE` statements for all tables and views in the given schema. Provide a valid DSN in [`go-sql-driver` format](https://github.com/go-sql-driver/mysql#dsn-data-source-name):

```sh
$ schemadiff load --source 'myuser:mypass@tcp(127.0.0.1:3306)/test'
```
```sql
CREATE TABLE `t` (
	`id` int,
	PRIMARY KEY (`id`)
);
CREATE TABLE `t2` (
	`id` int,
	`name` varchar(128) NOT NULL DEFAULT '',
	PRIMARY KEY (`id`)
);
```

- Read a specific table from a running MySQL server. Syntax is a valid DSN with table indicated as comment:

```sh
$ schemadiff load --source 'myuser:mypass@tcp(127.0.0.1:3306)/test?#t2'
```
```sql
CREATE TABLE `t2` (
	`id` int,
	`name` varchar(128) NOT NULL DEFAULT '',
	PRIMARY KEY (`id`)
);
```


### diff

- Diff two schemas:

```sh
$ echo "create table t (id int primary key); create view v as select id from t" > /tmp/schema_v1.sql
$ echo "create table t (id bigint primary key); create table t2 (id int primary key, name varchar(128) not null default '')" > /tmp/schema_v2.sql
$ schemadiff diff --source /tmp/schema_v1.sql --target /tmp/schema_v2.sql
```
```sql
DROP VIEW `v`;
ALTER TABLE `t` MODIFY COLUMN `id` bigint;
CREATE TABLE `t2` (
	`id` int,
	`name` varchar(128) NOT NULL DEFAULT '',
	PRIMARY KEY (`id`)
);
```

- Reverse the above diff. Show what it takes to convert `schema_v2` to `schema_v1`:
```sh
$ schemadiff diff --source /tmp/schema_v2.sql --target /tmp/schema_v1.sql
```
```sql
DROP TABLE `t2`;
ALTER TABLE `t` MODIFY COLUMN `id` int;
CREATE VIEW `v` AS SELECT `id` FROM `t`;
```

- Compare a running MySQL server's schema with schema found in a directory's `.sql` files (each file expected to contain a single `CREATE` statement):

```sh
$ schemadiff diff --source 'myuser:mypass@tcp(127.0.0.1:3306)/test' --target /path/to/schema
```
```sql
DROP VIEW `v`;
ALTER TABLE `t` MODIFY COLUMN `id` bigint;
CREATE TABLE `t2` (
	`id` int,
	`name` varchar(128) NOT NULL DEFAULT '',
	PRIMARY KEY (`id`)
);
```

- Generate a valid schema destruction sequence:

```sh
$ echo "create table t (id int primary key); create view v as select id from t" > /tmp/schema.sql
$ echo "" | schemadiff diff --source /tmp/schema.sql
```
```sql
DROP VIEW `v`;
DROP TABLE `t`;
```

- Same as above, alternative method:

```sh
$ echo > /tmp/empty_schema.sql
$ echo "create table t (id int primary key); create view v as select id from t" > /tmp/schema.sql
$ schemadiff diff --source /tmp/schema.sql --target /tmp/empty_schema.sql
```
```sql
DROP VIEW `v`;
DROP TABLE `t`;
```

### ordered-diff

- Generate a diff that has a strict ordering dependency:

```sh
$ echo "create table parent (id int primary key, uuid varchar(32) charset ascii); create table child (id int primary key, parent_uuid varchar(32) charset ascii)" > /tmp/schema_v1.sql
$ echo "create table parent (id int primary key, uuid varchar(32) charset ascii, unique key uuid_idx (uuid)); create table child (id int primary key, parent_uuid varchar(32) charset ascii, foreign key (parent_uuid) references parent (uuid))" > /tmp/schema_v2.sql
$ schemadiff ordered-diff --source /tmp/schema_v1.sql --target /tmp/schema_v2.sql
```
```sql
ALTER TABLE `parent` ADD UNIQUE KEY `uuid_idx` (`uuid`);
ALTER TABLE `child` ADD KEY `parent_uuid` (`parent_uuid`), ADD CONSTRAINT `child_ibfk_1` FOREIGN KEY (`parent_uuid`) REFERENCES `parent` (`uuid`);
```

Note that in the above the change on `child` cannot take place before the change on `parent`, because a MySQL foreign key requires an index on the referenced table column(s). `ordered-diff` returns an error if there is no serial sequence of steps which maintains validity at each step.

### diff-table

- Compare two tables. Note they may have different names; `schemadiff` will use the _source_ table name:

```sh
$ echo "create table t1 (id int primary key)" > /tmp/t1.sql
$ echo "create table t2 (id bigint unsigned primary key, ranking int not null default 0)" > /tmp/t2.sql
$ schemadiff diff-table --source /tmp/t1.sql --target /tmp/t2.sql
```
```sql
ALTER TABLE `t1` MODIFY COLUMN `id` bigint unsigned, ADD COLUMN `ranking` int NOT NULL DEFAULT 0;
```

Consider that running `schemadiff diff` on the same tables above yields with `DROP TABLE` for `t1` and `CREATE TABLE` for `t2`.

- Compare two tables, one from standard input, the other from the database:

```sh
$ echo "create table t1 (id int primary key)" | schemadiff diff-table --target 'myuser:mypass@tcp(127.0.0.1:3306)/test?#t2'
```
```sql
ALTER TABLE `t1` MODIFY COLUMN `id` bigint unsigned NOT NULL, ADD COLUMN `ranking` int NOT NULL DEFAULT '0', ENGINE InnoDB CHARSET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
```

### diff-view

- Compare two views. Note they may have different names; `schemadiff` will use the _source_ view name:

```sh
$ echo "create view v1 as select id from t1" > /tmp/v1.sql
$ echo "create view v2 as select id, name from t1" > /tmp/v2.sql
$ schemadiff diff-view --source /tmp/v1.sql --target /tmp/v2.sql
```
```sql
ALTER VIEW `v1` AS SELECT `id`, `name` FROM `t1`;
```

Consider that running `schemadiff diff` on the same views above results with validation error, because the referenced table `t1` does not appear in the schema definition. `diff-view` does not attempt to resolve dependencies.


## Binaries

Binaries for linux/amd64 and for darwin/arm64 are available in [Releases](https://github.com/planetscale/schemadiff/releases).

The `CI` action builds a Linux/amd64 `schemadiff` binary as artifact. See [Actions](https://github.com/planetscale/schemadiff/actions)

## Build

To build 	`schemadiff`, run:

```sh
$ make all
```

Or, directly invoke:

```sh
$ go build -trimpath -o bin/schemadiff ./cmd/schemadiff/main.go
```

`schemadiff` was built with `go1.20`.

## License

`schemadiff` command line tool is released under [Apache 2.0 license](LICENSE)
