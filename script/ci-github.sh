#!/bin/bash

#
# Intended to run as a GitHub CI workflow, and assuming `schemdiff` build is available, this
# test validates connectivity to a MySQL server, and that schemadiff is able to load and diff
# tables from a MySQL schema.

set -x
set -e
shopt -s expand_aliases

output_file="${TMPDIR:-/tmp}/schemadiff-ci-output.sql"
create_t1="create table t1 (id int primary key)"
create_t2="create table t2 (id bigint primary key)"

echo "${create_t1}" | schemadiff load > $output_file

grep -q 'CREATE TABLE' $output_file

alias my='mysql -uroot -proot --host 127.0.0.1 --port 33306'
my -e "create database if not exists test"
echo "${create_t1}" | my test

# diff two schemas, one with `t1`, another with `t2`:
echo "${create_t2}" | schemadiff diff --source 'root:root@tcp(127.0.0.1:33306)/test' > $output_file
cat $output_file
grep -q 'DROP TABLE `t1`' $output_file
grep -q 'CREATE TABLE `t2`' $output_file

# diff the tables `t1` vs `t2`
echo "${create_t2}" | schemadiff diff-table --source 'root:root@tcp(127.0.0.1:33306)/test?#t1' > $output_file
cat $output_file
grep -q 'ALTER TABLE `t1`' $output_file
grep -q -v 'CREATE TABLE`' $output_file
grep -q -v 'DROP TABLE`' $output_file

rm $output_file

echo "PASS"
