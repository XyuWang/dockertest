package dockertest

import (
	"context"
	"database/sql"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"
)

func mysqlHook(c *Container) (err error) {
	if c.Fresh {
		return
	}
	dsn := getDSN(c)
	if err = cleanMysql(dsn); err != nil {
		return
	}
	return initMysql(c, dsn)
}

func getDSN(c *Container) (dsn string) {
	var (
		user = "root"
		host = "127.0.0.1"
		pw   string
		port = "3306"
	)
	envs := make(map[string]string)
	for _, env := range c.Env {
		a := strings.Split(env, "=")
		if len(a) == 2 {
			envs[a[0]] = a[1]
		}
	}
	if len(c.ImageCfg.Ports) > 0 {
		for _, k := range c.ImageCfg.Ports {
			a := strings.Split(k, ":")
			if len(a) != 2 {
				continue
			}
			if a[1] == "3306" {
				port = a[0]
			}
		}
	}
	if envs["MYSQL_ROOT_PASSWORD"] != "" {
		pw = envs["MYSQL_ROOT_PASSWORD"]
	}
	if envs["MYSQL_USER"] != "" {
		user = envs["MYSQL_USER"]
	}
	if envs["MYSQL_PASSWORD"] != "" {
		pw = envs["MYSQL_PASSWORD"]
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/?multiStatements=true", user, pw, host, port)
}

func sqlPath(ct *Container) (res string) {
	for _, m := range ct.Mounts {
		if m.Target == "/docker-entrypoint-initdb.d" {
			return m.Source
		}
	}
	return
}

func cleanMysql(dsn string) (err error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		err = errors.WithStack(err)
		return
	}
	c := context.Background()
	defer db.Close()
	rows, err := db.QueryContext(c, "show databases")
	if err != nil {
		err = errors.WithStack(err)
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
			err = errors.WithStack(err)
			return
		}
	}
	return
}

func initMysql(ct *Container, dsn string) (err error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		err = errors.WithStack(err)
		return
	}
	c := context.Background()
	defer db.Close()
	pathDir := sqlPath(ct)
	files, err := ioutil.ReadDir(pathDir)
	if err != nil {
		err = errors.Wrapf(err, "read path: %s", pathDir)
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
			err = errors.Wrapf(err, "read %s", f.Name())
			return err
		}
		_, err = db.ExecContext(c, string(content))
		if err != nil {
			err = errors.Wrapf(err, "exec %s", f.Name())
			return err
		}
	}
	return
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
