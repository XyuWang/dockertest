package main

import (
	"context"
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	log "github.com/sirupsen/logrus"
)

func sqlPath() (res string) {
	dir, _ := os.Getwd()
	for filepath.Dir(dir) != "/" {
		if _, err := os.Stat(filepath.Join(dir, "resource")); err == nil {
			return filepath.Join(dir, "resource")
		}
		if _, err := os.Stat(filepath.Join(dir, "test")); err == nil {
			return filepath.Join(dir, "test")
		}
		dir = filepath.Dir(dir)
	}
	return
}

func cleanMysql(dsn string) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Errorf("open db err: %v", err)
		return
	}
	c := context.Background()
	defer db.Close()
	rows, err := db.QueryContext(c, "show databases")
	if err != nil {
		log.Errorf("show databases err: %v", err)
		return
	}
	var dbs []string
	for rows.Next() {
		var name string
		rows.Scan(&name)
		dbs = append(dbs, name)
	}
	dbs = businessDbs(dbs)
	for _, name := range dbs {
		_, err = db.ExecContext(c, fmt.Sprintf("drop database %s", name))
		if err != nil {
			log.Errorf("drop databases err: %v", err)
			return
		}
	}
}

func initMysql(dsn string) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Errorf("open db err: %v", err)
		return
	}
	c := context.Background()
	defer db.Close()
	pathDir := sqlPath()
	files, err := ioutil.ReadDir(pathDir)
	if err != nil {
		log.Errorf("read path: %s err: %s", pathDir, err)
		return
	}
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		if !strings.HasSuffix(f.Name(), ".sql") {
			continue
		}
		content, err := ioutil.ReadFile(filepath.Join(pathDir, f.Name()))
		if err != nil {
			log.Errorf("read %s err: %s", f.Name(), err)
			return
		}
		_, err = db.ExecContext(c, string(content))
		if err != nil {
			log.Errorf("exec %s err: %s", f.Name(), err)
			return
		}
	}
}

func businessDbs(dbs []string) (res []string) {
	for _, db := range dbs {
		if db == "information_schema" || db == "mysql" || db == "performance_schema" {
			continue
		}
		res = append(res, db)
	}
	return
}
